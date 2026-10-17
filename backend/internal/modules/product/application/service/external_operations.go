package service

import (
	"context"
	"fmt"

	productoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

func scheduleJiraTransition(ctx context.Context, writer messaging.OperationWriter, payload productoutbound.JiraTransitionPayload) error {
	metadata, ok := events.MetadataFromContext(ctx)
	if !ok {
		return fmt.Errorf("schedule jira transition: event metadata missing")
	}
	payload.EventID = metadata.EventID
	target := payload.Provider + "-" + payload.IssueKey
	return writer.Schedule(ctx, messaging.OperationRequest{
		Owner:          metadata.Subscriber,
		OperationType:  productoutbound.OperationTypeJiraTransition,
		IdempotencyKey: metadata.EventID + ":" + metadata.Subscriber + ":" + productoutbound.OperationTypeJiraTransition + ":" + target,
		Payload:        payload,
	})
}

func scheduleReleasePlanWebhook(ctx context.Context, writer messaging.OperationWriter, payload productoutbound.ReleasePlanWebhookPayload) error {
	metadata, ok := events.MetadataFromContext(ctx)
	if !ok {
		return fmt.Errorf("schedule release plan webhook: event metadata missing")
	}
	payload.EventID = metadata.EventID
	return writer.Schedule(ctx, messaging.OperationRequest{
		Owner:          metadata.Subscriber,
		OperationType:  productoutbound.OperationTypeReleasePlanWebhookSend,
		IdempotencyKey: metadata.EventID + ":" + metadata.Subscriber + ":" + productoutbound.OperationTypeReleasePlanWebhookSend + ":" + payload.PlanID,
		Payload:        payload,
	})
}
