package operationlog

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) AutoMigrate() error {
	if s.db.Dialector.Name() == "postgres" {
		if err := s.db.Exec("CREATE SCHEMA IF NOT EXISTS platform").Error; err != nil {
			return err
		}
	}
	return s.db.AutoMigrate(&recordModel{})
}

type recordModel struct {
	ID           string `gorm:"column:id;primaryKey"`
	IP           string `gorm:"column:ip"`
	Method       string `gorm:"column:method"`
	Path         string `gorm:"column:path"`
	Status       int    `gorm:"column:status"`
	LatencyMs    int64  `gorm:"column:latency_ms"`
	Agent        string `gorm:"column:agent"`
	ErrorMessage string `gorm:"column:error_message"`
	Body         string `gorm:"column:body;type:text"`
	Resp         string `gorm:"column:resp;type:text"`
	MemberID     *string
	WorkspaceID  *string
	TraceID      string
	CreatedAt    time.Time
}

func (recordModel) TableName() string { return "platform.operation_records" }

type Record struct {
	IP           string
	Method       string
	Path         string
	Status       int
	LatencyMs    int64
	Agent        string
	ErrorMessage string
	Body         string
	Resp         string
	MemberID     string
	WorkspaceID  string
	TraceID      string
}

type RecordView struct {
	ID           string `json:"id"`
	IP           string `json:"ip"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Status       int    `json:"status"`
	LatencyMs    int64  `json:"latency_ms"`
	Agent        string `json:"agent"`
	ErrorMessage string `json:"error_message"`
	Body         string `json:"body"`
	Resp         string `json:"resp"`
	MemberID     string `json:"member_id,omitempty"`
	WorkspaceID  string `json:"workspace_id,omitempty"`
	TraceID      string `json:"trace_id,omitempty"`
	CreatedAt    string `json:"created_at"`
}

type ListFilter struct {
	WorkspaceID string
	IP          string
	Method      string
	Path        string
	Status      *int
	MemberID    string
	TraceID     string
	Limit       int
	Offset      int
}

type ListResult struct {
	Items []RecordView `json:"items"`
	Total int64        `json:"total"`
}

func (s *Store) Create(ctx context.Context, rec Record) error {
	memberID := (*string)(nil)
	if rec.MemberID != "" {
		v := rec.MemberID
		memberID = &v
	}
	wsID := (*string)(nil)
	if rec.WorkspaceID != "" {
		v := rec.WorkspaceID
		wsID = &v
	}
	row := recordModel{
		ID: uuid.NewString(), IP: rec.IP, Method: rec.Method, Path: rec.Path,
		Status: rec.Status, LatencyMs: rec.LatencyMs, Agent: rec.Agent,
		ErrorMessage: rec.ErrorMessage, Body: rec.Body, Resp: rec.Resp,
		MemberID: memberID, WorkspaceID: wsID, TraceID: rec.TraceID,
		CreatedAt: time.Now().UTC(),
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

func (s *Store) applyFilter(q *gorm.DB, filter ListFilter) *gorm.DB {
	if filter.WorkspaceID != "" {
		q = q.Where("workspace_id = ?", filter.WorkspaceID)
	}
	if ip := strings.TrimSpace(filter.IP); ip != "" {
		q = q.Where("ip ILIKE ?", "%"+ip+"%")
	}
	if method := strings.ToUpper(strings.TrimSpace(filter.Method)); method != "" {
		q = q.Where("method = ?", method)
	}
	if path := strings.TrimSpace(filter.Path); path != "" {
		q = q.Where("path ILIKE ?", "%"+path+"%")
	}
	if filter.Status != nil {
		q = q.Where("status = ?", *filter.Status)
	}
	if memberID := strings.TrimSpace(filter.MemberID); memberID != "" {
		q = q.Where("member_id = ?", memberID)
	}
	if traceID := strings.TrimSpace(filter.TraceID); traceID != "" {
		q = q.Where("trace_id = ?", traceID)
	}
	return q
}

func (s *Store) List(ctx context.Context, filter ListFilter) (ListResult, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	base := s.applyFilter(s.db.WithContext(ctx).Model(&recordModel{}), filter)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return ListResult{}, err
	}

	var rows []recordModel
	if err := base.Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return ListResult{}, err
	}

	items := make([]RecordView, 0, len(rows))
	for _, row := range rows {
		items = append(items, toView(row))
	}
	return ListResult{Items: items, Total: total}, nil
}

func (s *Store) Delete(ctx context.Context, id, workspaceID string) error {
	q := s.db.WithContext(ctx).Where("id = ?", id)
	if workspaceID != "" {
		q = q.Where("workspace_id = ?", workspaceID)
	}
	res := q.Delete(&recordModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) DeleteByIDs(ctx context.Context, ids []string, workspaceID string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	q := s.db.WithContext(ctx).Where("id IN ?", ids)
	if workspaceID != "" {
		q = q.Where("workspace_id = ?", workspaceID)
	}
	res := q.Delete(&recordModel{})
	return res.RowsAffected, res.Error
}

func toView(row recordModel) RecordView {
	view := RecordView{
		ID: row.ID, IP: row.IP, Method: row.Method, Path: row.Path,
		Status: row.Status, LatencyMs: row.LatencyMs, Agent: row.Agent,
		ErrorMessage: row.ErrorMessage, Body: row.Body, Resp: row.Resp,
		TraceID: row.TraceID, CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339),
	}
	if row.MemberID != nil {
		view.MemberID = *row.MemberID
	}
	if row.WorkspaceID != nil {
		view.WorkspaceID = *row.WorkspaceID
	}
	return view
}
