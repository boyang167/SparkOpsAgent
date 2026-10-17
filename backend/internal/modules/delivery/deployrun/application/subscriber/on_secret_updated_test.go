package subscriber

import (
	"context"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type forbiddenCredentialSync struct{ calls int }

func (s *forbiddenCredentialSync) SyncForSecret(context.Context, kernel.WorkspaceID, kernel.SecretID, string) error {
	s.calls++
	return nil
}

type secretOperationWriter struct{ request messaging.OperationRequest }

func (w *secretOperationWriter) Schedule(_ context.Context, request messaging.OperationRequest) error {
	w.request = request
	return nil
}

func TestSecretUpdatedSchedulesCredentialOperationWithoutDirectSync(t *testing.T) {
	syncer := &forbiddenCredentialSync{}
	writer := &secretOperationWriter{}
	handler := NewOnSecretUpdated(syncer, nil, time.Minute, writer)
	eventID := "event-secret-1"
	ctx := events.WithMetadata(context.Background(), events.Metadata{
		EventID: eventID, Subscriber: "delivery.secret-updated.credential-sync",
	})
	workspaceID := kernel.NewWorkspaceID()
	secretID := kernel.NewSecretID()

	require.NoError(t, handler.Handle(ctx, integration.NewSecretUpdated(workspaceID, secretID, "jenkins")))
	require.Zero(t, syncer.calls)
	require.Equal(t, outbound.OperationTypeJenkinsCredentialSync, writer.request.OperationType)
	require.Equal(t,
		eventID+":delivery.secret-updated.credential-sync:"+outbound.OperationTypeJenkinsCredentialSync+":"+secretID.String(),
		writer.request.IdempotencyKey,
	)
}
