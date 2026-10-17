package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	deploymodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployrepo "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	OperationTypeWebhookSend           = "delivery.webhook.send.v1"
	OperationTypeJenkinsCredentialSync = "delivery.jenkins.credentials.sync.v1"
	OperationTypeDeployQueuePublish    = "delivery.queue.deploy.publish.v1"
)

type WebhookPayload struct {
	EventID   string         `json:"event_id"`
	WebhookID string         `json:"webhook_id"`
	URL       string         `json:"url"`
	Token     string         `json:"token,omitempty"`
	Body      map[string]any `json:"body"`
	Timeout   time.Duration  `json:"timeout"`
}

type JenkinsCredentialSyncPayload struct {
	EventID     string        `json:"event_id"`
	WorkspaceID string        `json:"workspace_id"`
	SecretID    string        `json:"secret_id"`
	ServiceType string        `json:"service_type"`
	Timeout     time.Duration `json:"timeout"`
}

type DeployQueuePayload struct {
	EventID     string `json:"event_id"`
	RunID       string `json:"run_id"`
	WorkspaceID string `json:"workspace_id"`
}

func NewDeployQueueHandler(repo deployrepo.DeploymentRunRepository, queue messaging.Queue) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload DeployQueuePayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode deploy queue operation: %w", err))
		}
		runID, err := kernel.ParseDeploymentRunID(payload.RunID)
		if err != nil {
			return messaging.Permanent(err)
		}
		run, err := repo.FindByID(ctx, runID)
		if err != nil {
			return err
		}
		switch run.Status {
		case deploymodel.StatusFailed, deploymodel.StatusAborted, deploymodel.StatusSucceeded:
			return nil
		}
		body, err := json.Marshal(map[string]string{
			"run_id": payload.RunID, "workspace_id": payload.WorkspaceID,
			"event_id": payload.EventID, "idempotency_key": operation.IdempotencyKey,
		})
		if err != nil {
			return messaging.Permanent(err)
		}
		return queue.Publish(ctx, messaging.TopicDeploy, body)
	}
}

type CredentialSyncer interface {
	SyncForSecret(context.Context, kernel.WorkspaceID, kernel.SecretID, string) error
}

func NewCredentialSyncHandler(syncer CredentialSyncer) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload JenkinsCredentialSyncPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode credential sync operation: %w", err))
		}
		workspaceID, err := kernel.ParseWorkspaceID(payload.WorkspaceID)
		if err != nil {
			return messaging.Permanent(err)
		}
		secretID, err := kernel.ParseSecretID(payload.SecretID)
		if err != nil {
			return messaging.Permanent(err)
		}
		timeout := payload.Timeout
		if timeout <= 0 {
			timeout = 2 * time.Minute
		}
		syncCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return syncer.SyncForSecret(syncCtx, workspaceID, secretID, payload.ServiceType)
	}
}

type WebhookHandler struct {
	client *http.Client
}

func NewWebhookHandler(client *http.Client) *WebhookHandler {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &WebhookHandler{client: client}
}

func (h *WebhookHandler) Handle(ctx context.Context, operation messaging.Operation) error {
	var payload WebhookPayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return messaging.Permanent(fmt.Errorf("decode webhook operation: %w", err))
	}
	body, err := json.Marshal(payload.Body)
	if err != nil {
		return messaging.Permanent(fmt.Errorf("encode webhook body: %w", err))
	}
	timeout := payload.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, payload.URL, bytes.NewReader(body))
	if err != nil {
		return messaging.Permanent(fmt.Errorf("create webhook request: %w", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-SparkOps-Event-ID", payload.EventID)
	req.Header.Set("X-SparkOps-Idempotency-Key", operation.IdempotencyKey)
	if payload.Token != "" {
		req.Header.Set("X-SparkOps-Token", payload.Token)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest && resp.StatusCode != http.StatusTooManyRequests {
		return messaging.Permanent(fmt.Errorf("webhook rejected with status %d", resp.StatusCode))
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
