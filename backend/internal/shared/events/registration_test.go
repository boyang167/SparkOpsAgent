package events_test

import (
	"encoding/json"
	"testing"

	agentevent "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/event"
	runtimeevent "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/event"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	identityevent "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/event"
	productworkflowevent "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/event"
	workflowevent "github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/event"
	workspaceevent "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
	"github.com/stretchr/testify/require"
)

func TestEventOwnersRegisterConcreteValueCodecs(t *testing.T) {
	registry := events.NewRegistry()
	registrations := []func(*events.Registry) error{
		identityevent.Register,
		integration.Register,
		workspaceevent.Register,
		agentevent.Register,
		deliveryevent.Register,
		workflowevent.Register,
		productworkflowevent.Register,
		runtimeevent.Register,
	}
	for _, register := range registrations {
		require.NoError(t, register(registry))
	}

	tests := []struct {
		name string
		want events.Event
	}{
		{identityevent.MemberRegisteredName, identityevent.MemberRegistered{}},
		{integration.EventWorkspaceCreated, integration.WorkspaceCreated{}},
		{workspaceevent.ProjectCreatedName, workspaceevent.ProjectCreated{}},
		{integration.EventSecretUpdated, integration.SecretUpdated{}},
		{agentevent.AgentPublishedName, agentevent.AgentPublished{}},
		{deliveryevent.BuildCompletedName, deliveryevent.BuildCompleted{}},
		{deliveryevent.DeployCompletedName, deliveryevent.DeployCompleted{}},
		{workflowevent.WorkflowRunCompletedName, workflowevent.WorkflowRunCompleted{}},
		{productworkflowevent.WorkflowRunCompletedName, productworkflowevent.WorkflowRunCompleted{}},
		{runtimeevent.RunCompletedName, runtimeevent.RunCompleted{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := registry.Decode(events.Envelope{
				Type:    tt.name,
				Version: 1,
				Payload: json.RawMessage(`{}`),
			})
			require.NoError(t, err)
			require.IsType(t, tt.want, decoded)
		})
	}

	require.ErrorContains(t, deliveryevent.Register(registry), "duplicate")
}
