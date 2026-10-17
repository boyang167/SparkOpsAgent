package command_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	workspaceapplication "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	workspacerepository "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	platformcrypto "github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type atomicSecretRepo struct {
	workspacerepository.SecretRepository
	db     *gorm.DB
	secret *model.Secret
}

func (r *atomicSecretRepo) FindByID(context.Context, kernel.WorkspaceID, kernel.SecretID) (*model.Secret, error) {
	copySecret := *r.secret
	return &copySecret, nil
}
func (r *atomicSecretRepo) Update(ctx context.Context, secret *model.Secret) error {
	return postgres.DB(ctx, r.db).Table("workspace_app.secrets").
		Where("id = ?", secret.ID.String()).
		Updates(map[string]any{
			"credential_blob": secret.CredentialBlob,
			"service_type":    secret.ServiceType,
			"display_name":    secret.DisplayName,
		}).Error
}

func TestSecretUpdateAndOutboxAreAtomic(t *testing.T) {
	db := openSecretAtomicityDB(t)
	box, err := platformcrypto.NewSecretBox(make([]byte, 32))
	require.NoError(t, err)
	secretCrypto := workspaceapplication.NewSecretCrypto(box)
	oldBlob, err := secretCrypto.EncryptCredentials(model.SecretCredentials{Password: "old"})
	require.NoError(t, err)
	name, err := model.NewSecretName("jenkins")
	require.NoError(t, err)
	secret := model.NewSecret(
		kernel.NewWorkspaceID(), name, "Jenkins", "", "jenkins", "", "", oldBlob,
		model.SecretStatusActive, false, false, kernel.NewMemberID(),
	)
	require.NoError(t, db.Table("workspace_app.secrets").Create(map[string]any{
		"id": secret.ID.String(), "credential_blob": oldBlob,
		"service_type": secret.ServiceType, "display_name": secret.DisplayName,
	}).Error)
	repo := &atomicSecretRepo{db: db, secret: secret}
	tx := postgres.NewTransactor(db)
	input := command.UpdateSecretInput{
		DisplayName: "Jenkins 2", ServiceType: "jenkins",
		Credentials: model.SecretCredentials{Password: "new"}, ReplaceCredentials: true,
		Status: model.SecretStatusActive,
	}

	handler := command.NewUpdateSecretHandler(
		repo, updateWorkspaceChecker{}, secretCrypto,
		messaging.NewReliableBus(
			sharedevents.NewRegistry(),
			messagingpersistence.NewOutboxStore(db),
			nil, nil, nil,
		),
		tx,
	)
	_, err = handler.Handle(context.Background(), secret.WorkspaceID, kernel.NewMemberID(), secret.ID, input)
	require.Error(t, err)
	require.Equal(t, oldBlob, readSecretBlob(t, db, secret.ID))
	requireSecretAtomicityCount(t, db, "platform.event_outbox", 0)

	registry := sharedevents.NewRegistry()
	require.NoError(t, integration.RegisterSecretUpdated(registry))
	handler = command.NewUpdateSecretHandler(
		repo, updateWorkspaceChecker{}, secretCrypto,
		messaging.NewReliableBus(
			registry,
			messagingpersistence.NewOutboxStore(db),
			nil, nil, nil,
		),
		tx,
	)
	_, err = handler.Handle(context.Background(), secret.WorkspaceID, kernel.NewMemberID(), secret.ID, input)
	require.NoError(t, err)
	require.NotEqual(t, oldBlob, readSecretBlob(t, db, secret.ID))
	requireSecretAtomicityCount(t, db, "platform.event_outbox", 1)
}

func openSecretAtomicityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS workspace_app").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE workspace_app.secrets (
			id TEXT PRIMARY KEY, credential_blob TEXT, service_type TEXT, display_name TEXT
		)
	`).Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	return db
}

func readSecretBlob(t *testing.T, db *gorm.DB, id kernel.SecretID) string {
	t.Helper()
	var row struct{ CredentialBlob string }
	require.NoError(t, db.Table("workspace_app.secrets").Where("id = ?", id.String()).Take(&row).Error)
	return row.CredentialBlob
}

func requireSecretAtomicityCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Count(&count).Error)
	require.Equal(t, want, count)
}
