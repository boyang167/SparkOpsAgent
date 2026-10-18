package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httperror"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type NotificationPayload struct {
	EventID        string        `json:"event_id"`
	Target         string        `json:"target"`
	Channel        string        `json:"channel"`
	Targets        string        `json:"targets,omitempty"`
	WorkflowID     string        `json:"workflow_id"`
	WorkflowName   string        `json:"workflow_name"`
	RunID          string        `json:"run_id"`
	Scope          string        `json:"scope"`
	JobID          string        `json:"job_id,omitempty"`
	NoteID         string        `json:"note_id"`
	Status         string        `json:"status"`
	ErrorMessage   string        `json:"error_message,omitempty"`
	Event          string        `json:"event"`
	Timeout        time.Duration `json:"timeout"`
	IdempotencyKey string        `json:"-"`
}

type NotificationSender func(context.Context, NotificationPayload) error

type WebhookResolver interface {
	ResolveWebhook(context.Context, NotificationPayload) (string, json.RawMessage, bool, error)
}

func NewNotificationHandler(sender NotificationSender) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload NotificationPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode workflow notification operation: %w", err))
		}
		payload.IdempotencyKey = operation.IdempotencyKey
		return sender(ctx, payload)
	}
}

func NewWebhookHandler(resolver WebhookResolver, client *http.Client) messaging.OperationHandler {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload NotificationPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode workflow webhook operation: %w", err))
		}
		if resolver == nil {
			return messaging.Permanent(fmt.Errorf("workflow webhook resolver is required"))
		}
		endpoint, body, active, err := resolver.ResolveWebhook(ctx, payload)
		if err != nil {
			return err
		}
		if !active {
			return nil
		}
		timeout := payload.Timeout
		if timeout <= 0 {
			timeout = 8 * time.Second
		}
		sendCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return messaging.Permanent(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-SparkOps-Event-ID", payload.EventID)
		req.Header.Set("X-SparkOps-Idempotency-Key", operation.IdempotencyKey)
		resp, err := client.Do(req)
		if err != nil {
			return httperror.SanitizeNetwork(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 &&
			resp.StatusCode != http.StatusRequestTimeout &&
			resp.StatusCode != http.StatusTooManyRequests &&
			resp.StatusCode < http.StatusInternalServerError {
			return messaging.Permanent(fmt.Errorf("workflow webhook rejected with status %d", resp.StatusCode))
		}
		if resp.StatusCode >= 300 {
			return fmt.Errorf("workflow webhook returned status %d", resp.StatusCode)
		}
		return nil
	}
}

const (
	OperationTypeBuildTrigger       = "product-workflow.build.trigger.v1"
	OperationTypeDeployTrigger      = "product-workflow.deploy.trigger.v1"
	OperationTypeWebhookSend        = "product-workflow.webhook.send.v1"
	OperationTypeMailSend           = "product-workflow.mail.send.v1"
	OperationTypeFeishuUserSend     = "product-workflow.feishu-user.send.v1"
	OperationTypeGitLabStatusReport = "product-workflow.gitlab.status.report.v1"
)

type GitLabStatusPayload struct {
	EventID         string `json:"event_id"`
	RunID           string `json:"run_id"`
	ProjectID       string `json:"project_id"`
	Commit          string `json:"commit"`
	Context         string `json:"context"`
	DesiredStatus   string `json:"desired_status"`
	Description     string `json:"description"`
	MergeRequestIID int    `json:"merge_request_iid,omitempty"`
}

type GitLabStatusReporter interface {
	CurrentStatus(context.Context, GitLabStatusPayload) (string, error)
	Report(context.Context, GitLabStatusPayload) error
}

func NewGitLabStatusHandler(reporter GitLabStatusReporter) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		if reporter == nil {
			return messaging.Permanent(fmt.Errorf("GitLab status reporter is required"))
		}
		var payload GitLabStatusPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode GitLab status operation: %w", err))
		}
		if payload.RunID == "" || payload.ProjectID == "" || payload.Commit == "" || payload.Context == "" || payload.DesiredStatus == "" {
			return messaging.Permanent(fmt.Errorf("GitLab status operation target is incomplete"))
		}
		current, err := reporter.CurrentStatus(ctx, payload)
		if err != nil {
			return err
		}
		if current == payload.DesiredStatus {
			return nil
		}
		return reporter.Report(ctx, payload)
	}
}

type CompletionPayload struct {
	EventID  string `json:"event_id"`
	TargetID string `json:"target_id"`
	Status   string `json:"status"`
}

type CompletionScheduler interface {
	HandleBuildCompleted(context.Context, kernel.BuildID, string) error
	HandleDeployCompleted(context.Context, kernel.DeploymentRunID, string) error
}

func NewBuildCompletionHandler(tx postgres.Transactor, scheduler CompletionScheduler) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload CompletionPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode build completion operation: %w", err))
		}
		id, err := kernel.ParseBuildID(payload.TargetID)
		if err != nil {
			return messaging.Permanent(err)
		}
		return tx.WithinTransaction(ctx, func(txCtx context.Context) error {
			txCtx = events.WithMetadata(txCtx, events.Metadata{EventID: operation.EventID, Subscriber: operation.Owner})
			return scheduler.HandleBuildCompleted(txCtx, id, payload.Status)
		})
	}
}

func NewDeployCompletionHandler(tx postgres.Transactor, scheduler CompletionScheduler) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload CompletionPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode deploy completion operation: %w", err))
		}
		id, err := kernel.ParseDeploymentRunID(payload.TargetID)
		if err != nil {
			return messaging.Permanent(err)
		}
		return tx.WithinTransaction(ctx, func(txCtx context.Context) error {
			txCtx = events.WithMetadata(txCtx, events.Metadata{EventID: operation.EventID, Subscriber: operation.Owner})
			return scheduler.HandleDeployCompleted(txCtx, id, payload.Status)
		})
	}
}
