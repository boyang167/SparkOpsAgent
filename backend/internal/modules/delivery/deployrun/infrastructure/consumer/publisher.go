package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

type Publisher struct {
	queue messaging.Queue
}

func NewPublisher(queue messaging.Queue) *Publisher {
	return &Publisher{queue: queue}
}

func (p *Publisher) PublishRun(ctx context.Context, run *model.DeploymentRun) error {
	payload, err := json.Marshal(Message{RunID: run.ID.String(), WorkspaceID: run.WorkspaceID.String()})
	if err != nil {
		return fmt.Errorf("marshal deployment message: %w", err)
	}
	if err := p.queue.Publish(ctx, messaging.TopicDeploy, payload); err != nil {
		return fmt.Errorf("publish deployment message: %w", err)
	}
	return nil
}
