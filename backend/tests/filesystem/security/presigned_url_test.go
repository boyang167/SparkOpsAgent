//go:build integration

package security_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/filesystemgateway"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/capability"
	testenv "github.com/sparkops/sparkops-agent/backend/tests/filesystem/integration"
)

func TestUploadCapabilityIsShortLivedAndSessionScoped(t *testing.T) {
	testenv.Require(t)
	const (
		secret   = "integration-capability-secret"
		audience = "filesystem-transfer-gateway"
	)
	sessionID := uuid.New()
	issuer, err := capability.NewIssuer(secret, audience)
	require.NoError(t, err)
	token, err := issuer.UploadToken(sessionID, time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.NotContains(t, token, secret)

	auth, err := filesystemgateway.NewCapabilityAuth(filesystemgateway.Options{
		CapabilitySecret: secret, CapabilityAudience: audience,
		NonceStore: filesystemgateway.NewMemoryNonceStore(),
	})
	require.NoError(t, err)

	_, err = auth.VerifyUpload(token, uuid.NewString())
	require.ErrorIs(t, err, filesystemgateway.ErrForbidden)
	_, err = auth.VerifyUpload(token, sessionID.String())
	require.NoError(t, err)
	_, err = auth.VerifyUpload(token, sessionID.String())
	require.ErrorIs(t, err, filesystemgateway.ErrCapabilityReplay)
}
