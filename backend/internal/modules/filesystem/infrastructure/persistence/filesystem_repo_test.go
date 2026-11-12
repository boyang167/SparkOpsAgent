package persistence

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

func TestCreateChildRejectsDuplicateNormalizedName(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	parent := seedDirectory(t, db, ns.ID)

	require.NoError(t, repo.CreateNode(context.Background(), mustFile(ns.ID, parent.ID, "Report.csv")))
	err := repo.CreateNode(context.Background(), mustFile(ns.ID, parent.ID, "report.CSV"))

	require.ErrorIs(t, err, domain.ErrNodeNameConflict)
}

func TestCreateRootRejectsDuplicateNormalizedName(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)

	require.NoError(t, repo.CreateNode(context.Background(), rootFile(ns.ID, "Report.csv")))
	err := repo.CreateNode(context.Background(), rootFile(ns.ID, "report.CSV"))

	require.ErrorIs(t, err, domain.ErrNodeNameConflict)
}

func TestGetNodeConvertsNotFound(t *testing.T) {
	repo, _ := newTestRepository(t)

	_, err := repo.GetNode(context.Background(), uuid.New(), uuid.New())

	require.ErrorIs(t, err, domain.ErrNodeNotFound)
}

func TestSaveRevisionAndSetCurrentRevisionRoundTrip(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	node := rootFile(ns.ID, "Report.csv")
	require.NoError(t, repo.CreateNode(context.Background(), node))
	blob := &model.Blob{
		ID: uuid.New(), DedupDomain: uuid.New(), SHA256: "hash", Size: 12,
		State: model.BlobStateAvailable,
	}
	revision, err := model.NewRevision(node.ID, 1, blob.ID, 12, "text/plain", uuid.New())
	require.NoError(t, err)
	replica := &model.StorageReplica{
		ID: uuid.New(), BlobID: blob.ID, ProviderID: uuid.New(),
		ObjectKey: "objects/report", State: model.ReplicaStateAvailable,
	}

	require.NoError(t, repo.SaveRevision(context.Background(), revision, blob, replica))
	require.NoError(t, repo.SetCurrentRevision(context.Background(), node.ID, revision.ID()))

	got, err := repo.GetNode(context.Background(), ns.ID, node.ID)
	require.NoError(t, err)
	require.Equal(t, revision.ID(), *got.CurrentRevisionID)
	require.Equal(t, revision.ID(), revision.ID())
	require.Equal(t, node.ID, revision.NodeID())
}

func newTestRepository(t *testing.T) (repository.FilesystemRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(0)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, AutoMigrate(db))
	return NewRepository(db), db
}

func seedNamespace(t *testing.T, db *gorm.DB) *model.Namespace {
	t.Helper()
	ns := &model.Namespace{ID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New()}
	require.NoError(t, db.Create(&namespaceRow{ID: ns.ID, OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID, Status: "ACTIVE"}).Error)
	return ns
}

func seedDirectory(t *testing.T, db *gorm.DB, namespaceID uuid.UUID) *model.Node {
	t.Helper()
	node := rootDirectory(namespaceID, "parent")
	require.NoError(t, db.Create(&nodeRow{ID: node.ID, NamespaceID: node.NamespaceID, ParentID: node.ParentID, Type: node.Type, Name: node.Name, NormalizedName: node.NormalizedName, State: node.State, OwnerID: node.OwnerID}).Error)
	return node
}

func mustFile(namespaceID, parentID uuid.UUID, name string) *model.Node {
	node, err := model.NewFile(namespaceID, uuid.New(), name, parentID)
	if err != nil {
		panic(err)
	}
	return node
}

func rootFile(namespaceID uuid.UUID, name string) *model.Node {
	node := mustFile(namespaceID, uuid.Nil, name)
	node.ParentID = nil
	return node
}

func rootDirectory(namespaceID uuid.UUID, name string) *model.Node {
	node, err := model.NewDirectory(namespaceID, uuid.New(), name, uuid.Nil)
	if err != nil {
		panic(err)
	}
	node.ParentID = nil
	return node
}
