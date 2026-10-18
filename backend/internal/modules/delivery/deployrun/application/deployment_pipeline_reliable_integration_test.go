//go:build integration

package application_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/subscriber"
	deploymodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/consumer"
	deploypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/persistence"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagepersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type terminalDeploymentStepper struct {
	repo  *deploypersistence.DeploymentRunRepository
	calls atomic.Int32
}

func (s *terminalDeploymentStepper) Step(
	ctx context.Context,
	runID kernel.DeploymentRunID,
	_ kernel.WorkspaceID,
) error {
	s.calls.Add(1)
	run, err := s.repo.FindByID(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status == deploymodel.StatusSucceeded {
		return nil
	}
	if err := run.Transition(deploymodel.StatusSucceeded, time.Now().UTC()); err != nil {
		return err
	}
	return s.repo.Update(ctx, run)
}

func TestReliableBuildCompletedPipelineReachesOneTerminalState(t *testing.T) {
	dsn, amqpURL := os.Getenv("DATABASE_DSN"), os.Getenv("AMQP_URL")
	if dsn == "" || amqpURL == "" {
		t.Skip("DATABASE_DSN and AMQP_URL are required for reliable deployment integration")
	}
	db := openReliablePipelinePostgres(t, dsn)
	require.NoError(t, db.Exec(`
		CREATE SCHEMA delivery;
		CREATE TABLE delivery.deployment_runs (
			id UUID PRIMARY KEY,
			workspace_id UUID NOT NULL,
			project_id UUID NOT NULL,
			cluster_id UUID NOT NULL,
			git_ref_type VARCHAR(16) NOT NULL,
			git_ref_value VARCHAR(255) NOT NULL,
			image_tag VARCHAR(255) NOT NULL,
			image_repository VARCHAR(512) NOT NULL DEFAULT '',
			cross_cluster BOOLEAN NOT NULL DEFAULT FALSE,
			sync_manifests JSONB NOT NULL DEFAULT '[]',
			status VARCHAR(16) NOT NULL,
			build_id UUID NULL,
			error_message TEXT NOT NULL DEFAULT '',
			created_by UUID NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			started_at TIMESTAMPTZ NULL,
			finished_at TIMESTAMPTZ NULL,
			current_stage VARCHAR(16) NOT NULL DEFAULT '',
			run_type VARCHAR(16) NOT NULL DEFAULT 'normal',
			rollback_of_run_id UUID NULL,
			version BIGINT NOT NULL DEFAULT 0,
			attempt_count INT NOT NULL DEFAULT 0,
			lease_owner VARCHAR(160) NULL,
			lease_expires_at TIMESTAMPTZ NULL,
			environment_id UUID NULL,
			project_version_id UUID NULL,
			deploy_namespace VARCHAR(255) NOT NULL DEFAULT '',
			undeploy_options JSONB NULL,
			archive_generation UUID NULL
		);
		CREATE TABLE delivery.deployment_run_events (
			id BIGSERIAL PRIMARY KEY,
			run_id UUID NOT NULL,
			at TIMESTAMPTZ NOT NULL,
			status VARCHAR(16) NOT NULL,
			stage VARCHAR(16) NOT NULL DEFAULT '',
			message TEXT NOT NULL
		);
		CREATE TABLE integration_notifications (
			event_id UUID PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL
		)
	`).Error)

	connection, err := amqp.Dial(amqpURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	channel, err := connection.Channel()
	require.NoError(t, err)
	t.Cleanup(func() { _ = channel.Close() })
	for _, subscriberName := range []string{
		"delivery.build-completed.deployrun",
		"notification.build-completed",
	} {
		deleteReliableEventQueues(channel, subscriberName)
	}
	t.Cleanup(func() {
		cleanup, cleanupErr := connection.Channel()
		if cleanupErr != nil {
			return
		}
		defer cleanup.Close()
		for _, subscriberName := range []string{
			"delivery.build-completed.deployrun",
			"notification.build-completed",
		} {
			deleteReliableEventQueues(cleanup, subscriberName)
		}
	})
	_, _ = channel.QueueDelete(messaging.TopicDeploy, false, false, false)
	_, _ = channel.QueueDelete(messaging.DeadLetterTopic(messaging.TopicDeploy), false, false, false)
	taskExchange := "sparkops-integration-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	taskQueue, err := messaging.NewAMQPQueue(messaging.AMQPConfig{
		URL: amqpURL, Exchange: taskExchange, ExchangeType: "direct",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = taskQueue.Close()
		cleanup, cleanupErr := connection.Channel()
		if cleanupErr == nil {
			_, _ = cleanup.QueueDelete(messaging.TopicDeploy, false, false, false)
			_, _ = cleanup.QueueDelete(messaging.DeadLetterTopic(messaging.TopicDeploy), false, false, false)
			_ = cleanup.ExchangeDelete(taskExchange, false, false)
			_ = cleanup.Close()
		}
	})

	repo := deploypersistence.NewDeploymentRunRepository(db)
	stepper := &terminalDeploymentStepper{repo: repo}
	deployConsumer, err := consumer.New(repo, stepper)
	require.NoError(t, err)
	runCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	require.NoError(t, deployConsumer.Register(runCtx, taskQueue))

	transactor := postgres.NewTransactor(db)
	eventRegistry := events.NewRegistry()
	require.NoError(t, deliveryevent.Register(eventRegistry))
	metrics, err := messaging.NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	eventTransport, err := messaging.NewAMQPEventTransport(amqpURL, metrics)
	require.NoError(t, err)
	t.Cleanup(func() { _ = eventTransport.Close() })
	outbox := messagepersistence.NewOutboxStore(db)
	inbox := messagepersistence.NewInboxStore(db)
	operationStore := messagepersistence.NewOperationStore(db)
	operationWriter := messaging.NewOperationWriter(operationStore, nil)
	eventDispatcher := messaging.NewOutboxDispatcher(
		outbox,
		eventTransport,
		messaging.OutboxDispatcherConfig{
			Owner: "pipeline-event", BatchSize: 10, Lease: time.Second, PollInterval: 10 * time.Millisecond,
		},
		nil,
		metrics,
	)
	inboxConsumer := messaging.NewInboxConsumer(transactor, inbox, eventRegistry, eventTransport, nil, metrics)
	reliableBus := messaging.NewReliableBus(eventRegistry, outbox, eventTransport, eventDispatcher, inboxConsumer)
	deploySubscriber := subscriber.NewOnBuildCompleted(repo, consumer.NewPublisher(taskQueue), operationWriter)
	require.NoError(t, reliableBus.Subscribe(
		deliveryevent.BuildCompletedName,
		"delivery.build-completed.deployrun",
		deploySubscriber.Handle,
	))
	require.NoError(t, reliableBus.Subscribe(
		deliveryevent.BuildCompletedName,
		"notification.build-completed",
		func(ctx context.Context, _ events.Event) error {
			metadata, ok := events.MetadataFromContext(ctx)
			if !ok {
				return fmt.Errorf("notification metadata missing")
			}
			return postgres.DB(ctx, db).Exec(`
				INSERT INTO integration_notifications(event_id, created_at)
				VALUES (?, ?)
				ON CONFLICT DO NOTHING
			`, metadata.EventID, time.Now().UTC()).Error
		},
	))
	require.NoError(t, reliableBus.Start(runCtx))
	t.Cleanup(func() { _ = reliableBus.Close() })

	operationRegistry := messaging.NewOperationRegistry()
	require.NoError(t, operationRegistry.Register(
		outbound.OperationTypeDeployQueuePublish,
		outbound.NewDeployQueueHandler(repo, taskQueue),
	))
	operationDispatcher := messaging.NewOperationDispatcher(
		operationStore,
		operationRegistry,
		messaging.OperationDispatcherConfig{
			Owner: "pipeline-operation", BatchSize: 1, Lease: time.Second,
			PollInterval: 10 * time.Millisecond, Workers: 1,
		},
		nil,
		metrics,
	)
	require.NoError(t, operationDispatcher.Start(runCtx))
	t.Cleanup(func() { _ = operationDispatcher.Close() })

	wsID, projectID := kernel.NewWorkspaceID(), kernel.NewProjectID()
	memberID, clusterID, buildID := kernel.NewMemberID(), kernel.NewClusterInfoID(), kernel.NewBuildID()
	run := deploymodel.NewDeploymentRun(wsID, projectID, clusterID, memberID, "branch", "main", "main")
	require.NoError(t, run.Transition(deploymodel.StatusPreparing, time.Now().UTC()))
	require.NoError(t, run.Transition(deploymodel.StatusBuilding, time.Now().UTC()))
	run.BuildID = &buildID
	require.NoError(t, repo.Create(context.Background(), run))
	completed := deliveryevent.NewBuildCompleted(
		buildID,
		projectID,
		memberID,
		wsID,
		"orders",
		string(buildmodel.BuildStatusSuccess),
		1,
	)
	envelope, err := eventRegistry.Encode(context.Background(), completed)
	require.NoError(t, err)
	require.NoError(t, transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return outbox.Insert(ctx, envelope)
	}))

	require.Eventually(t, func() bool {
		updated, findErr := repo.FindByID(context.Background(), run.ID)
		return findErr == nil && updated.Status == deploymodel.StatusSucceeded
	}, 15*time.Second, 25*time.Millisecond)
	require.Equal(t, int32(1), stepper.calls.Load())
	require.Equal(t, int64(1), reliablePipelineCount(t, db, "platform.event_inbox", "subscriber = 'delivery.build-completed.deployrun'"))
	require.Equal(t, int64(1), reliablePipelineCount(t, db, "platform.operation_outbox", "operation_type = 'delivery.queue.deploy.publish.v1'"))
	require.Equal(t, int64(1), reliablePipelineCount(t, db, "integration_notifications", "TRUE"))
	eventCount := reliablePipelineCount(t, db, "delivery.deployment_run_events", "TRUE")

	require.NoError(t, eventTransport.Publish(context.Background(), envelope))
	time.Sleep(300 * time.Millisecond)
	require.Equal(t, int32(1), stepper.calls.Load())
	require.Equal(t, eventCount, reliablePipelineCount(t, db, "delivery.deployment_run_events", "TRUE"))
	require.Equal(t, int64(1), reliablePipelineCount(t, db, "integration_notifications", "TRUE"))
	require.Equal(t, int64(1), reliablePipelineCount(t, db, "platform.operation_outbox", "operation_type = 'delivery.queue.deploy.publish.v1'"))
}

