package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Message struct {
	RunID       string `json:"run_id"`
	WorkspaceID string `json:"workspace_id"`
}

type Stepper interface {
	Step(context.Context, kernel.DeploymentRunID, kernel.WorkspaceID) error
}

var ErrStepperUnavailable = errors.New("deployment stepper unavailable")

type Consumer struct {
	repo     repository.DeploymentRunRepository
	stepper  Stepper
	mu       sync.Mutex
	inFlight map[string]struct{}
}

func New(repo repository.DeploymentRunRepository, stepper Stepper) (*Consumer, error) {
	if stepper == nil {
		return nil, ErrStepperUnavailable
	}
	return &Consumer{repo: repo, stepper: stepper, inFlight: make(map[string]struct{})}, nil
}

func (c *Consumer) Register(ctx context.Context, queue messaging.Queue) error {
	return queue.Subscribe(ctx, messaging.TopicDeploy, c.HandlePayload)
}

func (c *Consumer) HandlePayload(ctx context.Context, payload []byte) error {
	var message Message
	if err := json.Unmarshal(payload, &message); err != nil {
		return fmt.Errorf("decode deploy message: %w", err)
	}
	return c.Handle(ctx, message)
}

func (c *Consumer) Handle(ctx context.Context, message Message) error {
	runID, err := kernel.ParseDeploymentRunID(message.RunID)
	if err != nil {
		return fmt.Errorf("parse deploy run id: %w", err)
	}
	wsID, err := kernel.ParseWorkspaceID(message.WorkspaceID)
	if err != nil {
		return fmt.Errorf("parse deploy workspace id: %w", err)
	}
	run, err := c.repo.FindByID(ctx, runID)
	if errors.Is(err, domain.ErrDeploymentRunNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if run.WorkspaceID != wsID || isTerminal(run.Status) {
		return nil
	}

	// The guard only covers a currently executing Step. It deliberately does not
	// permanently deduplicate run IDs: a later delivery must re-read DB state so
	// the stepper can idempotently advance the same run through a later stage.
	key := run.ID.String()
	c.mu.Lock()
	if _, exists := c.inFlight[key]; exists {
		c.mu.Unlock()
		return nil
	}
	c.inFlight[key] = struct{}{}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.inFlight, key)
		c.mu.Unlock()
	}()
	return c.stepper.Step(ctx, run.ID, wsID)
}

func isTerminal(status model.Status) bool {
	return status == model.StatusSucceeded || status == model.StatusFailed || status == model.StatusAborted
}
