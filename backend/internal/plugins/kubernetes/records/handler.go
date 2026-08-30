package records

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

const (
	MaxRecordingBytes   = 5 << 20
	recordingChunkBytes = 32 << 10
	defaultPageSize     = 20
	maxPageSize         = 100
	maxSafeInteger      = int64(1<<53 - 1)
)

var truncatedMarkerLine = mustJSONLine(recordFrame{
	Time:   0,
	Stream: "truncated",
	Data:   "recording truncated at 5MiB",
})

type Store struct {
	db *gorm.DB
}

type Session struct {
	WorkspaceID string
	ClusterID   string
	Kind        string
	StartedAt   time.Time
}

type SessionRecorder struct {
	id          string
	workspaceID string
	store       *Store
	startedAt   time.Time
	mu          sync.Mutex
	blob        bytes.Buffer
	truncated   bool
	closed      bool
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Start(ctx context.Context, session Session) (*SessionRecorder, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("kubernetes records store unavailable")
	}
	startedAt := session.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	record := k8sschema.Record{
		ID:          uuid.NewString(),
		WorkspaceID: session.WorkspaceID,
		ClusterID:   session.ClusterID,
		Kind:        session.Kind,
		StartedAt:   startedAt,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, err
	}
	return &SessionRecorder{id: record.ID, workspaceID: record.WorkspaceID, store: s, startedAt: startedAt}, nil
}

func (r *SessionRecorder) ID() string {
	if r == nil {
		return ""
	}
	return r.id
}

func (r *SessionRecorder) Stdout() io.Writer {
	return recordStreamWriter{recorder: r, stream: "stdout"}
}

func (r *SessionRecorder) Stderr() io.Writer {
	return recordStreamWriter{recorder: r, stream: "stderr"}
}

func (r *SessionRecorder) Event(stream string, data string) {
	if r == nil {
		return
	}
	r.append(stream, []byte(data))
}

func (r *SessionRecorder) Close(ctx context.Context, endedAt time.Time) error {
	if r == nil || r.store == nil || r.store.db == nil {
		return nil
	}
	if endedAt.IsZero() {
		endedAt = time.Now().UTC()
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	blob := append([]byte(nil), r.blob.Bytes()...)
	r.mu.Unlock()

	result := r.store.db.WithContext(ctx).Model(&k8sschema.Record{}).
		Where("id = ? AND workspace_id = ?", r.id, r.workspaceID).
		Updates(map[string]any{"ended_at": endedAt, "blob": blob})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("kubernetes record finalize affected unexpected rows")
	}
	return nil
}

type recordStreamWriter struct {
	recorder *SessionRecorder
	stream   string
}

func (w recordStreamWriter) Write(p []byte) (int, error) {
	if w.recorder == nil {
		return len(p), nil
	}
	written := len(p)
	remaining := p
	for len(remaining) > 0 {
		n := len(remaining)
		if n > recordingChunkBytes {
			n = recordingChunkBytes
		}
		w.recorder.append(w.stream, remaining[:n])
		remaining = remaining[n:]
	}
	return written, nil
}

type recordFrame struct {
	Time   float64 `json:"time"`
	Stream string  `json:"stream"`
	Data   string  `json:"data"`
}

func (r *SessionRecorder) append(stream string, data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.truncated || r.closed {
		return
	}
	frame := recordFrame{
		Time:   math.Round(time.Since(r.startedAt).Seconds()*1000) / 1000,
		Stream: stream,
		Data:   string(bytes.ToValidUTF8(data, []byte("\uFFFD"))),
	}
	line, err := json.Marshal(frame)
	if err != nil {
		return
	}
	line = append(line, '\n')
	if r.blob.Len()+len(line) <= maxFrameBytesBeforeMarker() {
		_, _ = r.blob.Write(line)
		return
	}
	r.truncated = true
	r.appendTruncatedMarkerLocked()
}

func (r *SessionRecorder) appendTruncatedMarkerLocked() {
	limit := maxFrameBytesBeforeMarker()
	if r.blob.Len() > limit {
		data := r.blob.Bytes()
		cut := bytes.LastIndexByte(data[:limit], '\n')
		if cut >= 0 {
			r.blob.Truncate(cut + 1)
		} else {
			r.blob.Reset()
		}
	}
	if r.blob.Len() > 0 {
		data := r.blob.Bytes()
		if data[len(data)-1] != '\n' {
			if r.blob.Len()+1 > limit {
				r.blob.Truncate(limit - 1)
			}
			_ = r.blob.WriteByte('\n')
		}
	}
	_, _ = r.blob.Write(truncatedMarkerLine)
}

func maxFrameBytesBeforeMarker() int {
	return MaxRecordingBytes - len(truncatedMarkerLine)
}

func mustJSONLine(frame recordFrame) []byte {
	line, err := json.Marshal(frame)
	if err != nil {
		panic(err)
	}
	return append(line, '\n')
}

type Handler struct {
	ctx   sparkplugin.PluginContext
	store *Store
}

func NewHandler(ctx sparkplugin.PluginContext) *Handler {
	return &Handler{ctx: ctx, store: NewStore(ctx.DB)}
}

