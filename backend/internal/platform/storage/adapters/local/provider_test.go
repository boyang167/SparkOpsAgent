package local

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

func TestLocalProviderRejectsSymlinkAtOpaquePath(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	provider, err := New(root)
	require.NoError(t, err)
	path := provider.objectPath("objects/opaque")
	if err := os.Symlink(outside, path); err != nil {
		t.Skipf("symlink creation unavailable on this platform: %v", err)
	}
	_, _, err = provider.Get(context.Background(), storage.GetRequest{ObjectKey: "objects/opaque"})
	require.ErrorIs(t, err, storage.ErrInvalidObjectKey)
}

func TestLocalProviderRejectsSymlinkRoot(t *testing.T) {
	target := t.TempDir()
	root := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(target, root); err != nil {
		t.Skipf("symlink creation unavailable on this platform: %v", err)
	}
	_, err := New(root)
	require.ErrorIs(t, err, storage.ErrInvalidRoot)
}

func TestLocalProviderPublishSyncFailurePropagates(t *testing.T) {
	provider, err := New(t.TempDir())
	require.NoError(t, err)
	provider.syncDir = func(string) error { return os.ErrPermission }
	_, err = provider.Put(context.Background(), storage.PutRequest{
		ObjectKey: "objects/sync-failure", Body: bytes.NewReader([]byte("x")), Size: 1,
	})
	require.ErrorIs(t, err, os.ErrPermission)
}

func TestLocalProviderCancellationLeavesNoTemporaryFiles(t *testing.T) {
	root := t.TempDir()
	provider, err := New(root)
	require.NoError(t, err)
	body := bytes.Repeat([]byte("x"), 4096)
	ctx, cancel := context.WithCancel(context.Background())
	_, err = provider.Put(ctx, storage.PutRequest{
		ObjectKey: "objects/cancelled", Body: &localCancelReader{cancel: cancel, data: body}, Size: int64(len(body)),
	})
	require.ErrorIs(t, err, context.Canceled)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	for _, entry := range entries {
		require.NotContains(t, entry.Name(), ".storage-")
	}

	_, err = provider.Put(context.Background(), storage.PutRequest{
		ObjectKey: "objects/source", Body: bytes.NewReader(body), Size: int64(len(body)),
	})
	require.NoError(t, err)
	cancelCtx, stop := context.WithCancel(context.Background())
	stop()
	_, err = provider.Copy(cancelCtx, "objects/source", "objects/copy-cancelled")
	require.ErrorIs(t, err, context.Canceled)
	entries, err = os.ReadDir(root)
	require.NoError(t, err)
	for _, entry := range entries {
		require.NotContains(t, entry.Name(), ".storage-")
	}
}

type localCancelReader struct {
	cancel context.CancelFunc
	data   []byte
}

func (r *localCancelReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	r.cancel()
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestLocalProviderCrossInstanceConcurrentSamePutIsIdempotent(t *testing.T) {
	root := t.TempDir()
	first, err := New(root)
	require.NoError(t, err)
	second, err := New(root)
	require.NoError(t, err)
	body := bytes.Repeat([]byte("x"), 4096)
	var start sync.WaitGroup
	start.Add(1)
	errs := make(chan error, 2)
	infos := make(chan storage.ObjectInfo, 2)
	for _, provider := range []*Provider{first, second} {
		go func(provider *Provider) {
			start.Wait()
			info, putErr := provider.Put(context.Background(), storage.PutRequest{
				ObjectKey: "objects/race", Body: bytes.NewReader(body), Size: int64(len(body)),
			})
			errs <- putErr
			infos <- info
		}(provider)
	}
	start.Done()
	for i := 0; i < 2; i++ {
		require.NoError(t, <-errs)
	}
	require.Equal(t, <-infos, <-infos)
}

func TestLocalProviderCrossInstanceConcurrentDifferentPutConflicts(t *testing.T) {
	root := t.TempDir()
	first, err := New(root)
	require.NoError(t, err)
	second, err := New(root)
	require.NoError(t, err)
	var start sync.WaitGroup
	start.Add(1)
	var successes, conflicts atomic.Int32
	for i, provider := range []*Provider{first, second} {
		body := bytes.Repeat([]byte{byte('a' + i)}, 4096)
		go func(provider *Provider, body []byte) {
			start.Wait()
			_, putErr := provider.Put(context.Background(), storage.PutRequest{
				ObjectKey: "objects/different-race", Body: bytes.NewReader(body), Size: int64(len(body)),
			})
			if putErr == nil {
				successes.Add(1)
			} else if errors.Is(putErr, storage.ErrObjectConflict) {
				conflicts.Add(1)
			}
		}(provider, body)
	}
	start.Done()
	require.Eventually(t, func() bool { return successes.Load()+conflicts.Load() == 2 }, time.Second, time.Millisecond)
	require.Equal(t, int32(1), successes.Load())
	require.Equal(t, int32(1), conflicts.Load())
}
