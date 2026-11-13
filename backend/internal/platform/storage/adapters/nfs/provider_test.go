package nfs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/contracttest"
)

func TestNFSProviderContract(t *testing.T) {
	provider := newProvider(t, t.TempDir())
	contracttest.Run(t, provider)
}

func TestNFSProviderCapabilities(t *testing.T) {
	provider := newProvider(t, t.TempDir())
	caps := provider.Capabilities()
	require.True(t, caps.RangeRead)
	require.True(t, caps.AtomicPublish)
	require.True(t, caps.VerifiedCopy)
	require.True(t, caps.POSIX)
	require.False(t, caps.Multipart)
	require.False(t, caps.PresignedUpload)
	require.Equal(t, "nfs", provider.ID())
}

func newProvider(t *testing.T, root string) *Provider {
	t.Helper()
	provider, err := New(root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Close()) })
	return provider
}