func (h *Handler) List(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	page, pageSize, ok := pagination(c)
	if !ok {
		return
	}
	if h == nil || h.store == nil || h.store.db == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return
	}
	offset := (page - 1) * pageSize
	db := h.store.db.WithContext(c.Request.Context()).Model(&k8sschema.Record{}).Where("workspace_id = ?", wsID.String())
	var total int64
	if err := db.Count(&total).Error; err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_RECORDS_UNAVAILABLE", "terminal records unavailable", http.StatusServiceUnavailable)
		return
	}
	var rows []recordListRow
	if err := db.Select("id", "workspace_id", "cluster_id", "kind", "started_at", "ended_at", "object_path").
		Order("started_at DESC").
		Limit(int(pageSize)).
		Offset(int(offset)).
		Find(&rows).Error; err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_RECORDS_UNAVAILABLE", "terminal records unavailable", http.StatusServiceUnavailable)
		return
	}
	items := make([]recordListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toItem())
	}
	noStore(c)
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (h *Handler) Get(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	recordID := strings.TrimSpace(c.Param("recordId"))
	if recordID == "" {
		notFound(c)
		return
	}
	if h == nil || h.store == nil || h.store.db == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return
	}
	var record k8sschema.Record
	err := h.store.db.WithContext(c.Request.Context()).
		Where("id = ? AND workspace_id = ?", recordID, wsID.String()).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		notFound(c)
		return
	}
	if err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_RECORDS_UNAVAILABLE", "terminal record unavailable", http.StatusServiceUnavailable)
		return
	}
	noStore(c)
	c.Data(http.StatusOK, "application/x-ndjson", record.Blob)
}

func (h *Handler) Delete(c *gin.Context) {
	wsID, memberID, ok := deleteActor(c)
	if !ok {
		return
	}
	if h == nil || h.store == nil || h.store.db == nil || h.ctx.Writer == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return
	}
	allowed, err := h.ctx.Writer.CanWrite(c.Request.Context(), wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	if !allowed {
		httpx.Fail(c, "GOVERNANCE_API_ACCESS_DENIED", "governance admin permission required", http.StatusForbidden)
		return
	}
	recordID := strings.TrimSpace(c.Param("recordId"))
	if recordID == "" {
		notFound(c)
		return
	}
	result := h.store.db.WithContext(c.Request.Context()).
		Where("id = ? AND workspace_id = ?", recordID, wsID.String()).
		Delete(&k8sschema.Record{})
	if result.Error != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_RECORDS_UNAVAILABLE", "terminal record unavailable", http.StatusServiceUnavailable)
		return
	}
	if result.RowsAffected == 0 {
		notFound(c)
		return
	}
	noStore(c)
	c.Status(http.StatusNoContent)
}

type recordListRow struct {
	ID          string
	WorkspaceID string
	ClusterID   string
	Kind        string
	StartedAt   time.Time
	EndedAt     *time.Time
	ObjectPath  string
}

type recordListItem struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspaceId"`
	ClusterID   string     `json:"clusterId"`
	Kind        string     `json:"kind"`
	StartedAt   time.Time  `json:"startedAt"`
	EndedAt     *time.Time `json:"endedAt,omitempty"`
	ObjectPath  string     `json:"objectPath,omitempty"`
}

func (r recordListRow) toItem() recordListItem {
	return recordListItem{
		ID:          r.ID,
		WorkspaceID: r.WorkspaceID,
		ClusterID:   r.ClusterID,
		Kind:        r.Kind,
		StartedAt:   r.StartedAt,
		EndedAt:     r.EndedAt,
		ObjectPath:  r.ObjectPath,
	}
}

func pagination(c *gin.Context) (int64, int64, bool) {
	page, ok := positiveQueryInt(c, "page", 1, math.MaxInt64)
	if !ok {
		return 0, 0, false
	}
	pageSize, ok := positiveQueryInt(c, "pageSize", defaultPageSize, maxPageSize)
	if !ok {
		return 0, 0, false
	}
	if page > maxSafeInteger || pageSize > maxSafeInteger || page > maxSafeInteger/pageSize {
		httpx.Fail(c, "PLUGIN_KUBERNETES_RECORDS_PAGE_INVALID", "invalid records pagination", http.StatusBadRequest)
		return 0, 0, false
	}
	return page, pageSize, true
}

func positiveQueryInt(c *gin.Context, key string, defaultValue int64, maxValue int64) (int64, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return defaultValue, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 1 || value > maxValue {
		httpx.Fail(c, "PLUGIN_KUBERNETES_RECORDS_PAGE_INVALID", "invalid records pagination", http.StatusBadRequest)
		return 0, false
	}
	return value, true
}

func workspaceID(c *gin.Context) (kernel.WorkspaceID, bool) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return kernel.WorkspaceID{}, false
	}
	return wsID, true
}

func deleteActor(c *gin.Context) (kernel.WorkspaceID, kernel.MemberID, bool) {
	wsID, ok := workspaceID(c)
	if !ok {
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	return wsID, memberID, true
}

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
}

func notFound(c *gin.Context) {
	httpx.Fail(c, "PLUGIN_KUBERNETES_RECORD_NOT_FOUND", "terminal record not found", http.StatusNotFound)
}
