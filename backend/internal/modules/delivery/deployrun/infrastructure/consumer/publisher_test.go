package consumer

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type publisherQueue struct {
	topic   string
	payload []byte
	err     error
}

func (q *publisherQueue) Publish(_ context.Context, topic string, payload []byte) error {
	q.topic, q.payload = topic, append([]byte(nil), payload...)
	return q.err
}
func (*publisherQueue) Subscribe(context.Context, string, func(context.Context, []byte) error) error {
	return nil
}
func (*publisherQueue) Close() error { return nil }

func TestPublisherEmitsCanonicalRunMessage(t *testing.T) {
	queue := &publisherQueue{}
	run := model.NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(), kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")
	require.NoError(t, NewPublisher(queue).PublishRun(context.Background(), run))
	require.Equal(t, messaging.TopicDeploy, queue.topic)
	require.JSONEq(t, `{"run_id":"`+run.ID.String()+`","workspace_id":"`+run.WorkspaceID.String()+`"}`, string(queue.payload))
}
