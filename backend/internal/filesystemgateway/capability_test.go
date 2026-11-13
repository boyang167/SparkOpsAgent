package filesystemgateway

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCapabilityRejectsReplayedNonce(t *testing.T) {
	auth, err := NewCapabilityAuth(Options{
		CapabilitySecret:   testCapabilitySecret,
		CapabilityAudience: testCapabilityAudience,
		NonceStore:         NewMemoryNonceStore(),
	})
	require.NoError(t, err)

	token := capabilityToken(t, capabilityOptions{
		sessionID: "session-replay",
		action:    ActionTransferWrite,
	})

	_, err = auth.VerifyUpload(token, "session-replay")
	require.NoError(t, err)

	_, err = auth.VerifyUpload(token, "session-replay")
	require.Error(t, err)
}

func TestCapabilityRejectsExpiredToken(t *testing.T) {
	auth, err := NewCapabilityAuth(Options{
		CapabilitySecret:   testCapabilitySecret,
		CapabilityAudience: testCapabilityAudience,
		NonceStore:         NewMemoryNonceStore(),
	})
	require.NoError(t, err)

	token := capabilityToken(t, capabilityOptions{
		sessionID: "session-expired",
		action:    ActionTransferWrite,
		expires:   time.Now().Add(-time.Minute),
	})

	_, err = auth.VerifyUpload(token, "session-expired")
	require.Error(t, err)
}
