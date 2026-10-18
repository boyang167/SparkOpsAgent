package command

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type operationPublisherTransaction struct {
	calls int
}

func (t *operationPublisherTransaction) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	t.calls++
	return fn(ctx)
}

func TestOperationRunPublisherSchedulesDurableQueuePublish(t *testing.T) {
	writer := &metadataCheckingOperationWriter{}
	tx := &operationPublisherTransaction{}
	publisher := NewOperationRunPublisher(writer, tx)
	run := model.NewDeploymentRun(
		kernel.NewWorkspaceID(), kernel.NewProjectID(), kernel.NewClusterInfoID(),
		kernel.NewMemberID(), "branch", "main", "main",
	)

	require.NoError(t, publisher.PublishRun(context.Background(), run))

	require.Equal(t, 1, tx.calls)
	require.Equal(t, "delivery.reconciler", writer.request.Owner)
	require.Equal(t, "delivery.queue.deploy.publish.v1", writer.request.OperationType)
	require.True(t, writer.request.Repeatable)
	require.Contains(t, writer.request.IdempotencyKey, run.ID.String())
	firstKey := writer.request.IdempotencyKey

	require.NoError(t, publisher.PublishRun(context.Background(), run))
	require.Equal(t, firstKey, writer.request.IdempotencyKey)
}
