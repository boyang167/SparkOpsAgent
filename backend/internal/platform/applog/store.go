package applog

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	CategoryHTTP   = "http"
	CategoryApp    = "app"
	CategoryError  = "error"
	CategoryAudit  = "audit"
	CategoryMetric = "metric"
)

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

var logLevels = map[string]int{
	"debug": 10, "info": 20, "warn": 30, "error": 40, "fatal": 50, "panic": 60,
}

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
	return s.db.AutoMigrate(&appLogModel{})
}

type appLogModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	Level       string `gorm:"column:level;index"`
	Category    string `gorm:"column:category;index"`
	Message     string `gorm:"column:message;type:text"`
	TraceID     string `gorm:"column:trace_id;index"`
	WorkspaceID *string
	Caller      string `gorm:"column:caller"`
	Fields      []byte `gorm:"column:fields;type:jsonb"`
	CreatedAt   time.Time
}

func (appLogModel) TableName() string { return "platform.app_logs" }

type Entry struct {
	Level       string
	Category    string
	Message     string
	TraceID     string
	WorkspaceID string
	Caller      string
	Fields      map[string]any
}

type LogView struct {
	ID          string         `json:"id"`
	Level       string         `json:"level"`
	Category    string         `json:"category"`
	Message     string         `json:"message"`
	TraceID     string         `json:"trace_id,omitempty"`
	WorkspaceID string         `json:"workspace_id,omitempty"`
	Caller      string         `json:"caller,omitempty"`
	Fields      map[string]any `json:"fields,omitempty"`
	CreatedAt   string         `json:"created_at"`
}

type ListFilter struct {
	Level       string
	Category    string
	TraceID     string
	WorkspaceID string
	Keyword     string
	Limit       int
	Offset      int
}

type ListResult struct {
	Items []LogView `json:"items"`
	Total int64     `json:"total"`
}

func (s *Store) Append(ctx context.Context, entry Entry) error {
	wsID := (*string)(nil)
	if entry.WorkspaceID != "" {
		v := entry.WorkspaceID
		wsID = &v
	}
	fields := entry.Fields
	if fields == nil {
		fields = map[string]any{}
	}
	raw, _ := json.Marshal(fields)
	row := appLogModel{
		ID:          uuid.NewString(),
		Level:       strings.ToLower(strings.TrimSpace(entry.Level)),
		Category:    normalizeCategory(entry.Category),
		Message:     entry.Message,
		TraceID:     strings.TrimSpace(entry.TraceID),
		WorkspaceID: wsID,
		Caller:      entry.Caller,
		Fields:      raw,
		CreatedAt:   time.Now().UTC(),
	}
	if row.Level == "" {
		row.Level = "info"
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

func normalizeCategory(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		return CategoryApp
	}
	return category
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

	q := s.db.WithContext(ctx).Model(&appLogModel{})
	if level := strings.ToLower(strings.TrimSpace(filter.Level)); level != "" {
		minLevel := logLevels[level]
		levels := make([]string, 0, 6)
		for name, val := range logLevels {
			if val >= minLevel {
				levels = append(levels, name)
			}
		}
		q = q.Where("level IN ?", levels)
	}
	if category := strings.TrimSpace(filter.Category); category != "" {
		q = q.Where("category = ?", category)
	}
	if traceID := strings.TrimSpace(filter.TraceID); traceID != "" {
		q = q.Where("trace_id = ?", traceID)
	}
	if wsID := strings.TrimSpace(filter.WorkspaceID); wsID != "" {
		q = q.Where("workspace_id = ? OR workspace_id IS NULL", wsID)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("message ILIKE ? OR caller ILIKE ? OR fields::text ILIKE ?", like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return ListResult{}, err
	}

	var rows []appLogModel
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return ListResult{}, err
	}

	items := make([]LogView, 0, len(rows))
	for _, row := range rows {
		items = append(items, toView(row))
	}
	return ListResult{Items: items, Total: total}, nil
}

func (s *Store) ListByTrace(ctx context.Context, traceID string, limit int) ([]LogView, error) {
	if limit <= 0 || limit > MaxLimit {
		limit = MaxLimit
	}
	var rows []appLogModel
	err := s.db.WithContext(ctx).
		Where("trace_id = ?", traceID).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]LogView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toView(row))
	}
	return out, nil
}

func toView(row appLogModel) LogView {
	fields := map[string]any{}
	if len(row.Fields) > 0 {
		_ = json.Unmarshal(row.Fields, &fields)
	}
	view := LogView{
		ID: row.ID, Level: row.Level, Category: row.Category, Message: row.Message,
		TraceID: row.TraceID, Caller: row.Caller, Fields: fields,
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	if row.WorkspaceID != nil {
		view.WorkspaceID = *row.WorkspaceID
	}
	return view
}
