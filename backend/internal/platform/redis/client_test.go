package redis_test

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/redis"
	"github.com/stretchr/testify/require"
)

func TestClient_Ping_skips_when_addr_empty(t *testing.T) {
	c, err := redis.NewClient(redis.Config{Addr: ""})
	require.NoError(t, err)
	require.NoError(t, c.Ping(context.Background()))
}

func TestClient_Get_returns_error_when_not_configured(t *testing.T) {
	c, err := redis.NewClient(redis.Config{Addr: ""})
	require.NoError(t, err)

	_, err = c.Get(context.Background(), "key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "redis not configured")
}

func TestClient_Set_returns_error_when_not_configured(t *testing.T) {
	c, err := redis.NewClient(redis.Config{Addr: ""})
	require.NoError(t, err)

	err = c.Set(context.Background(), "key", "val", 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "redis not configured")
}
