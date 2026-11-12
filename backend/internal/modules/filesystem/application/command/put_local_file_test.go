package command

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type failingBus struct{}

func (failingBus) Publish(context.Context, ...events.Event) error {
	return errors.New("publish failed")
}

func (failingBus) Subscribe(string, string, messaging.Handler) error {
	return nil
}

func TestPutLocalFileRollsBackWhenEventPublishFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	repo := persistence.NewRepository(db)
	ns := &model.Namespace{ID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}
	require.NoError(t, repo.CreateNamespace(context.Background(), ns))
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	defer provider.Close()

	h := NewPutLocalFileHandler(repo, provider, postgres.NewTransactor(db), failingBus{}, nil)
	cmd := PutLocalFile{
		CommandID: uuid.New(), ActorID: uuid.New(), OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, Name: "fixture.txt",
		MIMEType: "text/plain", Size: int64(len("fixture-content")),
		Body: bytes.NewBufferString("fixture-content"), Authorization: Authorization{Allowed: true},
		ReplicaProviderID: uuid.New(),
	}

	_, err = h.Handle(context.Background(), cmd)
	require.Error(t, err)
	_, err = repo.GetNode(context.Background(), ns.ID, uuid.Nil)
	require.ErrorIs(t, err, domain.ErrNodeNotFound)

	var nodes, revisions, blobs int64
	require.NoError(t, db.Table("filesystem.nodes").Where("namespace_id = ?", ns.ID).Count(&nodes).Error)
	require.NoError(t, db.Table("filesystem.file_revisions").Count(&revisions).Error)
	require.NoError(t, db.Table("filesystem.blobs").Count(&blobs).Error)
	require.Zero(t, nodes)
	require.Zero(t, revisions)
	require.Zero(t, blobs)
}
