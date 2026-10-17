package outbound

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	OperationTypeWorkflowTrigger        = "product.workflow.trigger.v1"
	OperationTypeJiraTransition         = "product.jira.transition.v1"
	OperationTypeReleasePlanWebhookSend = "product.release-plan.webhook.send.v1"
)

type WorkflowCompletionPayload struct {
	EventID      string `json:"event_id"`
	RunID        string `json:"run_id"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type JiraTransitionPayload struct {
	EventID       string `json:"event_id"`
	WorkspaceID   string `json:"workspace_id"`
	PMWorkspaceID string `json:"pm_workspace_id"`
	Provider      string `json:"provider"`
	IssueKey      string `json:"issue_key"`
	Status        string `json:"status"`
}

type ReleasePlanWebhookPayload struct {
	EventID string `json:"event_id"`
	PlanID  string `json:"plan_id"`
	Event   string `json:"event"`
}

type WorkflowCompletionHandler func(context.Context, kernel.ProductWorkflowRunID, bool, string) error

func NewWorkflowCompletionOperation(tx postgres.Transactor, handler WorkflowCompletionHandler) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload WorkflowCompletionPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode workflow completion operation: %w", err))
		}
		runID, err := kernel.ParseProductWorkflowRunID(payload.RunID)
		if err != nil {
			return messaging.Permanent(err)
		}
		return tx.WithinTransaction(ctx, func(txCtx context.Context) error {
			txCtx = events.WithMetadata(txCtx, events.Metadata{EventID: operation.EventID, Subscriber: operation.Owner})
			return handler(txCtx, runID, payload.Status == "success", payload.ErrorMessage)
		})
	}
}

type JiraTransitionHandler func(context.Context, JiraTransitionPayload) error

func NewJiraTransitionOperation(handler JiraTransitionHandler) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload JiraTransitionPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode jira transition operation: %w", err))
		}
		if payload.IssueKey == "" || payload.Status == "" {
			return messaging.Permanent(fmt.Errorf("jira transition target is required"))
		}
		return handler(ctx, payload)
	}
}

type ReleasePlanWebhookHandler func(context.Context, kernel.ReleasePlanID, string, string) error

func NewReleasePlanWebhookOperation(handler ReleasePlanWebhookHandler) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload ReleasePlanWebhookPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode release plan webhook operation: %w", err))
		}
		planID, err := kernel.ParseReleasePlanID(payload.PlanID)
		if err != nil {
			return messaging.Permanent(err)
		}
		return handler(ctx, planID, payload.Event, operation.IdempotencyKey)
	}
}
