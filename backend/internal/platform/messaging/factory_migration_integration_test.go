//go:build integration

package messaging

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/dbmigration"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRealMigrationRunnerControlsApplicationFactoryGate(t *testing.T) {
	for _, oldVersion := range []uint{69, 74} {
		t.Run(fmt.Sprintf("from-%d", oldVersion), func(t *testing.T) {
			db, dsn := openFactoryMigrationPostgres(t, requireIntegrationEnv(t, "DATABASE_DSN"))
			prepareFactoryMigrationBaseline(t, db)
			require.NoError(t, dbmigration.RunFrom(dsn, "../../../migrations", 70, 69))
			if oldVersion > 69 {
				require.NoError(t, dbmigration.RunFrom(dsn, "../../../migrations", 70, oldVersion))
			}
			_, err := newMigrationGateFactory(db)
			require.ErrorContains(t, err, "required version 75")

			require.NoError(t, dbmigration.RunFrom(dsn, "../../../migrations", 70, 0))
			factory, err := newMigrationGateFactory(db)
			require.NoError(t, err)
			require.NoError(t, factory.Close())
		})
	}
}

func prepareFactoryMigrationBaseline(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`
		CREATE SCHEMA product;
		CREATE SCHEMA workspace_app;
		CREATE SCHEMA delivery;
		CREATE TABLE product.product_workflow_runs (id UUID PRIMARY KEY);
		CREATE TABLE workspace_app.projects (
			id UUID PRIMARY KEY,
			status VARCHAR(32) NOT NULL
		);
		CREATE TABLE delivery.deployment_runs (id UUID PRIMARY KEY)
	`).Error)
}

func newMigrationGateFactory(db *gorm.DB) (*Factory, error) {
	return newFromConfig(
		&config.Config{Messaging: config.MessagingConfig{
			Driver: "amqp",
			AMQP:   config.AMQPConfig{URL: "amqp://unused", EventExchange: "events"},
		}},
		db,
		events.NewRegistry(),
		postgres.NewTransactor(db),
		zap.NewNop(),
		factoryDependencies{
			newQueue: func(AMQPConfig) (Queue, error) {
				return &recordingQueue{close: func() error { return nil }}, nil
			},
			newMetrics: func(prometheus.Registerer) (*Metrics, error) {
				return NewMetrics(prometheus.NewRegistry())
			},
			newEventTransport: func(string, *Metrics, ...string) (EventTransport, error) {
				return &fakeEventTransport{}, nil
			},
			runtimeIdentity: func() (string, string) { return "integration", "migration-gate" },
		},
	)
}

func openFactoryMigrationPostgres(t *testing.T, adminDSN string) (*gorm.DB, string) {
	t.Helper()
	admin, err := gorm.Open(gormpostgres.Open(adminDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	databaseName := "sparkops_factory_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE DATABASE "+databaseName).Error)
	dsn := factoryMigrationDSN(t, adminDSN, databaseName)
	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		require.NoError(t, dbErr)
		require.NoError(t, sqlDB.Close())
		require.NoError(t, admin.Exec("DROP DATABASE "+databaseName).Error)
		adminSQL, adminErr := admin.DB()
		require.NoError(t, adminErr)
		require.NoError(t, adminSQL.Close())
	})
	return db, dsn
}

func factoryMigrationDSN(t *testing.T, dsn, databaseName string) string {
	t.Helper()
	if strings.Contains(dsn, "://") {
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		parsed.Path = "/" + databaseName
		return parsed.String()
	}
	databaseSetting := regexp.MustCompile(`(?i)\b(?:dbname|database)\s*=\s*(?:'[^']*'|\S+)`)
	if databaseSetting.MatchString(dsn) {
		return databaseSetting.ReplaceAllString(dsn, "dbname="+databaseName)
	}
	return dsn + " dbname=" + databaseName
}
