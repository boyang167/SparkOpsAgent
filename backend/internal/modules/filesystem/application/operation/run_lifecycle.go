package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

const OperationTypeRunLifecycle = "filesystem.run-lifecycle.v1"

type RunLifecyclePayload struct {
	PolicyID      string                  `json:"policy_id"`
	PolicyVersion int64                   `json:"policy_version"`
	Candidate     service.LifecycleCandidate `json:"candidate"`
}

type lifecyclePolicyRepository interface {
	GetLifecyclePolicy(context.Context, uuid.UUID, int64) (*model.LifecyclePolicy, error)
	SaveLifecycleJobCheckpoint(context.Context, uuid.UUID, map[string]any) error
}

type RunLifecycleHandler struct {
	repo   lifecyclePolicyRepository
	engine *service.LifecycleEngine
}

func NewRunLifecycleHandler(repo lifecyclePolicyRepository, engine *service.LifecycleEngine) *RunLifecycleHandler {
	if engine == nil {
		engine = service.NewLifecycleEngine()
	}
	return &RunLifecycleHandler{repo: repo, engine: engine}
}

func (h *RunLifecycleHandler) Handle(ctx context.Context, operation messaging.Operation) error {
	var payload RunLifecyclePayload
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return messaging.Permanent(fmt.Errorf("decode run lifecycle operation: %w", err))
	}
	policyID, err := uuid.Parse(strings.TrimSpace(payload.PolicyID))
	if err != nil {
		return messaging.Permanent(err)
	}
	if h.repo == nil {
		return fmt.Errorf("run lifecycle repository is required")
	}
	policy, err := h.repo.GetLifecyclePolicy(ctx, policyID, payload.PolicyVersion)
	if err != nil {
		return err
	}
	if policy.Version != payload.PolicyVersion {
		return messaging.Permanent(fmt.Errorf("lifecycle policy version mismatch"))
	}
	decisions, err := h.engine.Evaluate(policy, payload.Candidate)
	if err != nil {
		return err
	}
	if len(decisions) == 0 {
		return nil
	}
	checkpoint := map[string]any{
		"policy_id": policyID.String(), "policy_version": payload.PolicyVersion,
		"decisions": decisions, "candidate_scope": payload.Candidate.ScopeType,
	}
	jobID := uuid.NewSHA1(uuid.NameSpaceURL, operation.Payload)
	return h.repo.SaveLifecycleJobCheckpoint(ctx, jobID, checkpoint)
}
