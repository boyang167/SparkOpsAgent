package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func TestShareRequestRepositoryPersistsAndReviewsWithCAS(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, AutoMigrate(db))
	repo := NewRepository(db)

	owner := uuid.New()
	namespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeUser, OrganizationID: uuid.New(),
		OwnerUserID: owner, Status: "ACTIVE",
	}
	require.NoError(t, repo.CreateNamespace(context.Background(), namespace))
	node, err := model.NewRootDirectory(namespace.ID, owner, ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), node))

	request := model.ShareRequest{
		ID: uuid.New(), OrganizationID: namespace.OrganizationID, NamespaceID: namespace.ID, NodeID: node.ID,
		RequestedBy: owner, PrincipalType: model.PrincipalWorkspace, PrincipalIDs: []uuid.UUID{uuid.New(), uuid.New()},
		Actions:           []model.ResourcePermission{model.ResourcePermissionRead, model.ResourcePermissionDownload},
		InheritToChildren: true, Classification: model.TransferClassificationConfidential,
		Status: model.ShareRequestPending, CreatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.CreateShareRequest(context.Background(), request))
	stored, err := repo.GetShareRequest(context.Background(), request.ID)
	require.NoError(t, err)
	require.Equal(t, request.PrincipalIDs, stored.PrincipalIDs)
	require.Equal(t, request.Actions, stored.Actions)

	reviewer, reviewedAt := uuid.New(), time.Now().UTC()
	stored.Status, stored.ReviewedBy, stored.ReviewedAt = model.ShareRequestApproved, &reviewer, &reviewedAt
	require.NoError(t, repo.ReviewShareRequestCAS(context.Background(), stored, model.ShareRequestPending))
	require.ErrorIs(t, repo.ReviewShareRequestCAS(context.Background(), stored, model.ShareRequestPending), domain.ErrShareRequestConflict)
}

func TestCreateResourceGrantsRequiresTransactionAndWritesWholeBatch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, AutoMigrate(db))
	repo := NewRepository(db)
	owner := uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeUser, OrganizationID: uuid.New(), OwnerUserID: owner, Status: "ACTIVE"}
	require.NoError(t, repo.CreateNamespace(context.Background(), namespace))
	node, err := model.NewRootDirectory(namespace.ID, owner, ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), node))
	principalID := uuid.New()
	grants := []model.ResourceGrant{
		{ID: uuid.New(), NodeID: node.ID, OrganizationID: namespace.OrganizationID, GranteeID: principalID, PrincipalType: model.PrincipalUser, PrincipalID: principalID, Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow},
		{ID: uuid.New(), NodeID: node.ID, OrganizationID: namespace.OrganizationID, GranteeID: principalID, PrincipalType: model.PrincipalUser, PrincipalID: principalID, Permission: model.ResourcePermissionDownload, Effect: model.GrantEffectAllow},
	}
	require.ErrorIs(t, repo.CreateResourceGrants(context.Background(), grants), domain.ErrPersistence)
	require.NoError(t, postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		return repo.CreateResourceGrants(ctx, grants)
	}))
	stored, err := repo.ListNodeResourceGrants(context.Background(), node.ID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
}

func TestNodeClassificationUsesSingleAuthoritativeNodeRowAndFailsClosed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, AutoMigrate(db))
	repo := NewRepository(db)
	owner := uuid.New()
	namespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: uuid.New(),
		WorkspaceID: uuid.New(), Status: "ACTIVE",
	}
	require.NoError(t, repo.CreateNamespace(context.Background(), namespace))
	root, err := model.NewRootDirectory(namespace.ID, owner, ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), root))
	require.NoError(t, repo.SetNamespaceRoot(context.Background(), namespace.WorkspaceID, root.ID))
	node, err := model.NewFile(namespace.ID, owner, "restricted.bin", root.ID)
	require.NoError(t, err)
	node.Classification = model.TransferClassificationRestricted
	require.NoError(t, repo.CreateNode(context.Background(), node))

	classification, err := repo.GetNodeClassification(context.Background(), node.ID)
	require.NoError(t, err)
	require.Equal(t, model.TransferClassificationRestricted, classification)

	_, err = repo.GetNodeClassification(context.Background(), uuid.New())
	require.ErrorIs(t, err, domain.ErrNodeNotFound)

	require.NoError(t, db.Exec("PRAGMA ignore_check_constraints = ON").Error)
	require.NoError(t, db.Table("filesystem.nodes").Where("id = ?", node.ID).Update("classification", "").Error)
	_, err = repo.GetNodeClassification(context.Background(), node.ID)
	require.Error(t, err)
}

func TestSubtreeHighestClassificationUsesOneRecursiveQuerySQLite(t *testing.T) {
	testSubtreeHighestClassification(t, openShareSQLite(t))
}

func TestSubtreeHighestClassificationUsesOneRecursiveQueryPostgres(t *testing.T) {
	db := openFilesystemPostgres(t)
	require.NoError(t, AutoMigrate(db))
	testSubtreeHighestClassification(t, db)
}

func testSubtreeHighestClassification(t *testing.T, db *gorm.DB) {
	t.Helper()
	repo := NewRepository(db)
	owner := uuid.New()
	namespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: uuid.New(),
		WorkspaceID: uuid.New(), Status: "ACTIVE",
	}
	require.NoError(t, repo.CreateNamespace(context.Background(), namespace))
	root, err := model.NewRootDirectory(namespace.ID, owner, ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), root))
	confidential, err := model.NewFile(namespace.ID, owner, "confidential.bin", root.ID)
	require.NoError(t, err)
	confidential.Classification = model.TransferClassificationConfidential
	require.NoError(t, repo.CreateNode(context.Background(), confidential))
	restricted, err := model.NewFile(namespace.ID, owner, "restricted.bin", root.ID)
	require.NoError(t, err)
	restricted.Classification = model.TransferClassificationRestricted
	require.NoError(t, repo.CreateNode(context.Background(), restricted))

	classification, err := repo.GetSubtreeHighestClassification(context.Background(), namespace.ID, root.ID)
	require.NoError(t, err)
	require.Equal(t, model.TransferClassificationRestricted, classification)
}

func openShareSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, AutoMigrate(db))
	return db
}
