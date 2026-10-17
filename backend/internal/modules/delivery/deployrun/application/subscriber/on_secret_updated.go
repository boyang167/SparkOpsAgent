package subscriber

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"go.uber.org/zap"
)

type secretCredentialSyncer interface {
	SyncForSecret(context.Context, kernel.WorkspaceID, kernel.SecretID, string) error
}

type OnSecretUpdated struct {
	sync       secretCredentialSyncer
	logger     *zap.Logger
	timeout    time.Duration
	operations messaging.OperationWriter
}

func NewOnSecretUpdated(sync secretCredentialSyncer, logger *zap.Logger, timeout time.Duration, writers ...messaging.OperationWriter) *OnSecretUpdated {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	handler := &OnSecretUpdated{sync: sync, logger: logger, timeout: timeout}
	if len(writers) > 0 {
		handler.operations = writers[0]
	}
	return handler
}

func (s *OnSecretUpdated) Handle(ctx context.Context, event events.Event) error {
	updated, ok := event.(integration.SecretUpdated)
	if !ok {
		return nil
	}
	if s.operations != nil {
		metadata, ok := events.MetadataFromContext(ctx)
		if !ok {
			return nil
		}
		return s.operations.Schedule(ctx, messaging.OperationRequest{
			Owner: metadata.Subscriber, OperationType: outbound.OperationTypeJenkinsCredentialSync,
			IdempotencyKey: metadata.EventID + ":" + metadata.Subscriber + ":" + outbound.OperationTypeJenkinsCredentialSync + ":" + updated.SecretID.String(),
			Payload: outbound.JenkinsCredentialSyncPayload{
				EventID: metadata.EventID, WorkspaceID: updated.WorkspaceID.String(),
				SecretID: updated.SecretID.String(), ServiceType: updated.ServiceType, Timeout: s.timeout,
			},
		})
	}
	syncCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	if err := s.sync.SyncForSecret(syncCtx, updated.WorkspaceID, updated.SecretID, updated.ServiceType); err != nil {
		if s.logger != nil {
			s.logger.Error("jenkins credential sync failed",
				zap.String("workspace_id", updated.WorkspaceID.String()),
				zap.String("secret_id", updated.SecretID.String()),
				zap.String("service_type", updated.ServiceType),
				zap.Error(err),
			)
		}
	}
	return nil
}
