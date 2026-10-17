package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateAgentHandler struct {
	repo           repository.AgentRepository
	projectChecker port.ProjectChecker
	quotaChecker   port.QuotaChecker
	bus            eventbus.Bus
	tx             postgres.Transactor
}

func NewCreateAgentHandler(
	repo repository.AgentRepository,
	projectChecker port.ProjectChecker,
	quotaChecker port.QuotaChecker,
	bus eventbus.Bus,
	tx postgres.Transactor,
) *CreateAgentHandler {
	return &CreateAgentHandler{repo: repo, projectChecker: projectChecker, quotaChecker: quotaChecker, bus: bus, tx: tx}
}

func (h *CreateAgentHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	rawName, prompt, modelName string,
) (*model.Agent, error) {
	active, err := h.projectChecker.ProjectIsActive(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, domain.ErrProjectNotActive
	}
	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}

	count, err := h.repo.CountByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	if err := h.quotaChecker.CanCreateAgent(ctx, wsID, count); err != nil {
		return nil, err
	}

	name, err := model.NewAgentName(rawName)
	if err != nil {
		return nil, err
	}
	if existing, _ := h.repo.FindByProjectAndName(ctx, projectID, name); existing != nil {
		return nil, domain.ErrAgentNameExists
	}

	agent := model.NewAgent(projectID, name)
	if _, err := agent.AddVersion(prompt, modelName, model.DefaultModelConfig(), nil, nil); err != nil {
		return nil, err
	}
	domainEvents := agent.PullEvents()
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Save(txCtx, agent); err != nil {
			return err
		}
		if len(domainEvents) == 0 {
			return nil
		}
		return h.bus.Publish(txCtx, domainEvents...)
	})
	if err != nil {
		return nil, err
	}
	return agent, nil
}

type AddVersionHandler struct {
	repo           repository.AgentRepository
	projectChecker port.ProjectChecker
}

func NewAddVersionHandler(repo repository.AgentRepository, projectChecker port.ProjectChecker) *AddVersionHandler {
	return &AddVersionHandler{repo: repo, projectChecker: projectChecker}
}

func (h *AddVersionHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	agentID kernel.AgentID,
	prompt, modelName string,
	cfg model.ModelConfig,
	tools, knowledgeBaseIDs []string,
) (*model.AgentVersion, error) {
	agent, err := h.repo.FindByID(ctx, agentID)
	if err != nil {
		return nil, err
	}
	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, agent.ProjectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	ver, err := agent.AddVersion(prompt, modelName, cfg, tools, knowledgeBaseIDs)
	if err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, agent); err != nil {
		return nil, err
	}
	return ver, nil
}

type PublishAgentHandler struct {
	repo           repository.AgentRepository
	projectChecker port.ProjectChecker
	bus            eventbus.Bus
	tx             postgres.Transactor
}

func NewPublishAgentHandler(repo repository.AgentRepository, projectChecker port.ProjectChecker, bus eventbus.Bus, tx postgres.Transactor) *PublishAgentHandler {
	return &PublishAgentHandler{repo: repo, projectChecker: projectChecker, bus: bus, tx: tx}
}

func (h *PublishAgentHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, agentID kernel.AgentID, versionID kernel.AgentVersionID) error {
	agent, err := h.repo.FindByID(ctx, agentID)
	if err != nil {
		return err
	}
	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, agent.ProjectID, wsID)
	if err != nil {
		return err
	}
	if !inWs {
		return domain.ErrProjectAccessDenied
	}
	ev, err := agent.Publish(versionID)
	if err != nil {
		return err
	}
	return h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Save(txCtx, agent); err != nil {
			return err
		}
		return h.bus.Publish(txCtx, ev)
	})
}

type UpdateAgentHandler struct {
	repo           repository.AgentRepository
	projectChecker port.ProjectChecker
}

func NewUpdateAgentHandler(repo repository.AgentRepository, projectChecker port.ProjectChecker) *UpdateAgentHandler {
	return &UpdateAgentHandler{repo: repo, projectChecker: projectChecker}
}

func (h *UpdateAgentHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	agentID kernel.AgentID,
	rawName string,
) (*model.Agent, error) {
	agent, err := h.repo.FindByID(ctx, agentID)
	if err != nil {
		return nil, err
	}
	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, agent.ProjectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	name, err := model.NewAgentName(rawName)
	if err != nil {
		return nil, err
	}
	if name.String() != agent.Name.String() {
		if existing, _ := h.repo.FindByProjectAndName(ctx, agent.ProjectID, name); existing != nil {
			return nil, domain.ErrAgentNameExists
		}
	}
	if err := agent.Rename(name); err != nil {
		return nil, err
	}
	if err := h.repo.Update(ctx, agent); err != nil {
		return nil, err
	}
	return agent, nil
}

type ArchiveAgentHandler struct {
	repo           repository.AgentRepository
	projectChecker port.ProjectChecker
}

func NewArchiveAgentHandler(repo repository.AgentRepository, projectChecker port.ProjectChecker) *ArchiveAgentHandler {
	return &ArchiveAgentHandler{repo: repo, projectChecker: projectChecker}
}

func (h *ArchiveAgentHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, agentID kernel.AgentID) error {
	agent, err := h.repo.FindByID(ctx, agentID)
	if err != nil {
		return err
	}
	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, agent.ProjectID, wsID)
	if err != nil {
		return err
	}
	if !inWs {
		return domain.ErrProjectAccessDenied
	}
	if err := agent.Archive(); err != nil {
		return err
	}
	return h.repo.Update(ctx, agent)
}
