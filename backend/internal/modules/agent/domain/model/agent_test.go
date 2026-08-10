package model_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func mustAgentName(t *testing.T, raw string) model.AgentName {
	t.Helper()
	name, err := model.NewAgentName(raw)
	require.NoError(t, err)
	return name
}

func TestAgent_Publish_Success(t *testing.T) {
	projectID := kernel.NewProjectID()
	agent := model.NewAgent(projectID, mustAgentName(t, "bot"))
	ver, err := agent.AddVersion("you are helpful", "gpt-4o", model.DefaultModelConfig(), nil, nil)
	require.NoError(t, err)

	ev, err := agent.Publish(ver.ID)
	require.NoError(t, err)
	require.Equal(t, model.AgentStatusPublished, agent.Status)
	require.Equal(t, ver.ID, *agent.CurrentVersionID)
	require.Equal(t, agent.ID, ev.AgentID)
}

func TestAgent_Publish_UnknownVersion(t *testing.T) {
	agent := model.NewAgent(kernel.NewProjectID(), mustAgentName(t, "bot"))
	_, err := agent.Publish(kernel.NewAgentVersionID())
	require.ErrorIs(t, err, domain.ErrVersionNotFound)
}

func TestAgent_AddVersion_AfterArchived(t *testing.T) {
	agent := model.NewAgent(kernel.NewProjectID(), mustAgentName(t, "bot"))
	require.NoError(t, agent.Archive())
	_, err := agent.AddVersion("x", "gpt-4o", model.DefaultModelConfig(), nil, nil)
	require.ErrorIs(t, err, domain.ErrAgentArchived)
}

func TestAgent_Rename(t *testing.T) {
	agent := model.NewAgent(kernel.NewProjectID(), mustAgentName(t, "bot"))
	require.NoError(t, agent.Rename(mustAgentName(t, "new-bot")))
	require.Equal(t, "new-bot", agent.Name.String())
}
