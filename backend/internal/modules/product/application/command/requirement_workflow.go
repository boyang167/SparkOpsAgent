package command

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ApproveRequirementHandler struct {
	repo repository.ProductRepository
}

func NewApproveRequirementHandler(repo repository.ProductRepository) *ApproveRequirementHandler {
	return &ApproveRequirementHandler{repo: repo}
}

func (h *ApproveRequirementHandler) Handle(ctx context.Context, productID kernel.ProductID, reqID kernel.ProductRequirementID) (*model.ProductRequirement, error) {
	req, err := loadRequirement(ctx, h.repo, productID, reqID)
	if err != nil {
		return nil, err
	}
	if req.Status != model.ProductRequirementStatusPending {
		return nil, domain.ErrInvalidRequirementTransition
	}
	req.Status = model.ProductRequirementStatusApproved
	if err := h.repo.UpdateRequirement(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

type RejectRequirementHandler struct {
	repo repository.ProductRepository
}

func NewRejectRequirementHandler(repo repository.ProductRepository) *RejectRequirementHandler {
	return &RejectRequirementHandler{repo: repo}
}

func (h *RejectRequirementHandler) Handle(ctx context.Context, productID kernel.ProductID, reqID kernel.ProductRequirementID, reason string) (*model.ProductRequirement, error) {
	req, err := loadRequirement(ctx, h.repo, productID, reqID)
	if err != nil {
		return nil, err
	}
	if req.Status != model.ProductRequirementStatusPending {
		return nil, domain.ErrInvalidRequirementTransition
	}
	req.Status = model.ProductRequirementStatusRejected
	if reason != "" {
		req.OtherInfo["reject_reason"] = reason
	}
	if err := h.repo.UpdateRequirement(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

type ReleaseRequirementHandler struct {
	repo    repository.ProductRepository
	trigger appctx.BuildTrigger
}

func NewReleaseRequirementHandler(repo repository.ProductRepository, trigger appctx.BuildTrigger) *ReleaseRequirementHandler {
	return &ReleaseRequirementHandler{repo: repo, trigger: trigger}
}

func (h *ReleaseRequirementHandler) Handle(
	ctx context.Context,
	productID kernel.ProductID,
	reqID kernel.ProductRequirementID,
) (*model.ProductRequirement, error) {
	req, err := loadRequirement(ctx, h.repo, productID, reqID)
	if err != nil {
		return nil, err
	}
	if req.Status != model.ProductRequirementStatusApproved {
		return nil, domain.ErrInvalidRequirementTransition
	}
	if h.trigger == nil {
		return nil, fmt.Errorf("build trigger not configured")
	}
	branch := req.ProjectTag
	if branch == "" {
		branch = "main"
	}
	buildID, err := h.trigger.TriggerAutomated(ctx, req.ProjectID, branch)
	if err != nil {
		return nil, err
	}
	req.BuildID = buildID
	req.Status = model.ProductRequirementStatusRunning
	if err := h.repo.UpdateRequirement(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

type CrossReleaseRequirementHandler struct {
	repo    repository.ProductRepository
	syncJob appctx.SyncJobCreator
}

func NewCrossReleaseRequirementHandler(repo repository.ProductRepository, syncJob appctx.SyncJobCreator) *CrossReleaseRequirementHandler {
	return &CrossReleaseRequirementHandler{repo: repo, syncJob: syncJob}
}

func (h *CrossReleaseRequirementHandler) Handle(
	ctx context.Context,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	reqID kernel.ProductRequirementID,
	sourceCluster, targetCluster string,
) (*model.ProductRequirement, error) {
	req, err := loadRequirement(ctx, h.repo, productID, reqID)
	if err != nil {
		return nil, err
	}
	if req.Status != model.ProductRequirementStatusApproved && req.Status != model.ProductRequirementStatusRunning {
		return nil, domain.ErrInvalidRequirementTransition
	}
	if h.syncJob == nil {
		return nil, fmt.Errorf("sync job creator not configured")
	}
	jobName := fmt.Sprintf("release-%s", req.ProjectName)
	jobID, err := h.syncJob.CreateSyncJob(ctx, memberID, wsID, jobName, req.ProjectTag, sourceCluster, targetCluster, "all")
	if err != nil {
		return nil, err
	}
	req.MulticlusterStatus = "sync_queued:" + jobID.String()
	if err := h.repo.UpdateRequirement(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

func loadRequirement(ctx context.Context, repo repository.ProductRepository, productID kernel.ProductID, reqID kernel.ProductRequirementID) (*model.ProductRequirement, error) {
	req, err := repo.FindRequirementByID(ctx, reqID)
	if err != nil {
		return nil, err
	}
	if req.ProductID != productID {
		return nil, domain.ErrProductRequirementNotFound
	}
	return req, nil
}
