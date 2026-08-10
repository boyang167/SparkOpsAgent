package webhook

import (
	"context"
	"encoding/json"
	"strings"

	"go.uber.org/zap"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type BuildTrigger interface {
	TriggerAutomated(ctx context.Context, projectID kernel.ProjectID, branch string) error
}

type triggerAdapter struct {
	handler *command.TriggerBuildHandler
}

func (a triggerAdapter) TriggerAutomated(ctx context.Context, projectID kernel.ProjectID, branch string) error {
	_, err := a.handler.TriggerAutomated(ctx, projectID, branch)
	return err
}

type BuildConsumer struct {
	logger  *zap.Logger
	trigger BuildTrigger
}

func NewBuildConsumer(logger *zap.Logger, trigger *command.TriggerBuildHandler) *BuildConsumer {
	if logger == nil {
		logger = zap.NewNop()
	}
	var t BuildTrigger
	if trigger != nil {
		t = triggerAdapter{handler: trigger}
	}
	return &BuildConsumer{logger: logger, trigger: t}
}

func (c *BuildConsumer) Register(ctx context.Context, q messaging.Queue) error {
	if q == nil {
		return nil
	}
	if err := q.Subscribe(ctx, messaging.TopicBuild, c.handle); err != nil {
		return err
	}
	return q.Subscribe(ctx, messaging.TopicHook, c.handle)
}

type hookPayload struct {
	ProjectID string `json:"project_id"`
	Branch    string `json:"branch"`
	Event     string `json:"event"`
}

func (c *BuildConsumer) handle(ctx context.Context, payload []byte) error {
	var msg hookPayload
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.logger.Warn("webhook payload is not json", zap.ByteString("payload", payload))
		return nil
	}
	if c.trigger == nil {
		c.logger.Info("webhook received (no trigger wired)", zap.Any("payload", msg))
		return nil
	}
	projectID, err := kernel.ParseProjectID(strings.TrimSpace(msg.ProjectID))
	if err != nil {
		c.logger.Warn("webhook missing project_id", zap.Any("payload", msg))
		return nil
	}
	branch := strings.TrimSpace(msg.Branch)
	if branch == "" {
		branch = "main"
	}
	if err := c.trigger.TriggerAutomated(ctx, projectID, branch); err != nil {
		c.logger.Warn("webhook trigger build failed", zap.Error(err), zap.String("project_id", projectID.String()))
		return nil
	}
	c.logger.Info("webhook triggered build", zap.String("project_id", projectID.String()), zap.String("branch", branch))
	return nil
}
