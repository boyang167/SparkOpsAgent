package s3

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

func TestS3ProviderRejectsInvalidConfig(t *testing.T) {
	_, err := New(Config{Region: "us-east-1"})
	require.Error(t, err)
}

func TestS3ProviderCapabilitiesUnit(t *testing.T) {
	p := &Provider{}
	caps := p.Capabilities()
	require.True(t, caps.RangeRead)
	require.True(t, caps.Multipart)
	require.True(t, caps.PresignedUpload)
	require.False(t, caps.POSIX)
	require.Equal(t, "s3", p.ID())
}

func TestValidateObjectKey(t *testing.T) {
	require.ErrorIs(t, validateObjectKey("../escape"), storage.ErrInvalidObjectKey)
	require.ErrorIs(t, validateObjectKey("good/key"), nil)
}
