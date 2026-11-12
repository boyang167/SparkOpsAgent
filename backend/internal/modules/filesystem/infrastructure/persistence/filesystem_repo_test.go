package persistence

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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

func TestCreateChildRejectsFileParent(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	fileParent := rootFile(ns.ID, "file-parent")
	require.NoError(t, repo.CreateNode(context.Background(), fileParent))
	child := mustFile(ns.ID, fileParent.ID, "child")

	err := repo.CreateNode(context.Background(), child)

	require.ErrorIs(t, err, domain.ErrParentNotDirectory)
}

func TestCreateChildRejectsCrossNamespaceParent(t *testing.T) {
	repo, db := newTestRepository(t)
	ns1, ns2 := seedNamespace(t, db), seedNamespace(t, db)
	parent := rootDirectory(ns2.ID, "parent")
	require.NoError(t, repo.CreateNode(context.Background(), parent))

	err := repo.CreateNode(context.Background(), mustFile(ns1.ID, parent.ID, "child"))

	require.ErrorIs(t, err, domain.ErrNamespaceMismatch)
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

func TestSetCurrentRevisionAllowsAtomicSwitch(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	node := rootFile(ns.ID, "report")
	require.NoError(t, repo.CreateNode(context.Background(), node))
	rev1, blob1, replica1 := revisionFixture(t, node.ID, 1)
	rev2, blob2, replica2 := revisionFixture(t, node.ID, 2)
	require.NoError(t, repo.SaveRevision(context.Background(), rev1, blob1, replica1))
	require.NoError(t, repo.SaveRevision(context.Background(), rev2, blob2, replica2))
	require.NoError(t, repo.SetCurrentRevision(context.Background(), node.ID, rev1.ID()))
	require.NoError(t, repo.SetCurrentRevision(context.Background(), node.ID, rev2.ID()))

	got, err := repo.GetNode(context.Background(), ns.ID, node.ID)
	require.NoError(t, err)
	require.Equal(t, rev2.ID(), *got.CurrentRevisionID)
}

func TestSetCurrentRevisionRejectsRevisionFromAnotherNode(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	node1, node2 := rootFile(ns.ID, "one"), rootFile(ns.ID, "two")
	require.NoError(t, repo.CreateNode(context.Background(), node1))
	require.NoError(t, repo.CreateNode(context.Background(), node2))
	revision, blob, replica := revisionFixture(t, node2.ID, 1)
	require.NoError(t, repo.SaveRevision(context.Background(), revision, blob, replica))

	err := repo.SetCurrentRevision(context.Background(), node1.ID, revision.ID())

	require.ErrorIs(t, err, domain.ErrRevisionNodeMismatch)
}

func TestMoveNodeRejectsInvalidParents(t *testing.T) {
	repo, db := newTestRepository(t)
	ns1, ns2 := seedNamespace(t, db), seedNamespace(t, db)
	node := rootFile(ns1.ID, "node")
	fileParent := rootFile(ns1.ID, "file-parent")
	foreignParent := rootDirectory(ns2.ID, "foreign-parent")
	require.NoError(t, repo.CreateNode(context.Background(), node))
	require.NoError(t, repo.CreateNode(context.Background(), fileParent))
	require.NoError(t, repo.CreateNode(context.Background(), foreignParent))

	require.ErrorIs(t, repo.MoveNode(context.Background(), node.ID, &fileParent.ID, "renamed"), domain.ErrParentNotDirectory)
	require.ErrorIs(t, repo.MoveNode(context.Background(), node.ID, &foreignParent.ID, "renamed"), domain.ErrNamespaceMismatch)
	require.NoError(t, repo.MoveNode(context.Background(), node.ID, nil, "  Renamed  "))

	got, err := repo.GetNode(context.Background(), ns1.ID, node.ID)
	require.NoError(t, err)
	require.Equal(t, "Renamed", got.Name)
	require.Equal(t, "renamed", got.NormalizedName)
}

func TestSaveRevisionRejectsBlobIDContentConflictAndRollsBack(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	node := rootFile(ns.ID, "report")
	require.NoError(t, repo.CreateNode(context.Background(), node))
	revision1, blob, replica := revisionFixture(t, node.ID, 1)
	require.NoError(t, repo.SaveRevision(context.Background(), revision1, blob, replica))
	revision2, conflictingBlob, replica2 := revisionFixtureWithBlob(t, node.ID, 2, blob.ID)
	conflictingBlob.SHA256 = "different"

	err := repo.SaveRevision(context.Background(), revision2, conflictingBlob, replica2)

	require.ErrorIs(t, err, domain.ErrBlobConflict)
	var count int64
	require.NoError(t, db.Table("filesystem.file_revisions").Where("id = ?", revision2.ID()).Count(&count).Error)
	require.Zero(t, count)
}

func TestSaveRevisionRejectsMismatchedAssociationsBeforeWriting(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	node := rootFile(ns.ID, "report")
	require.NoError(t, repo.CreateNode(context.Background(), node))
	revision, blob, replica := revisionFixture(t, node.ID, 1)
	revisionBlobID := uuid.New()
	revision, err := model.NewRevision(node.ID, 1, revisionBlobID, blob.Size, "text/plain", uuid.New())
	require.NoError(t, err)

	err = repo.SaveRevision(context.Background(), revision, blob, replica)

	require.ErrorIs(t, err, domain.ErrRevisionBlobMismatch)
	assertNoFilesystemRows(t, db, revision.ID(), blob.ID, replica.ID)
}

func TestSaveRevisionRejectsMismatchedReplicaBeforeWriting(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	node := rootFile(ns.ID, "report")
	require.NoError(t, repo.CreateNode(context.Background(), node))
	revision, blob, replica := revisionFixture(t, node.ID, 1)
	replica.BlobID = uuid.New()

	err := repo.SaveRevision(context.Background(), revision, blob, replica)

	require.ErrorIs(t, err, domain.ErrReplicaBlobMismatch)
	assertNoFilesystemRows(t, db, revision.ID(), blob.ID, replica.ID)
}

func TestSaveRevisionConcurrentIdenticalRequestsAreIdempotent(t *testing.T) {
	seedRepo, db, mainDSN, filesystemDSN := newConcurrentTestRepository(t)
	ns := seedNamespace(t, db)
	node := rootFile(ns.ID, "concurrent")
	require.NoError(t, seedRepo.CreateNode(context.Background(), node))
	revision, blob, replica := revisionFixture(t, node.ID, 1)

	const callers = 4
	ready := make(chan error, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(callers)
	for i := 0; i < callers; i++ {
		go func() {
			defer wg.Done()
			workerDB, openErr := gorm.Open(sqlite.Open(mainDSN), &gorm.Config{
				Logger: logger.Default.LogMode(logger.Silent),
			})
			if openErr != nil {
				ready <- openErr
				errs <- openErr
				return
			}
			workerSQLDB, dbErr := workerDB.DB()
			if dbErr != nil {
				ready <- dbErr
				errs <- dbErr
				return
			}
			defer workerSQLDB.Close()
			if attachErr := workerDB.Exec("ATTACH DATABASE ? AS filesystem", filesystemDSN).Error; attachErr != nil {
				ready <- attachErr
				errs <- attachErr
				return
			}
			if pragmaErr := workerDB.Exec("PRAGMA busy_timeout = 5000").Error; pragmaErr != nil {
				ready <- pragmaErr
				errs <- pragmaErr
				return
			}
			ready <- nil
			<-start
			errs <- NewRepository(workerDB).SaveRevision(context.Background(), revision, blob, replica)
		}()
	}
	for i := 0; i < callers; i++ {
		require.NoError(t, <-ready)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	verifyDB, err := gorm.Open(sqlite.Open(mainDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	verifySQLDB, err := verifyDB.DB()
	require.NoError(t, err)
	defer verifySQLDB.Close()
	require.NoError(t, verifyDB.Exec("ATTACH DATABASE ? AS filesystem", filesystemDSN).Error)
	var count int64
	require.NoError(t, verifyDB.Table("filesystem.blobs").Where("id = ?", blob.ID).Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, verifyDB.Table("filesystem.file_revisions").Where("id = ?", revision.ID()).Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, verifyDB.Table("filesystem.storage_replicas").Where("id = ?", replica.ID).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestListChildrenCursorHasStableExclusiveBoundary(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	for _, name := range []string{"a", "b", "c"} {
		require.NoError(t, repo.CreateNode(context.Background(), rootFile(ns.ID, name)))
	}

	first, cursor, err := repo.ListChildren(context.Background(), ns.ID, nil, 2, "")
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.NotEmpty(t, cursor)
	second, next, err := repo.ListChildren(context.Background(), ns.ID, nil, 2, cursor)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Empty(t, next)
	require.NotEqual(t, first[1].ID, second[0].ID)
}

func TestUniqueConflictMappingIsConstraintSpecific(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"node name unique", &pgconn.PgError{Code: "23505", ConstraintName: "nodes_namespace_id_parent_id_normalized_name_key"}, domain.ErrNodeNameConflict},
		{"node id unique", &pgconn.PgError{Code: "23505", ConstraintName: "nodes_pkey"}, domain.ErrNodeIDConflict},
		{"revision unique", &pgconn.PgError{Code: "23505", ConstraintName: "file_revisions_node_id_revision_no_key"}, domain.ErrRevisionConflict},
		{"blob unique", &pgconn.PgError{Code: "23505", ConstraintName: "blobs_dedup_domain_sha256_size_key"}, domain.ErrBlobConflict},
		{"foreign key with similar name", &pgconn.PgError{Code: "23503", ConstraintName: "nodes_namespace_id_parent_id_normalized_name_key"}, domain.ErrPersistence},
		{"check with similar name", &pgconn.PgError{Code: "23514", ConstraintName: "blobs_dedup_domain_sha256_size_key"}, domain.ErrPersistence},
		{"unrelated unique", &pgconn.PgError{Code: "23505", ConstraintName: "unrelated_constraint"}, domain.ErrPersistence},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, mapUniqueConflict(tt.err), tt.want)
		})
	}
}

func TestPersistenceErrorsDoNotExposeDriverType(t *testing.T) {
	err := wrapPersistence(errors.New("driver-specific failure"))

	require.ErrorIs(t, err, domain.ErrPersistence)
	require.NotErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NotContains(t, err.Error(), "driver-specific failure")
}

func TestCreateNodeRejectsIDConflictWithoutNameConflict(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	first := rootFile(ns.ID, "first")
	require.NoError(t, repo.CreateNode(context.Background(), first))
	second := rootFile(ns.ID, "second")
	second.ID = first.ID

	err := repo.CreateNode(context.Background(), second)

	require.ErrorIs(t, err, domain.ErrNodeIDConflict)
	require.NotErrorIs(t, err, domain.ErrNodeNameConflict)
}

func TestAutoMigratePropagatesDatabaseFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	require.ErrorIs(t, AutoMigrate(db), domain.ErrPersistence)
}

func newTestRepository(t *testing.T) (repository.FilesystemRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
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

func newConcurrentTestRepository(t *testing.T) (repository.FilesystemRepository, *gorm.DB, string, string) {
	t.Helper()
	mainDSN := "file:" + t.Name() + "?mode=memory&cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	filesystemDSN := filepath.Join(t.TempDir(), "filesystem.db")
	db, err := gorm.Open(sqlite.Open(mainDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.Exec("ATTACH DATABASE ? AS filesystem", filesystemDSN).Error)
	require.NoError(t, AutoMigrate(db))
	return NewRepository(db), db, mainDSN, filesystemDSN
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

func revisionFixture(t *testing.T, nodeID uuid.UUID, revisionNo int) (*model.FileRevision, *model.Blob, *model.StorageReplica) {
	return revisionFixtureWithBlob(t, nodeID, revisionNo, uuid.New())
}

func revisionFixtureWithBlob(t *testing.T, nodeID uuid.UUID, revisionNo int, blobID uuid.UUID) (*model.FileRevision, *model.Blob, *model.StorageReplica) {
	t.Helper()
	blob := &model.Blob{ID: blobID, DedupDomain: uuid.New(), SHA256: uuid.NewString(), Size: int64(revisionNo), State: model.BlobStateAvailable}
	revision, err := model.NewRevision(nodeID, revisionNo, blob.ID, blob.Size, "text/plain", uuid.New())
	require.NoError(t, err)
	replica := &model.StorageReplica{ID: uuid.New(), BlobID: blob.ID, ProviderID: uuid.New(), ObjectKey: uuid.NewString(), State: model.ReplicaStateAvailable}
	return revision, blob, replica
}

func assertNoFilesystemRows(t *testing.T, db *gorm.DB, revisionID, blobID, replicaID uuid.UUID) {
	t.Helper()
	var count int64
	require.NoError(t, db.Table("filesystem.file_revisions").Where("id = ?", revisionID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Table("filesystem.blobs").Where("id = ?", blobID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Table("filesystem.storage_replicas").Where("id = ?", replicaID).Count(&count).Error)
	require.Zero(t, count)
}
