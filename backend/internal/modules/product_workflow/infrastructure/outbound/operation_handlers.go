package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WebhookPayload struct {
	EventID string          `json:"event_id"`
	Target  string          `json:"target"`
	URL     string          `json:"url"`
	Body    json.RawMessage `json:"body"`
	Timeout time.Duration   `json:"timeout"`
}

func NewWebhookHandler(client *http.Client) messaging.OperationHandler {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload WebhookPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode workflow webhook operation: %w", err))
		}
		timeout := payload.Timeout
		if timeout <= 0 {
			timeout = 8 * time.Second
		}
		sendCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, payload.URL, bytes.NewReader(payload.Body))
		if err != nil {
			return messaging.Permanent(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-SparkOps-Event-ID", payload.EventID)
		req.Header.Set("X-SparkOps-Idempotency-Key", operation.IdempotencyKey)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 && resp.StatusCode != http.StatusTooManyRequests {
			return messaging.Permanent(fmt.Errorf("workflow webhook rejected with status %d", resp.StatusCode))
		}
		if resp.StatusCode >= 300 {
			return fmt.Errorf("workflow webhook returned status %d", resp.StatusCode)
		}
		return nil
	}
}

const (
	OperationTypeBuildTrigger  = "product-workflow.build.trigger.v1"
	OperationTypeDeployTrigger = "product-workflow.deploy.trigger.v1"
	OperationTypeWebhookSend   = "product-workflow.webhook.send.v1"
)

type CompletionPayload struct {
	EventID  string `json:"event_id"`
	TargetID string `json:"target_id"`
	Status   string `json:"status"`
}

type CompletionScheduler interface {
	HandleBuildCompleted(context.Context, kernel.BuildID, string) error
	HandleDeployCompleted(context.Context, kernel.DeploymentRunID, string) error
}

func NewBuildCompletionHandler(scheduler CompletionScheduler) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload CompletionPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode build completion operation: %w", err))
		}
		id, err := kernel.ParseBuildID(payload.TargetID)
		if err != nil {
			return messaging.Permanent(err)
		}
		return scheduler.HandleBuildCompleted(ctx, id, payload.Status)
	}
}

func NewDeployCompletionHandler(scheduler CompletionScheduler) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload CompletionPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode deploy completion operation: %w", err))
		}
		id, err := kernel.ParseDeploymentRunID(payload.TargetID)
		if err != nil {
			return messaging.Permanent(err)
		}
		return scheduler.HandleDeployCompleted(ctx, id, payload.Status)
	}
}
