package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

func TestValidateReleasePlanHookRejectsEnabledMissingOrInvalidURL(t *testing.T) {
	event := model.ReleasePlanHookEventBeforeExecute
	for _, rawURL := range []string{"", "file:///tmp/hook", "https://"} {
		plan := &model.ReleasePlan{HookSettings: model.ReleasePlanHookSettings{
			Enabled: true,
			Events: map[model.ReleasePlanHookEvent]model.ReleasePlanHookEventConfig{
				event: {Enabled: true, URL: rawURL},
			},
		}}
		err := ValidateReleasePlanHook(plan, event)
		require.Error(t, err, rawURL)
	}
}

func TestValidateReleasePlanApprovalRejectsMissingOrInvalidURL(t *testing.T) {
	for _, rawURL := range []string{"", "javascript:alert(1)", "https://"} {
		plan := &model.ReleasePlan{ApprovalConfig: model.ApprovalConfig{
			Enabled: true, Provider: model.ApprovalProviderFeishu, WebhookURL: rawURL,
		}}
		err := ValidateReleasePlanApproval(plan)
		require.Error(t, err, rawURL)
	}
}

func TestReleasePlanNotifiersClassifyInvalidRuntimeConfigPermanent(t *testing.T) {
	hookPlan := &model.ReleasePlan{
		HookSettings: model.ReleasePlanHookSettings{
			Enabled: true,
			Events: map[model.ReleasePlanHookEvent]model.ReleasePlanHookEventConfig{
				model.ReleasePlanHookEventAllDone: {Enabled: true, URL: "file:///tmp/hook"},
			},
		},
		HookState: &model.ReleasePlanHookState{Event: model.ReleasePlanHookEventAllDone, Token: "token"},
	}
	err := NewReleasePlanHookNotifier("http://localhost").Send(
		context.Background(), hookPlan, model.ReleasePlanHookEventAllDone,
	)
	require.Error(t, err)
	require.True(t, messaging.IsPermanent(err))

	approvalPlan := &model.ReleasePlan{ApprovalConfig: model.ApprovalConfig{
		Enabled: true, Provider: model.ApprovalProviderFeishu, WebhookURL: "",
		Instance: &model.ApprovalInstance{Token: "token"},
	}}
	err = NewReleasePlanApprovalNotifier("http://localhost").NotifyApproval(context.Background(), approvalPlan)
	require.Error(t, err)
	require.True(t, messaging.IsPermanent(err))
}
