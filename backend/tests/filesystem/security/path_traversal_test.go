//go:build integration

package security_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
	testenv "github.com/sparkops/sparkops-agent/backend/tests/filesystem/integration"
)

func TestLocalProviderRejectsTraversalObjectKeys(t *testing.T) {
	testenv.Require(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })

	for _, key := range []string{"../secret", "objects/../../secret", `/absolute`, `objects\escape`} {
		_, err = provider.Put(context.Background(), storage.PutRequest{
			ObjectKey: key,
			Body:      bytes.NewReader([]byte("secret")),
			Size:      6,
			SHA256:    storage.SHA256([]byte("secret")),
		})
		require.ErrorIs(t, err, storage.ErrInvalidObjectKey, key)
	}
}
