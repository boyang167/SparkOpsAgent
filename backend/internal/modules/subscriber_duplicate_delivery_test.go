package modules_test

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	governancecommand "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	governanceservice "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/service"
	governancesubscriber "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/subscriber"
	governancepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	productsubscriber "github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/subscriber"
	productpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/persistence"
	workspacesubscriber "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/subscriber"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type subscriberTestTransport struct {
	retries int
}

type failingSubscriberOperationWriter struct{}

func (failingSubscriberOperationWriter) Schedule(context.Context, messaging.OperationRequest) error {
	return errors.New("operation outbox unavailable")
}

func (*subscriberTestTransport) Declare(context.Context, []messaging.Subscription) error { return nil }
func (*subscriberTestTransport) Publish(context.Context, events.Envelope) error          { return nil }
func (*subscriberTestTransport) Consume(context.Context, messaging.Subscription, messaging.DeliveryHandler) error {
	return nil
}
func (t *subscriberTestTransport) PublishRetry(context.Context, messaging.Subscription, events.Envelope, messaging.RetryDelay, int) error {
	t.retries++
	return nil
}
func (*subscriberTestTransport) PublishDeadLetter(context.Context, messaging.Subscription, events.Envelope, string) error {
	return nil
}
func (*subscriberTestTransport) Close() error { return nil }

func TestGovernanceInboxCommitsDatabaseAndPolicyOperationAtomically(t *testing.T) {
	db := openSubscriberDB(t)
	registry := events.NewRegistry()
	require.NoError(t, integration.RegisterWorkspaceCreated(registry))
	transport := &subscriberTestTransport{}
	consumer := messaging.NewInboxConsumer(
		postgres.NewTransactor(db),
		messagingpersistence.NewInboxStore(db),
		registry,
		transport,
		nil,
		nil,
	)
	store := governancepersistence.NewStore(db)
	operationWriter := messaging.NewOperationWriter(messagingpersistence.NewOperationStore(db), nil)
	handler := governancesubscriber.NewOnWorkspaceCreated(
		governancecommand.NewBootstrapWorkspaceHandler(
			governancepersistence.NewRoleRepository(store),
			governancepersistence.NewBindingRepository(store),
			&governanceservice.APIPolicySync{},
			operationWriter,
		),
		nil,
	)
	event := integration.NewWorkspaceCreated(
		kernel.NewWorkspaceID(),
		kernel.NewOrgID(),
		kernel.NewMemberID(),
	)
	envelope, err := registry.Encode(context.Background(), event)
	require.NoError(t, err)
	var handlerErr error
	subscription := messaging.Subscription{
		EventName:  integration.EventWorkspaceCreated,
		Version:    1,
		Subscriber: "governance.workspace-created.bootstrap",
		Handler: func(ctx context.Context, event events.Event) error {
			handlerErr = handler.Handle(ctx, event)
			return handlerErr
		},
	}

	require.NoError(t, consumer.Handle(context.Background(), subscription, envelope, 0))

	require.NoError(t, handlerErr)
	require.Zero(t, transport.retries)
	assertCount(t, db, "platform.event_inbox", 1)
	assertCount(t, db, "governance.roles", int64(len(authz.DefaultWorkspaceRoles)))
	assertCount(t, db, "governance.role_bindings", 1)
	assertCount(t, db, "operation_outbox", 1)
}

func TestOperationScheduleFailureRollsBackLocalDatabaseAndInbox(t *testing.T) {
	db := openSubscriberDB(t)
	require.NoError(t, db.Exec("CREATE TABLE product.rollback_probe (id TEXT PRIMARY KEY)").Error)
	registry := events.NewRegistry()
	require.NoError(t, integration.RegisterWorkspaceCreated(registry))
	transport := &subscriberTestTransport{}
	consumer := messaging.NewInboxConsumer(
		postgres.NewTransactor(db),
		messagingpersistence.NewInboxStore(db),
		registry,
		transport,
		nil,
		nil,
	)
	event := integration.NewWorkspaceCreated(kernel.NewWorkspaceID(), kernel.NewOrgID(), kernel.NewMemberID())
	envelope, err := registry.Encode(context.Background(), event)
	require.NoError(t, err)
	subscription := messaging.Subscription{
		EventName: event.EventName(), Version: 1, Subscriber: "task10.rollback-proof",
		Handler: func(ctx context.Context, _ events.Event) error {
			if err := postgres.DB(ctx, db).Exec("INSERT INTO product.rollback_probe (id) VALUES (?)", "local-change").Error; err != nil {
				return err
			}
			return failingSubscriberOperationWriter{}.Schedule(ctx, messaging.OperationRequest{
				Owner: "task10.rollback-proof", OperationType: "task10.proof.v1", IdempotencyKey: "event:subscriber:type:target",
			})
		},
	}

	require.NoError(t, consumer.Handle(context.Background(), subscription, envelope, 0))

	require.Equal(t, 1, transport.retries)
	assertCount(t, db, "product.rollback_probe", 0)
	assertCount(t, db, "platform.event_inbox", 0)
}

