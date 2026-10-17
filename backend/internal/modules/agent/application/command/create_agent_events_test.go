package command_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type createAgentNoEventRepo struct {
	repository.AgentRepository
	saved bool
	agent *model.Agent
}

func (*createAgentNoEventRepo) CountByWorkspace(context.Context, kernel.WorkspaceID) (int, error) {
	return 0, nil
}

func (*createAgentNoEventRepo) FindByProjectAndName(
	context.Context,
	kernel.ProjectID,
	model.AgentName,
) (*model.Agent, error) {
	return nil, domain.ErrAgentNotFound
}

func (r *createAgentNoEventRepo) Save(context.Context, *model.Agent) error {
	r.saved = true
	return nil
}

func (r *createAgentNoEventRepo) FindByID(context.Context, kernel.AgentID) (*model.Agent, error) {
	return r.agent, nil
}

type createAgentChecker struct{}

func (createAgentChecker) ProjectInWorkspace(context.Context, kernel.ProjectID, kernel.WorkspaceID) (bool, error) {
	return true, nil
}

func (createAgentChecker) ProjectIsActive(context.Context, kernel.ProjectID) (bool, error) {
	return true, nil
}

type createAgentQuota struct{}

func (createAgentQuota) CanCreateAgent(context.Context, kernel.WorkspaceID, int) error {
	return nil
}

func TestCreateAgentWithNoDomainEventsDoesNotRequireBus(t *testing.T) {
	repo := &createAgentNoEventRepo{}
	handler := command.NewCreateAgentHandler(
		repo,
		createAgentChecker{},
		createAgentQuota{},
		nil,
		postgres.NewDirectTransactor(),
	)

	created, err := handler.Handle(
		context.Background(),
		kernel.NewWorkspaceID(),
		kernel.NewProjectID(),
		"release-agent",
		"prompt",
		"model",
	)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.True(t, repo.saved)
}

func TestPublishingExistingDomainEventRequiresConfiguredBus(t *testing.T) {
	name, err := model.NewAgentName("release-agent")
	require.NoError(t, err)
	agent := model.NewAgent(kernel.NewProjectID(), name)
	version, err := agent.AddVersion("prompt", "model", model.DefaultModelConfig(), nil, nil)
	require.NoError(t, err)

	repo := &createAgentNoEventRepo{agent: agent}
	handler := command.NewPublishAgentHandler(
		repo,
		createAgentChecker{},
		nil,
		postgres.NewDirectTransactor(),
	)

	require.Panics(t, func() {
		_ = handler.Handle(context.Background(), kernel.NewWorkspaceID(), agent.ID, version.ID)
	})
}
