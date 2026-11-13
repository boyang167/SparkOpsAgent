package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type lifecyclePolicyWriter interface {
	SaveLifecyclePolicy(context.Context, *model.LifecyclePolicy) error
}

type UpsertLifecyclePolicy struct {
	CommandID      uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Version        int64
	Rules          map[string]any
	Status         model.LifecyclePolicyStatus
}

type UpsertLifecyclePolicyResult struct {
	ID      uuid.UUID
	Version int64
}

type UpsertLifecyclePolicyHandler struct {
	repo lifecyclePolicyWriter
}

func NewUpsertLifecyclePolicyHandler(repo lifecyclePolicyWriter) *UpsertLifecyclePolicyHandler {
	if repo == nil {
		panic("upsert lifecycle policy requires repository")
	}
	return &UpsertLifecyclePolicyHandler{repo: repo}
}

func (h *UpsertLifecyclePolicyHandler) Handle(ctx context.Context, cmd UpsertLifecyclePolicy) (UpsertLifecyclePolicyResult, error) {
	policy, err := model.NewLifecyclePolicy(model.LifecyclePolicyInput{
		ID: uuid.New(), OrganizationID: cmd.OrganizationID, Name: cmd.Name,
		Version: cmd.Version, Rules: cmd.Rules, Status: cmd.Status,
	})
	if err != nil {
		return UpsertLifecyclePolicyResult{}, err
	}
	if err := h.repo.SaveLifecyclePolicy(ctx, policy); err != nil {
		return UpsertLifecyclePolicyResult{}, err
	}
	return UpsertLifecyclePolicyResult{ID: policy.ID, Version: policy.Version}, nil
}
