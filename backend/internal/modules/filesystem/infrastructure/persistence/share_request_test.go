package persistence

import (
	"context"
	"strings"
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

func TestGetNodeClassificationUsesAuthoritativeTransferState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, AutoMigrate(db))
	repo := NewRepository(db)
	owner, sessionID := uuid.New(), uuid.New()
	namespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: uuid.New(),
		WorkspaceID: uuid.New(), Status: "ACTIVE",
	}
	require.NoError(t, repo.CreateNamespace(context.Background(), namespace))
	root, err := model.NewRootDirectory(namespace.ID, owner, ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), root))
	require.NoError(t, repo.SetNamespaceRoot(context.Background(), namespace.WorkspaceID, root.ID))
	session, err := model.NewTransferSession(model.TransferSessionInput{
		ID: sessionID, WorkspaceID: namespace.WorkspaceID, NamespaceID: namespace.ID, TargetParentID: &root.ID,
		TargetName: "restricted.bin", CreatedBy: owner, Protocol: model.TransferProtocolMultipart,
		ExpectedSize: 1, ExpectedSHA256: strings.Repeat("a", 64), IdempotencyKey: "classification-1",
		RequestHash: strings.Repeat("b", 64), TemporaryObjectKey: "transfers/restricted",
		ExpiresAt: time.Now().UTC().Add(time.Hour), Classification: model.TransferClassificationRestricted,
	})
	require.NoError(t, err)
	require.NoError(t, repo.CreateTransfer(context.Background(), session))
	require.NoError(t, db.Table("filesystem.transfer_sessions").Where("id = ?", sessionID).Update("status", model.TransferCompleted).Error)
	nodeID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-transfer-node:"+sessionID.String()))
	node, err := model.NewFile(namespace.ID, owner, "restricted.bin", root.ID)
	require.NoError(t, err)
	node.ID = nodeID
	require.NoError(t, repo.CreateNode(context.Background(), node))

	classification, err := repo.GetNodeClassification(context.Background(), nodeID)
	require.NoError(t, err)
	require.Equal(t, model.TransferClassificationRestricted, classification)
}
