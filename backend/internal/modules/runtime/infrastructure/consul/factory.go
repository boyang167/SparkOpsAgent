package consul

import (
	"context"
	"fmt"
	"strings"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

func NewKVStore(cfg *config.Config, db *gorm.DB, logger *zap.Logger) (KVStore, error) {
	dbStore := NewStore(db)
	if err := dbStore.AutoMigrate(); err != nil {
		return nil, err
	}

	if cfg.Consul.Driver != "consul" || strings.TrimSpace(cfg.Consul.Address) == "" {
		return dbStore, nil
	}

	client, err := newConsulClient(cfg.Consul)
	if err != nil {
		if logger != nil {
			logger.Warn("consul client unavailable, falling back to db store", zap.Error(err))
		}
		return dbStore, nil
	}
	return &ClientStore{client: client, prefix: cfg.Consul.KeyPrefix}, nil
}

var _ KVStore = (*ClientStore)(nil)

func newConsulClient(cfg config.ConsulConfig) (*consulapi.Client, error) {
	apiCfg := consulapi.DefaultConfig()
	apiCfg.Address = cfg.Address
	if cfg.Token != "" {
		apiCfg.Token = cfg.Token
	}
	if cfg.Datacenter != "" {
		apiCfg.Datacenter = cfg.Datacenter
	}
	return consulapi.NewClient(apiCfg)
}

type ClientStore struct {
	client *consulapi.Client
	prefix string
}

func (s *ClientStore) workspacePrefix(workspaceID string) string {
	base := strings.Trim(strings.TrimSpace(s.prefix), "/")
	if base == "" {
		base = "sparkops"
	}
	return fmt.Sprintf("%s/ws/%s/", base, workspaceID)
}

func (s *ClientStore) List(ctx context.Context, workspaceID, prefix string) ([]KVEntry, error) {
	_ = ctx
	kv := s.client.KV()
	searchPrefix := s.workspacePrefix(workspaceID) + strings.TrimPrefix(prefix, "/")
	pairs, _, err := kv.List(searchPrefix, nil)
	if err != nil {
		return nil, err
	}
	out := make([]KVEntry, 0, len(pairs))
	wsPrefix := s.workspacePrefix(workspaceID)
	for _, pair := range pairs {
		key := strings.TrimPrefix(pair.Key, wsPrefix)
		value := ""
		if pair.Value != nil {
			value = string(pair.Value)
		}
		out = append(out, KVEntry{
			Key:       key,
			Value:     value,
			UpdatedAt: timeFromModifyIndex(pair.ModifyIndex),
		})
	}
	return out, nil
}

func (s *ClientStore) Put(ctx context.Context, workspaceID, key, value string) error {
	_ = ctx
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	_, err := s.client.KV().Put(&consulapi.KVPair{
		Key:   s.workspacePrefix(workspaceID) + key,
		Value: []byte(value),
	}, nil)
	return err
}

func (s *ClientStore) Delete(ctx context.Context, workspaceID, key string) error {
	_ = ctx
	_, err := s.client.KV().Delete(s.workspacePrefix(workspaceID)+key, nil)
	return err
}

func timeFromModifyIndex(_ uint64) time.Time {
	return time.Now().UTC()
}
