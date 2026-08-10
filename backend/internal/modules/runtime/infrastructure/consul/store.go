package consul

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
)

type kvModel struct {
	WorkspaceID string `gorm:"column:workspace_id;primaryKey"`
	Key         string `gorm:"column:key;primaryKey"`
	Value       string `gorm:"type:text"`
	UpdatedAt   time.Time
}

func (kvModel) TableName() string { return "platform.consul_kv" }

type Store struct{ db *gorm.DB }

func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

var _ KVStore = (*Store)(nil)

func (s *Store) AutoMigrate() error {
	if s.db.Dialector.Name() == "postgres" {
		if err := s.db.Exec("CREATE SCHEMA IF NOT EXISTS platform").Error; err != nil {
			return err
		}
	}
	return s.db.AutoMigrate(&kvModel{})
}

func (s *Store) List(ctx context.Context, workspaceID, prefix string) ([]KVEntry, error) {
	var rows []kvModel
	q := s.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if prefix != "" {
		q = q.Where("key LIKE ?", prefix+"%")
	}
	if err := q.Order("key asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]KVEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, KVEntry{Key: row.Key, Value: row.Value, UpdatedAt: row.UpdatedAt})
	}
	return out, nil
}

func (s *Store) Put(ctx context.Context, workspaceID, key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	row := kvModel{
		WorkspaceID: workspaceID, Key: key, Value: value, UpdatedAt: time.Now().UTC(),
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

func (s *Store) Delete(ctx context.Context, workspaceID, key string) error {
	return s.db.WithContext(ctx).
		Where("workspace_id = ? AND key = ?", workspaceID, key).
		Delete(&kvModel{}).Error
}
