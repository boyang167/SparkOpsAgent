//go:build integration

package security_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/capability"
	testenv "github.com/sparkops/sparkops-agent/backend/tests/filesystem/integration"
)

func TestAgentCapabilityCannotExpandRunScope(t *testing.T) {
	testenv.Require(t)
	const audience = service.AgentCapabilityAudience
	signer, err := capability.NewSigner("integration-agent-secret", audience)
	require.NoError(t, err)
	issuer := service.NewCapabilityIssuer(signer)
	allowedResource, forbiddenResource := uuid.New(), uuid.New()

	issued, err := issuer.Issue(context.Background(), service.CapabilityRequest{
		OrganizationID: uuid.New(), WorkspaceID: uuid.New(), ActorID: uuid.New(), RunID: uuid.New(),
		ResourceIDs: []uuid.UUID{allowedResource, forbiddenResource},
		Actions:     []string{"file:read", "file:delete"},
		Audience:    audience,
		ExpiresAt:   time.Now().Add(time.Minute),
	}, service.RunScope{
		ResourceIDs: []uuid.UUID{allowedResource},
		Actions:     []string{"file:read"},
	})
	require.NoError(t, err)

	claims, err := signer.Verify(issued.Token, time.Now())
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{allowedResource}, claims.ResourceIDs)
	require.Equal(t, []string{"file:read"}, claims.Actions)

	wrongAudience, err := capability.NewSigner("integration-agent-secret", "other-audience")
	require.NoError(t, err)
	_, err = wrongAudience.Verify(issued.Token, time.Now())
	require.Error(t, err)
}
