package command

import (
	"context"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateCronJobHandler struct {
	repo    *persistence.CronJobRepository
	checker port.ProjectChecker
}

func NewCreateCronJobHandler(repo *persistence.CronJobRepository, checker port.ProjectChecker) *CreateCronJobHandler {
	return &CreateCronJobHandler{repo: repo, checker: checker}
}

func (h *CreateCronJobHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	name, schedule, branch string,
) (*persistence.CronJobRecord, error) {
	inWs, err := h.checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	name = strings.TrimSpace(name)
	schedule = strings.TrimSpace(schedule)
	if name == "" || schedule == "" {
		return nil, domain.ErrInvalidCronJob
	}
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}
	job := &persistence.CronJobRecord{
		ID: kernel.NewCronJobID(), ProjectID: projectID,
		Name: name, Schedule: schedule, Branch: branch, Enabled: true,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.repo.Save(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

type DeleteCronJobHandler struct {
	repo    *persistence.CronJobRepository
	checker port.ProjectChecker
}

func NewDeleteCronJobHandler(repo *persistence.CronJobRepository, checker port.ProjectChecker) *DeleteCronJobHandler {
	return &DeleteCronJobHandler{repo: repo, checker: checker}
}

func (h *DeleteCronJobHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	cronJobID kernel.CronJobID,
) error {
	inWs, err := h.checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return err
	}
	if !inWs {
		return domain.ErrProjectAccessDenied
	}
	return h.repo.Delete(ctx, cronJobID)
}
