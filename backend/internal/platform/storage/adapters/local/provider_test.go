package local

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/contracttest"
)

func TestLocalProviderContract(t *testing.T) {
	provider, err := New(t.TempDir())
	require.NoError(t, err)
	contracttest.Run(t, provider)
}

func TestLocalProviderRejectsSymlinkComponents(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink creation unavailable on this platform: %v", err)
	}
	provider, err := New(root)
	require.NoError(t, err)
	_, err = provider.Put(t.Context(), storage.PutRequest{
		ObjectKey: "link/escaped", Body: bytes.NewReader([]byte("x")), Size: 1,
	})
	require.ErrorIs(t, err, storage.ErrInvalidObjectKey)
}
