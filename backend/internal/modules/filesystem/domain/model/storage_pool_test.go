package model

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPoolRejectsTwoPrimaryProvidersWithSamePriority(t *testing.T) {
	pool := NewStoragePool(uuid.New(), "default")
	require.NoError(t, pool.AddProvider(uuid.New(), ProviderModePrimary, 10))
	err := pool.AddProvider(uuid.New(), ProviderModePrimary, 10)
	require.ErrorIs(t, err, ErrAmbiguousPrimaryProvider)
}
