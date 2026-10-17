package subscriber

import (
	"context"
	"time"

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
	sync    secretCredentialSyncer
	logger  *zap.Logger
	timeout time.Duration
}

func NewOnSecretUpdated(sync secretCredentialSyncer, logger *zap.Logger, timeout time.Duration) *OnSecretUpdated {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	return &OnSecretUpdated{sync: sync, logger: logger, timeout: timeout}
}

func (s *OnSecretUpdated) Handle(ctx context.Context, event events.Event) error {
	updated, ok := event.(integration.SecretUpdated)
	if !ok {
		return nil
	}
	if messaging.IsInboxConsumerContext(ctx) {
		return messaging.ErrOperationOutboxRequired
	}
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), s.timeout)
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
	}()
	return nil
}