func TestPureDatabaseSubscribersSkipDuplicateDeliveries(t *testing.T) {
	db := openSubscriberDB(t)
	registry := events.NewRegistry()
	require.NoError(t, deliveryevent.Register(registry))
	require.NoError(t, integration.RegisterWorkspaceCreated(registry))
	transport := &subscriberTestTransport{}
	consumer := messaging.NewInboxConsumer(
		postgres.NewTransactor(db),
		messagingpersistence.NewInboxStore(db),
		registry,
		transport,
		nil,
		nil,
	)

	workspaceID := kernel.NewWorkspaceID()
	ownerID := kernel.NewMemberID()
	projectID := kernel.NewProjectID()
	buildID := kernel.NewBuildID()
	environmentID := kernel.NewEnvironmentID()
	requirementID := kernel.NewProductRequirementID()
	require.NoError(t, db.Exec(`
		INSERT INTO product.product_requirements
			(id, product_id, product_plan_id, project_id, build_id, status, other_info, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'running', '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, requirementID.String(), kernel.NewProductID().String(), kernel.NewProductPlanID().String(),
		projectID.String(), buildID.String()).Error)

	cases := []struct {
		event      events.Event
		subscriber string
		handler    messaging.Handler
	}{
		{
			event:      integration.NewWorkspaceCreated(workspaceID, kernel.NewOrgID(), ownerID),
			subscriber: "workspace-app.workspace-created.seed-default-templates",
			handler:    workspacesubscriber.NewOnWorkspaceCreated(db).Handle,
		},
		{
			event: deliveryevent.NewBuildCompleted(
				buildID, projectID, ownerID, workspaceID, "api", "success", 1,
			),
			subscriber: "product.build-completed",
			handler: productsubscriber.NewOnBuildCompleted(
				productpersistence.NewProductRepository(db),
			).Handle,
		},
		{
			event: deliveryevent.NewDeployCompleted(
				kernel.NewDeploymentRunID(), projectID, workspaceID, &environmentID,
				"api", "succeeded", "normal", "image:v1",
			),
			subscriber: "product.deploy-completed",
			handler: productsubscriber.NewOnDeployCompleted(
				productpersistence.NewEnvironmentRepository(db),
			).Handle,
		},
	}

	for _, tt := range cases {
		envelope, err := registry.Encode(context.Background(), tt.event)
		require.NoError(t, err)
		var handlerErr error
		subscription := messaging.Subscription{
			EventName: tt.event.EventName(), Version: 1,
			Subscriber: tt.subscriber,
			Handler: func(ctx context.Context, event events.Event) error {
				handlerErr = tt.handler(ctx, event)
				return handlerErr
			},
		}
		firstErr := consumer.Handle(context.Background(), subscription, envelope, 0)
		require.NoErrorf(t, handlerErr, "subscriber %s", tt.subscriber)
		require.NoError(t, firstErr)
		require.NoError(t, consumer.Handle(context.Background(), subscription, envelope, 0))
	}

	require.Zero(t, transport.retries)
	assertCount(t, db, "workspace_app.templates", 14)
	assertCount(t, db, "platform.event_inbox", int64(len(cases)))
	assertCount(t, db, "product.environment_projects", 1)
	var status string
	require.NoError(t, db.Table("product.product_requirements").
		Select("status").Where("id = ?", requirementID.String()).Scan(&status).Error)
	require.Equal(t, "completed", status)
}

func openSubscriberDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	for _, schema := range []string{"platform", "governance", "workspace_app", "product"} {
		require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS "+schema).Error)
	}
	require.NoError(t, db.Exec(`
		CREATE TABLE platform.event_inbox (
			event_id TEXT NOT NULL, subscriber TEXT NOT NULL, event_type TEXT NOT NULL,
			processed_at DATETIME NOT NULL, created_at DATETIME NOT NULL,
			PRIMARY KEY(event_id, subscriber)
		);
		CREATE TABLE operation_outbox (
			id TEXT PRIMARY KEY, event_id TEXT NOT NULL, subscriber TEXT NOT NULL,
			operation_type TEXT NOT NULL, idempotency_key TEXT NOT NULL UNIQUE, payload BLOB NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt_at DATETIME NOT NULL, locked_at DATETIME, locked_by TEXT NOT NULL DEFAULT '',
			completed_at DATETIME, last_error TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL
		);
		CREATE TABLE governance.roles (
			id TEXT PRIMARY KEY, workspace_id TEXT, parent_id TEXT, name TEXT, permissions BLOB,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
		CREATE TABLE governance.role_bindings (
			id TEXT PRIMARY KEY, role_id TEXT, member_id TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
		CREATE TABLE workspace_app.templates (
			id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, kind TEXT, language TEXT,
			detail TEXT, rules TEXT, status TEXT, is_public BOOLEAN, created_by TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
		CREATE TABLE product.product_requirements (
			id TEXT PRIMARY KEY, product_id TEXT, product_plan_id TEXT, project_id TEXT,
			project_name TEXT, project_tag TEXT, build_id TEXT, description TEXT,
			status TEXT, multicluster_status TEXT, operator TEXT, other_info TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
		CREATE TABLE product.environment_projects (
			environment_id TEXT, project_id TEXT, enabled BOOLEAN,
			current_image_tag TEXT, current_deployment_run_id TEXT, last_deployed_at DATETIME,
			PRIMARY KEY(environment_id, project_id)
		)
	`).Error)
	return db
}

func assertCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Count(&count).Error)
	require.Equal(t, want, count)
}
