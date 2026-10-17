package command

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

func TestReleasePlanStateRollsBackWhenOperationScheduleFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("CREATE TABLE release_plan_state (id TEXT PRIMARY KEY)").Error)

	tx := postgres.NewTransactor(db)
	writer := messaging.NewOperationWriter(messagingpersistence.NewOperationStore(db), nil)
	ctx := events.WithMetadata(context.Background(), events.Metadata{
		EventID: "event-1", Subscriber: "product.release-plan",
	})

	_, err = withinReleasePlanOperationTx(ctx, writer, tx, func(txCtx context.Context) (struct{}, error) {
		if err := postgres.DB(txCtx, db).Exec("INSERT INTO release_plan_state (id) VALUES ('plan-1')").Error; err != nil {
			return struct{}{}, err
		}
		return struct{}{}, writer.Schedule(txCtx, messaging.OperationRequest{
			Owner:          "product.release-plan",
			OperationType:  "product.release-plan.webhook.send.v1",
			IdempotencyKey: "event-1:product.release-plan:product.release-plan.webhook.send.v1:plan-1",
			Payload:        map[string]string{"plan_id": "plan-1"},
		})
	})

	require.Error(t, err)
	var count int64
	require.NoError(t, db.Table("release_plan_state").Count(&count).Error)
	require.Zero(t, count)
}

func TestPlanningExternalConfigRejectsMissingHookBeforeWaitState(t *testing.T) {
	event := model.ReleasePlanHookEventFinishPlanning
	plan := &model.ReleasePlan{
		Status: model.ReleasePlanStatusPlanning,
		HookSettings: model.ReleasePlanHookSettings{
			Enabled: true,
			Events: map[model.ReleasePlanHookEvent]model.ReleasePlanHookEventConfig{
				event: {Enabled: true},
			},
		},
	}

	err := validatePlanningExternalConfig(plan)

	require.Error(t, err)
	require.Equal(t, model.ReleasePlanStatusPlanning, plan.Status)
	require.Nil(t, plan.HookState)
}

func TestApprovalExternalConfigRejectsMissingURLBeforeTokenCreation(t *testing.T) {
	plan := &model.ReleasePlan{
		Status: model.ReleasePlanStatusWaitForApproval,
		ApprovalConfig: model.ApprovalConfig{
			Enabled: true, Provider: model.ApprovalProviderFeishu,
		},
	}

	err := validateApprovalExternalConfig(plan)

	require.Error(t, err)
	require.Nil(t, plan.ApprovalConfig.Instance)
}
