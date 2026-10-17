package engine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type webhookResolverRepo struct {
	repository.ProductWorkflowRepository
	run         *model.ProductWorkflowRun
	workflow    *model.ProductWorkflow
	runErr      error
	workflowErr error
}

func (r webhookResolverRepo) FindRunByID(context.Context, kernel.ProductWorkflowRunID) (*model.ProductWorkflowRun, error) {
	return r.run, r.runErr
}

func (r webhookResolverRepo) FindWorkflowByID(context.Context, kernel.ProductWorkflowID) (*model.ProductWorkflow, error) {
	return r.workflow, r.workflowErr
}

func TestWebhookOperationCompletesWhenWorkflowWasDeleted(t *testing.T) {
	runID := kernel.NewProductWorkflowRunID()
	workflowID := kernel.NewProductWorkflowID()
	resolver := NewWorkflowWebhookResolver(webhookResolverRepo{
		run:         &model.ProductWorkflowRun{ID: runID, WorkflowID: workflowID},
		workflowErr: domain.ErrProductWorkflowNotFound,
	})
	payload, err := json.Marshal(workflowoutbound.NotificationPayload{
		RunID: runID.String(), WorkflowID: workflowID.String(), NoteID: "hook-a",
	})
	require.NoError(t, err)

	err = workflowoutbound.NewWebhookHandler(resolver, nil)(context.Background(), messaging.Operation{Payload: payload})

	require.NoError(t, err)
}

func TestWebhookResolverKeepsDatabaseFailureRetryable(t *testing.T) {
	runID := kernel.NewProductWorkflowRunID()
	dbErr := errors.New("database unavailable")
	resolver := NewWorkflowWebhookResolver(webhookResolverRepo{runErr: dbErr})

	_, _, _, err := resolver.ResolveWebhook(context.Background(), workflowoutbound.NotificationPayload{
		RunID: runID.String(),
	})

	require.ErrorIs(t, err, dbErr)
	require.False(t, messaging.IsPermanent(err))
}
