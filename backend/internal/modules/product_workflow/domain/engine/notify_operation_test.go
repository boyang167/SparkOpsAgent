package engine

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type notificationOperationWriter struct {
	requests []messaging.OperationRequest
	err      error
}

func (w *notificationOperationWriter) Schedule(_ context.Context, request messaging.OperationRequest) error {
	w.requests = append(w.requests, request)
	return w.err
}

func TestEmitNotesSchedulesOneOperationPerExternalTarget(t *testing.T) {
	writer := &notificationOperationWriter{}
	scheduler := &Scheduler{operations: writer}
	run := &model.ProductWorkflowRun{ID: kernel.NewProductWorkflowRunID(), Status: model.RunStatusSuccess}
	wf := &model.ProductWorkflow{Name: "release"}
	ctx := events.WithMetadata(context.Background(), events.Metadata{
		EventID: "event-1", Subscriber: "product-workflow.build-completed",
	})
	notes := []model.WorkflowNotification{
		{ID: "hook-a", Channel: model.NotifyChannelWebhook, WebhookURL: "https://example.invalid/a", Events: []string{model.NotifyEventPassed}},
		{ID: "mail-a", Channel: model.NotifyChannelMail, Targets: "a@example.com", Events: []string{model.NotifyEventPassed}},
		{ID: "feishu-a", Channel: model.NotifyChannelFeishuUser, Targets: "ou_1", Events: []string{model.NotifyEventPassed}},
	}

	err := scheduler.emitNotes(ctx, wf, run, notes, model.NotifyEventPassed)

	require.NoError(t, err)
	require.Len(t, writer.requests, 3)
	require.Equal(t, workflowoutbound.OperationTypeWebhookSend, writer.requests[0].OperationType)
	require.Equal(t, workflowoutbound.OperationTypeMailSend, writer.requests[1].OperationType)
	require.Equal(t, workflowoutbound.OperationTypeFeishuUserSend, writer.requests[2].OperationType)
	require.Equal(t,
		"event-1:product-workflow.build-completed:"+workflowoutbound.OperationTypeWebhookSend+":hook-a",
		writer.requests[0].IdempotencyKey,
	)
}

func TestEmitNotesReturnsScheduleFailure(t *testing.T) {
	writer := &notificationOperationWriter{err: context.DeadlineExceeded}
	scheduler := &Scheduler{operations: writer}
	ctx := events.WithMetadata(context.Background(), events.Metadata{
		EventID: "event-1", Subscriber: "product-workflow.deploy-completed",
	})

	err := scheduler.emitNotes(ctx, &model.ProductWorkflow{Name: "release"},
		&model.ProductWorkflowRun{ID: kernel.NewProductWorkflowRunID()},
		[]model.WorkflowNotification{{
			ID: "hook-a", Channel: model.NotifyChannelWebhook, WebhookURL: "https://example.invalid/a",
			Events: []string{model.NotifyEventFailed},
		}}, model.NotifyEventFailed)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestEmitNotesKeepsSynchronousCompatibilityWithoutInboxMetadata(t *testing.T) {
	writer := &notificationOperationWriter{err: context.DeadlineExceeded}
	scheduler := &Scheduler{operations: writer}

	err := scheduler.emitNotes(context.Background(), &model.ProductWorkflow{Name: "release"},
		&model.ProductWorkflowRun{ID: kernel.NewProductWorkflowRunID()},
		[]model.WorkflowNotification{{
			ID: "mail-a", Channel: model.NotifyChannelMail, Targets: "a@example.com",
			Events: []string{model.NotifyEventFailed},
		}}, model.NotifyEventFailed)

	require.NoError(t, err)
	require.Empty(t, writer.requests)
}

func TestEmitNotesPersistsAllNotificationTypesAndDeduplicates(t *testing.T) {
	db := openNotificationOperationDB(t)
	writer := messaging.NewOperationWriter(messagingpersistence.NewOperationStore(db), nil)
	scheduler := &Scheduler{operations: writer}
	run := &model.ProductWorkflowRun{ID: kernel.NewProductWorkflowRunID(), Status: model.RunStatusSuccess}
	notes := []model.WorkflowNotification{
		{ID: "hook-a", Channel: model.NotifyChannelWebhook, WebhookURL: "https://example.invalid/a", Events: []string{model.NotifyEventPassed}},
		{ID: "mail-a", Channel: model.NotifyChannelMail, Targets: "a@example.com", Events: []string{model.NotifyEventPassed}},
		{ID: "feishu-a", Channel: model.NotifyChannelFeishuUser, Targets: "ou_1", Events: []string{model.NotifyEventPassed}},
	}
	err := postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		ctx = events.WithMetadata(ctx, events.Metadata{EventID: "event-1", Subscriber: "product-workflow.build-completed"})
		if err := scheduler.emitNotes(ctx, &model.ProductWorkflow{Name: "release"}, run, notes, model.NotifyEventPassed); err != nil {
			return err
		}
		return scheduler.emitNotes(ctx, &model.ProductWorkflow{Name: "release"}, run, notes, model.NotifyEventPassed)
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Table("platform.operation_outbox").Count(&count).Error)
	require.Equal(t, int64(3), count)
}

func openNotificationOperationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE platform.operation_outbox (
			id TEXT PRIMARY KEY, event_id TEXT NOT NULL, subscriber TEXT NOT NULL,
			operation_type TEXT NOT NULL, idempotency_key TEXT NOT NULL UNIQUE, payload BLOB NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt_at DATETIME NOT NULL, locked_at DATETIME, locked_by TEXT NOT NULL DEFAULT '',
			completed_at DATETIME, last_error TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL
		)
	`).Error)
	return db
}
