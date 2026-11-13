package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	fsdomain "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
)

func TestReadFileToolCannotEscapeCapabilityScope(t *testing.T) {
	fileA, fileB := uuid.New(), uuid.New()
	workspaceID := uuid.New()
	tool := newReadFileTool(capabilityFor(fileA, workspaceID), FilesystemToolDeps{})
	_, err := tool.Call(context.Background(), map[string]any{"file_id": fileB.String()})
	require.ErrorIs(t, err, fsdomain.ErrCapabilityScopeDenied)
}

func capabilityFor(fileID, workspaceID uuid.UUID) service.Claims {
	return service.Claims{
		WorkspaceID: workspaceID,
		ActorID:     uuid.New(),
		RunID:       uuid.New(),
		ResourceIDs: []uuid.UUID{fileID},
		Actions:     []string{actionFileRead},
		Audience:    service.AgentCapabilityAudience,
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
		Nonce:       "test-nonce",
	}
}
