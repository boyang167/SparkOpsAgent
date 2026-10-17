package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type hookOperationPlanLoader struct {
	plan *model.ReleasePlan
	err  error
}

func (l hookOperationPlanLoader) FindPlanByID(context.Context, kernel.ReleasePlanID) (*model.ReleasePlan, error) {
	return l.plan, l.err
}

type hookOperationSender struct {
	calls     int
	seenToken string
	err       error
}

func (s *hookOperationSender) SendWithIdempotency(
	_ context.Context, plan *model.ReleasePlan, _ model.ReleasePlanHookEvent, _ string,
) error {
	s.calls++
	if plan.HookState != nil {
		s.seenToken = plan.HookState.Token
	}
	return s.err
}

func TestReleasePlanHookOperationCompletesAfterCallbackClearsPendingState(t *testing.T) {
	plan := &model.ReleasePlan{
		ID: kernel.NewReleasePlanID(),
		HookState: &model.ReleasePlanHookState{
			Event: model.ReleasePlanHookEventFinishPlanning, Token: "hook-token", Status: "pending",
		},
	}
	sender := &hookOperationSender{}

	require.NoError(t, SendReleasePlanHookOperation(
		context.Background(), hookOperationPlanLoader{plan: plan}, sender,
		plan.ID, model.ReleasePlanHookEventFinishPlanning, ReleasePlanHookTokenDigest("hook-token"), "operation-key",
	))
	require.Equal(t, 1, sender.calls, "first delivery succeeds before operation completion persistence")
	require.Equal(t, "hook-token", sender.seenToken, "raw capability is loaded only at handler execution")

	// The external callback advances the business flow before MarkCompleted can persist.
	plan.ClearHookState()
	require.NoError(t, SendReleasePlanHookOperation(
		context.Background(), hookOperationPlanLoader{plan: plan}, sender,
		plan.ID, model.ReleasePlanHookEventFinishPlanning, ReleasePlanHookTokenDigest("hook-token"), "operation-key",
	))
	require.Equal(t, 1, sender.calls, "operation retry must converge without a duplicate send")
}

func TestReleasePlanHookOperationRejectsInvalidMatchingPendingConfiguration(t *testing.T) {
	plan := &model.ReleasePlan{
		ID: kernel.NewReleasePlanID(),
		HookSettings: model.ReleasePlanHookSettings{
			Enabled: true,
			Events: map[model.ReleasePlanHookEvent]model.ReleasePlanHookEventConfig{
				model.ReleasePlanHookEventFinishPlanning: {Enabled: true, URL: "file:///tmp/hook"},
			},
		},
		HookState: &model.ReleasePlanHookState{
			Event: model.ReleasePlanHookEventFinishPlanning, Token: "hook-token", Status: "pending",
		},
	}

	err := SendReleasePlanHookOperation(
		context.Background(), hookOperationPlanLoader{plan: plan},
		NewReleasePlanHookNotifier("https://sparkops.example"), plan.ID,
		model.ReleasePlanHookEventFinishPlanning, ReleasePlanHookTokenDigest("hook-token"), "operation-key",
	)

	require.Error(t, err)
	require.True(t, messaging.IsPermanent(err))
}

func TestReleasePlanHookOperationIgnoresSupersededTokenOrEvent(t *testing.T) {
	plan := &model.ReleasePlan{
		ID: kernel.NewReleasePlanID(),
		HookState: &model.ReleasePlanHookState{
			Event: model.ReleasePlanHookEventBeforeExecute, Token: "new-token", Status: "pending",
		},
	}
	sender := &hookOperationSender{}

	require.NoError(t, SendReleasePlanHookOperation(
		context.Background(), hookOperationPlanLoader{plan: plan}, sender,
		plan.ID, model.ReleasePlanHookEventFinishPlanning, ReleasePlanHookTokenDigest("old-token"), "operation-key",
	))
	require.Zero(t, sender.calls)
}

func TestReleasePlanHookOperationKeepsDatabaseFailureRetryable(t *testing.T) {
	err := SendReleasePlanHookOperation(
		context.Background(), hookOperationPlanLoader{err: errors.New("database unavailable")},
		&hookOperationSender{}, kernel.NewReleasePlanID(),
		model.ReleasePlanHookEventFinishPlanning, ReleasePlanHookTokenDigest("hook-token"), "operation-key",
	)

	require.Error(t, err)
	require.False(t, messaging.IsPermanent(err))
}
