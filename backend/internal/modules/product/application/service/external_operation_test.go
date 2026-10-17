package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	productoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type recordingOperationWriter struct {
	requests []messaging.OperationRequest
	err      error
}

func (w *recordingOperationWriter) Schedule(_ context.Context, request messaging.OperationRequest) error {
	w.requests = append(w.requests, request)
	return w.err
}

func TestScheduleJiraTransitionUsesStableEventKey(t *testing.T) {
	writer := &recordingOperationWriter{}
	ctx := events.WithMetadata(context.Background(), events.Metadata{
		EventID: "event-1", Subscriber: "product.workflow-run-completed",
	})
	payload := productoutbound.JiraTransitionPayload{
		WorkspaceID: "workspace-1", PMWorkspaceID: "pm-1", Provider: "jira", IssueKey: "PROJ-1", Status: "Done",
	}

	if err := scheduleJiraTransition(ctx, writer, payload); err != nil {
		t.Fatalf("schedule transition: %v", err)
	}
	if len(writer.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(writer.requests))
	}
	request := writer.requests[0]
	wantKey := "event-1:product.workflow-run-completed:product.jira.transition.v1:jira-PROJ-1"
	if request.IdempotencyKey != wantKey {
		t.Fatalf("key = %q, want %q", request.IdempotencyKey, wantKey)
	}
}

func TestScheduleReleasePlanWebhookPropagatesWriterFailure(t *testing.T) {
	writer := &recordingOperationWriter{err: errors.New("outbox unavailable")}
	ctx := events.WithMetadata(context.Background(), events.Metadata{
		EventID: "event-1", Subscriber: "product.workflow-run-completed",
	})
	payload := productoutbound.ReleasePlanWebhookPayload{PlanID: "plan-1", Event: "all_done"}

	if err := scheduleReleasePlanWebhook(ctx, writer, payload); err == nil {
		t.Fatal("expected schedule error")
	}
}

func TestScheduleReleasePlanApprovalUsesIndependentTypeAndStableKey(t *testing.T) {
	writer := &recordingOperationWriter{}
	ctx := events.WithMetadata(context.Background(), events.Metadata{
		EventID: "event-approval", Subscriber: "product.release-plan",
	})

	err := scheduleReleasePlanApproval(ctx, writer, productoutbound.ReleasePlanApprovalPayload{
		PlanID: "plan-1",
	})

	require.NoError(t, err)
	require.Len(t, writer.requests, 1)
	require.Equal(t, productoutbound.OperationTypeReleasePlanApprovalSend, writer.requests[0].OperationType)
	require.Equal(t,
		"event-approval:product.release-plan:product.release-plan.approval.send.v1:plan-1",
		writer.requests[0].IdempotencyKey,
	)
}

func TestProductExternalSchedulesPersistRealRowsAndDeduplicate(t *testing.T) {
	db := openExternalOperationDB(t)
	writer := messaging.NewOperationWriter(messagingpersistence.NewOperationStore(db), nil)
	tx := postgres.NewTransactor(db)

	err := tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		ctx = events.WithMetadata(ctx, events.Metadata{EventID: "event-1", Subscriber: "product.workflow-run-completed"})
		jira := productoutbound.JiraTransitionPayload{Provider: "jira", IssueKey: "PROJ-1", Status: "Done"}
		if err := scheduleJiraTransition(ctx, writer, jira); err != nil {
			return err
		}
		if err := scheduleJiraTransition(ctx, writer, jira); err != nil {
			return err
		}
		if err := scheduleReleasePlanWebhook(ctx, writer, productoutbound.ReleasePlanWebhookPayload{
			PlanID: "plan-1", Event: "all_done", TokenDigest: ReleasePlanHookTokenDigest("capability-token"),
		}); err != nil {
			return err
		}
		return scheduleReleasePlanApproval(ctx, writer, productoutbound.ReleasePlanApprovalPayload{
			PlanID: "plan-1",
		})
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Table("operation_outbox").Count(&count).Error)
	require.Equal(t, int64(3), count)
	var types []string
	require.NoError(t, db.Table("operation_outbox").Order("operation_type").Pluck("operation_type", &types).Error)
	require.Equal(t, []string{
		productoutbound.OperationTypeJiraTransition,
		productoutbound.OperationTypeReleasePlanApprovalSend,
		productoutbound.OperationTypeReleasePlanWebhookSend,
	}, types)
	var hookPayload string
	require.NoError(t, db.Table("operation_outbox").Select("payload").
		Where("operation_type = ?", productoutbound.OperationTypeReleasePlanWebhookSend).
		Scan(&hookPayload).Error)
	require.NotContains(t, hookPayload, "capability-token")
	require.NotContains(t, hookPayload, "https://callback.example")
	require.True(t, strings.Contains(hookPayload, ReleasePlanHookTokenDigest("capability-token")))
}

func openExternalOperationDB(t *testing.T) *gorm.DB {
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
