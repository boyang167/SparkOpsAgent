package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/application/command"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Dispatcher struct {
	repo       *persistence.WebhookRepository
	client     *http.Client
	logger     *zap.Logger
	operations messaging.OperationWriter
	sync       bool
}

func NewDispatcher(repo *persistence.WebhookRepository, logger *zap.Logger, writers ...messaging.OperationWriter) *Dispatcher {
	dispatcher := &Dispatcher{
		repo: repo,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger,
	}
	if len(writers) > 0 {
		dispatcher.operations = writers[0]
	}
	return dispatcher
}

func (d *Dispatcher) Handle(ctx context.Context, e events.Event) error {
	payload, eventName, projectID, ok := payloadFromEvent(e)
	if !ok {
		return nil
	}
	hooks, err := d.repo.ListEnabledByProject(ctx, projectID)
	if err != nil {
		if d.logger != nil {
			d.logger.Warn("list webhooks failed", zap.Error(err))
		}
		return fmt.Errorf("list enabled webhooks: %w", err)
	}
	for i := range hooks {
		hook := hooks[i]
		if !command.HookSubscribes(hook.Events, eventName) {
			continue
		}
		if d.operations != nil {
			metadata, ok := events.MetadataFromContext(ctx)
			if !ok {
				return fmt.Errorf("webhook operation requires event metadata")
			}
			if err := d.operations.Schedule(ctx, messaging.OperationRequest{
				Owner:          metadata.Subscriber,
				OperationType:  OperationTypeWebhookSend,
				IdempotencyKey: metadata.EventID + ":" + metadata.Subscriber + ":" + OperationTypeWebhookSend + ":" + hook.ID.String(),
				Payload: WebhookPayload{
					EventID: metadata.EventID, WebhookID: hook.ID.String(),
					Body: payload, Timeout: 5 * time.Second,
				},
			}); err != nil {
				return err
			}
			continue
		}
		if err := d.dispatch(ctx, hook, payload); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dispatcher) dispatch(ctx context.Context, hook persistence.WebhookRecord, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if hook.Token != "" {
		req.Header.Set("X-SparkOps-Token", hook.Token)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		if d.logger != nil {
			d.logger.Warn("webhook post failed",
				zap.String("webhook_id", hook.ID.String()),
				zap.String("url", hook.URL),
				zap.Error(err),
			)
		}
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 && d.logger != nil {
		d.logger.Warn("webhook post rejected",
			zap.String("webhook_id", hook.ID.String()),
			zap.String("url", hook.URL),
			zap.Int("status", resp.StatusCode),
		)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook rejected with status %d", resp.StatusCode)
	}
	return nil
}

func payloadFromEvent(e events.Event) (map[string]any, string, kernel.ProjectID, bool) {
	switch ev := e.(type) {
	case deliveryevent.BuildCompleted:
		return map[string]any{
			"event":        command.WebhookEventBuild,
			"occurred_at":  ev.OccurredAt().UTC().Format(time.RFC3339),
			"workspace_id": ev.WorkspaceID.String(),
			"project_id":   ev.ProjectID.String(),
			"project_name": ev.ProjectName,
			"data": map[string]any{
				"build_id":     ev.BuildID.String(),
				"status":       ev.Status,
				"build_number": ev.BuildNumber,
			},
		}, command.WebhookEventBuild, ev.ProjectID, true
	case deliveryevent.DeployCompleted:
		name := command.WebhookEventDeploy
		if ev.RunType == "rollback" {
			name = command.WebhookEventRollback
		}
		return map[string]any{
			"event":        name,
			"occurred_at":  ev.OccurredAt().UTC().Format(time.RFC3339),
			"workspace_id": ev.WorkspaceID.String(),
			"project_id":   ev.ProjectID.String(),
			"project_name": ev.ProjectName,
			"data": map[string]any{
				"run_id":    ev.RunID.String(),
				"status":    ev.Status,
				"run_type":  ev.RunType,
				"image_tag": ev.ImageTag,
			},
		}, name, ev.ProjectID, true
	default:
		return nil, "", kernel.ProjectID{}, false
	}
}
