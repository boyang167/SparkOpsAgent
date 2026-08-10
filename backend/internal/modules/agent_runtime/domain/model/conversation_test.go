package model_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestRun_Complete(t *testing.T) {
	run := model.NewRun(kernel.NewConversationID(), kernel.NewAgentVersionID(), "hello")
	run.Complete("world")
	require.Equal(t, model.RunStatusCompleted, run.Status)
	require.Equal(t, "world", run.Output)
	require.NotNil(t, run.CompletedAt)
}