func openReliablePipelinePostgres(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	admin, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	databaseName := "sparkops_pipeline_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE DATABASE "+databaseName).Error)
	db, err := gorm.Open(gormpostgres.Open(reliablePipelineDSN(t, dsn, databaseName)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	migration, err := os.ReadFile("../../../../../migrations/000070_transactional_events.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration)).Error)
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
		_ = admin.Exec(fmt.Sprintf(
			"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s'",
			databaseName,
		)).Error
		_ = admin.Exec("DROP DATABASE IF EXISTS " + databaseName).Error
		adminSQL, adminErr := admin.DB()
		if adminErr == nil {
			_ = adminSQL.Close()
		}
	})
	return db
}

func reliablePipelineDSN(t *testing.T, dsn, databaseName string) string {
	t.Helper()
	if strings.Contains(dsn, "://") {
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		parsed.Path = "/" + databaseName
		return parsed.String()
	}
	setting := regexp.MustCompile(`(?i)\b(?:dbname|database)\s*=\s*(?:'[^']*'|\S+)`)
	if setting.MatchString(dsn) {
		return setting.ReplaceAllString(dsn, "dbname="+databaseName)
	}
	return dsn + " dbname=" + databaseName
}

func reliablePipelineCount(t *testing.T, db *gorm.DB, table, where string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Where(where).Count(&count).Error)
	return count
}

func deleteReliableEventQueues(channel *amqp.Channel, subscriber string) {
	main := "sparkops.event." + subscriber + ".delivery-buildcompleted.v1"
	for _, queue := range []string{
		main,
		main + ".retry.5s",
		main + ".retry.30s",
		main + ".retry.5m",
		main + ".retry.30m",
		main + ".dlq",
	} {
		_, _ = channel.QueueDelete(queue, false, false, false)
	}
}
