package capability

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
)

func TestSignerRoundTripBindsAudienceAndHash(t *testing.T) {
	signer, err := NewSigner("test-secret", "sparkops-agent-capability")
	require.NoError(t, err)
	fileA := uuid.New()
	agentID := uuid.New()
	claims := service.Claims{
		OrganizationID: uuid.New(), WorkspaceID: uuid.New(), ActorID: uuid.New(), AgentID: &agentID, RunID: uuid.New(),
		ResourceIDs: []uuid.UUID{fileA}, Actions: []string{"file:read"},
		Audience: "sparkops-agent-capability", ExpiresAt: time.Now().UTC().Add(time.Hour), Nonce: "nonce-1",
	}
	token, hash, err := signer.Sign(claims)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.Len(t, hash, 64)
	require.NotContains(t, hash, ".")
	require.Equal(t, service.HashToken(token), hash)

	parsed, err := signer.Verify(token, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, claims.ResourceIDs, parsed.ResourceIDs)
	require.Equal(t, claims.RunID, parsed.RunID)
	require.Equal(t, agentID, *parsed.AgentID)

	other, err := NewSigner("test-secret", "sparkops-file-transfer")
	require.NoError(t, err)
	_, err = other.Verify(token, time.Now().UTC())
	require.Error(t, err)
}
