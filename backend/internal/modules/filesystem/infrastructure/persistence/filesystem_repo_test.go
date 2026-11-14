package persistence

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func TestCreateNamespaceRejectsInvalidTypedNamespace(t *testing.T) {
	repo, _ := newTestRepository(t)

	err := repo.CreateNamespace(context.Background(), &model.Namespace{
		ID:             uuid.New(),
		NamespaceType:  model.NamespaceTypeOrganization,
		OrganizationID: uuid.New(),
		WorkspaceID:    uuid.New(),
	})
	require.ErrorIs(t, err, model.ErrInvalidNamespace)

	err = repo.CreateNamespace(context.Background(), &model.Namespace{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
	})
	require.ErrorIs(t, err, model.ErrInvalidNamespace)
}

func TestCreateNamespaceCreatesQuotaForEveryNamespace(t *testing.T) {
	repo, db := newTestRepository(t)
	organizationID := uuid.New()
	workspaceID := uuid.New()
	userID := uuid.New()

	require.NoError(t, repo.CreateNamespace(context.Background(), &model.Namespace{
		ID:             uuid.New(),
		NamespaceType:  model.NamespaceTypeOrganization,
		OrganizationID: organizationID,
	}))
	require.NoError(t, repo.CreateNamespace(context.Background(), &model.Namespace{
		ID:             uuid.New(),
		NamespaceType:  model.NamespaceTypeUser,
		OrganizationID: organizationID,
		OwnerUserID:    userID,
	}))
	require.NoError(t, repo.CreateNamespace(context.Background(), &model.Namespace{
		ID:             uuid.New(),
		NamespaceType:  model.NamespaceTypeWorkspace,
		OrganizationID: organizationID,
		WorkspaceID:    workspaceID,
	}))

	var quotaCount int64
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("organization_id = ?", organizationID).Count(&quotaCount).Error)
	require.Equal(t, int64(3), quotaCount)
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", workspaceID).Count(&quotaCount).Error)
	require.Equal(t, int64(1), quotaCount)
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id IS NULL").Count(&quotaCount).Error)
	require.Equal(t, int64(2), quotaCount)
}

func TestGetNamespaceByWorkspaceRequiresWorkspaceType(t *testing.T) {
	repo, db := newTestRepository(t)
	workspaceID := uuid.New()
	organizationID := uuid.New()
	require.NoError(t, db.Create(&namespaceRow{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization, OrganizationID: organizationID,
		Status: "ACTIVE",
	}).Error)
	_, err := repo.GetNamespaceByWorkspace(context.Background(), workspaceID)
	require.ErrorIs(t, err, domain.ErrNamespaceNotFound)
}

func TestCreateNamespacePreservesConflictError(t *testing.T) {
	repo, _ := newTestRepository(t)
	namespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace,
		OrganizationID: uuid.New(), WorkspaceID: uuid.New(),
	}
	require.NoError(t, repo.CreateNamespace(context.Background(), namespace))
	require.Error(t, repo.CreateNamespace(context.Background(), namespace))
}

func TestNamespaceRootLinkIsIdempotentOnlyThroughDedicatedMethod(t *testing.T) {
	repo, db := newTestRepository(t)
	namespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace,
		OrganizationID: uuid.New(), WorkspaceID: uuid.New(),
	}
	require.NoError(t, repo.CreateNamespace(context.Background(), namespace))
	root := rootDirectory(namespace.ID, ".root")
	require.NoError(t, repo.CreateNode(context.Background(), root))
	link := model.FileLink{
		ID: uuid.New(), NodeID: root.ID, LinkType: model.FileLinkTypeNamespaceRoot,
		LinkID: namespace.ID, CreatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.CreateFileLink(context.Background(), link))
	require.Error(t, repo.CreateFileLink(context.Background(), link))
	dedicated, ok := repo.(interface {
		CreateNamespaceRootLink(context.Context, model.FileLink) error
	})
	require.True(t, ok)
	require.ErrorIs(t, dedicated.CreateNamespaceRootLink(context.Background(), model.FileLink{
		LinkType: model.FileLinkTypeNamespaceRoot, NodeID: root.ID, LinkID: uuid.Nil,
	}), domain.ErrPersistence)
	require.ErrorIs(t, dedicated.CreateNamespaceRootLink(context.Background(), model.FileLink{
		LinkType: model.FileLinkTypeNamespaceRoot, NodeID: uuid.Nil, LinkID: namespace.ID,
	}), domain.ErrPersistence)
	require.ErrorIs(t, dedicated.CreateNamespaceRootLink(context.Background(), model.FileLink{
		ID: uuid.Nil, LinkType: model.FileLinkTypeNamespaceRoot, NodeID: root.ID, LinkID: namespace.ID,
	}), domain.ErrPersistence)
	require.NoError(t, dedicated.CreateNamespaceRootLink(context.Background(), link))
	require.NoError(t, dedicated.CreateNamespaceRootLink(context.Background(), link))
	var count int64
	require.NoError(t, db.Table("filesystem.file_links").
		Where("link_type = ? AND link_id = ?", model.FileLinkTypeNamespaceRoot, namespace.ID).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSetNamespaceRootByIDUpdatesOnlyRequestedNamespace(t *testing.T) {
	repo, _ := newTestRepository(t)
	first := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization,
		OrganizationID: uuid.New(),
	}
	second := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization,
		OrganizationID: uuid.New(),
	}
	require.NoError(t, repo.CreateNamespace(context.Background(), first))
	require.NoError(t, repo.CreateNamespace(context.Background(), second))
	dedicated, ok := repo.(interface {
		SetNamespaceRootByID(context.Context, uuid.UUID, uuid.UUID) error
	})
	require.True(t, ok)
	root := rootDirectory(first.ID, ".root")
	require.NoError(t, repo.CreateNode(context.Background(), root))
	rootID := root.ID
	require.NoError(t, dedicated.SetNamespaceRootByID(context.Background(), first.ID, rootID))
	gotFirst, err := repo.GetNamespace(context.Background(), first.ID)
	require.NoError(t, err)
	gotSecond, err := repo.GetNamespace(context.Background(), second.ID)
	require.NoError(t, err)
	require.Equal(t, &rootID, gotFirst.RootNodeID)
	require.Nil(t, gotSecond.RootNodeID)
}

func TestNamespaceQueriesAreScopedByTypeAndOrganization(t *testing.T) {
	repo, db := newTestRepository(t)
	organizationID := uuid.New()
	otherOrganizationID := uuid.New()
	userID := uuid.New()
	workspaceID := uuid.New()
	organization := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization, OrganizationID: organizationID}
	user := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeUser, OrganizationID: organizationID, OwnerUserID: userID}
	workspace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: organizationID, WorkspaceID: workspaceID}
	foreignUser := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeUser, OrganizationID: otherOrganizationID, OwnerUserID: userID}
	for _, namespace := range []*model.Namespace{organization, user, workspace, foreignUser} {
		require.NoError(t, repo.CreateNamespace(context.Background(), namespace))
	}

	gotOrganization, err := repo.GetNamespaceByOrganization(context.Background(), organizationID)
	require.NoError(t, err)
	require.Equal(t, organization.ID, gotOrganization.ID)
	gotUser, err := repo.GetNamespaceByUser(context.Background(), organizationID, userID)
	require.NoError(t, err)
	require.Equal(t, user.ID, gotUser.ID)
	_, err = repo.GetNamespaceByUser(context.Background(), otherOrganizationID, userID)
	require.NoError(t, err)
	namespaces, err := repo.ListNamespacesForOrganization(context.Background(), organizationID)
	require.NoError(t, err)
	require.Len(t, namespaces, 3)
	for _, namespace := range namespaces {
		require.Equal(t, organizationID, namespace.OrganizationID)
	}
	require.NoError(t, db.Table("filesystem.namespaces").Where("namespace_type = ?", model.NamespaceTypeWorkspace).Count(new(int64)).Error)
}

func TestCreateChildRejectsDuplicateNormalizedName(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	parent := seedDirectory(t, db, ns.ID)

	require.NoError(t, repo.CreateNode(context.Background(), mustFile(ns.ID, parent.ID, "Report.csv")))
	err := repo.CreateNode(context.Background(), mustFile(ns.ID, parent.ID, "report.CSV"))

	require.ErrorIs(t, err, domain.ErrNodeNameConflict)
}

func TestEqualTimePointersUsesPostgresMicrosecondPrecision(t *testing.T) {
	original := time.Date(2026, time.November, 14, 16, 8, 33, 123456789, time.UTC)
	persisted := original.Truncate(time.Microsecond)

	require.True(t, equalTimePointers(&persisted, &original))

	different := persisted.Add(2 * time.Microsecond)
	require.False(t, equalTimePointers(&persisted, &different))
}

func TestRepositoryUsesAmbientTransactionContext(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	node := rootFile(ns.ID, "transactional")
	tx := postgres.NewTransactor(db)

	err := tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		if err := repo.CreateNode(ctx, node); err != nil {
			return err
		}
		return errors.New("rollback filesystem transaction")
	})

	require.Error(t, err)
	_, err = repo.GetNode(context.Background(), ns.ID, node.ID)
	require.ErrorIs(t, err, domain.ErrNodeNotFound)
}

func TestSaveRevisionUsesAmbientTransactionAndRollsBackAllRows(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	node := rootFile(ns.ID, "transactional-revision")
	require.NoError(t, repo.CreateNode(context.Background(), node))
	revision, blob, replica := revisionFixture(t, node.ID, 1)

	err := postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		if err := repo.SaveRevision(ctx, revision, blob, replica); err != nil {
			return err
		}
		return errors.New("rollback revision transaction")
	})
	require.Error(t, err)
	assertNoFilesystemRows(t, db, revision.ID(), blob.ID, replica.ID)
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

func TestTrashedDirectoryHidesSubtreeAndListChildren(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	parent := rootDirectory(ns.ID, "parent")
	require.NoError(t, repo.CreateNode(context.Background(), parent))
	child := mustFile(ns.ID, parent.ID, "child")
	require.NoError(t, repo.CreateNode(context.Background(), child))
	require.NoError(t, repo.TrashNode(context.Background(), parent.ID))

	_, err := repo.GetNode(context.Background(), ns.ID, parent.ID)
	require.ErrorIs(t, err, domain.ErrNodeNotFound)
	_, err = repo.GetNode(context.Background(), ns.ID, child.ID)
	require.ErrorIs(t, err, domain.ErrNodeNotFound)
	_, _, err = repo.ListChildren(context.Background(), ns.ID, &parent.ID, nil, 100, "")
	require.ErrorIs(t, err, domain.ErrNodeNotFound)
	included, err := repo.GetNodeIncludingTrashed(context.Background(), ns.ID, parent.ID)
	require.NoError(t, err)
	require.Equal(t, model.NodeStateTrashed, included.State)
}

func TestAcquireBlobReusesCanonicalWithinOrganizationButNotAcrossOrganizations(t *testing.T) {
	repo, db := newTestRepository(t)
	organizationID := uuid.New()
	var first, second, foreign *model.Blob
	require.NoError(t, postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		var err error
		first, err = repo.AcquireBlob(ctx, organizationID, strings.Repeat("a", 64), 42)
		if err != nil {
			return err
		}
		second, err = repo.AcquireBlob(ctx, organizationID, strings.Repeat("a", 64), 42)
		if err != nil {
			return err
		}
		foreign, err = repo.AcquireBlob(ctx, uuid.New(), strings.Repeat("a", 64), 42)
		return err
	}))

	require.Equal(t, first.ID, second.ID)
	require.Equal(t, int64(2), second.RefCount)
	require.NotEqual(t, first.ID, foreign.ID)
}

func TestResourceGrantEffectAndExpiryRoundTrip(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	node := rootDirectory(ns.ID, "shared")
	require.NoError(t, repo.CreateNode(context.Background(), node))
	granteeID := uuid.New()
	expired := time.Now().Add(-time.Hour)
	require.NoError(t, db.Create(&resourceGrantRow{ID: uuid.New(), NodeID: node.ID, OrganizationID: ns.OrganizationID, GranteeID: granteeID, PrincipalType: model.PrincipalUser, PrincipalID: granteeID, InheritToChildren: true, PolicyVersion: 1, Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow}).Error)
	require.NoError(t, db.Create(&resourceGrantRow{ID: uuid.New(), NodeID: node.ID, OrganizationID: ns.OrganizationID, GranteeID: granteeID, PrincipalType: model.PrincipalUser, PrincipalID: granteeID, InheritToChildren: true, PolicyVersion: 1, Permission: model.ResourcePermissionRead, Effect: model.GrantEffectDeny, ExpiresAt: &expired}).Error)

	grants, err := repo.ListResourceGrants(context.Background(), []uuid.UUID{node.ID}, []repository.PrincipalRef{{Type: model.PrincipalUser, ID: granteeID}}, model.ResourcePermissionRead, time.Now())
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, model.GrantEffectAllow, grants[0].Effect)
}

func TestListResourceGrantsMatchesExactPrincipalAndRejectsEmptyFilters(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	node := rootDirectory(ns.ID, "typed")
	require.NoError(t, repo.CreateNode(context.Background(), node))
	principalID := uuid.New()
	for _, principalType := range []model.PrincipalType{model.PrincipalWorkspace, model.PrincipalRole} {
		require.NoError(t, repo.CreateResourceGrant(context.Background(), model.ResourceGrant{
			ID: uuid.New(), NodeID: node.ID, OrganizationID: ns.OrganizationID,
			GranteeID: principalID, PrincipalType: principalType, PrincipalID: principalID,
			Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
			InheritToChildren: true, PolicyVersion: 1,
		}))
	}
	grants, err := repo.ListResourceGrants(context.Background(), []uuid.UUID{node.ID}, []repository.PrincipalRef{{Type: model.PrincipalWorkspace, ID: principalID}}, model.ResourcePermissionRead, time.Now())
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, model.PrincipalWorkspace, grants[0].PrincipalType)

	grants, err = repo.ListResourceGrants(context.Background(), nil, []repository.PrincipalRef{{Type: model.PrincipalWorkspace, ID: principalID}}, model.ResourcePermissionRead, time.Now())
	require.NoError(t, err)
	require.Empty(t, grants)
	grants, err = repo.ListResourceGrants(context.Background(), []uuid.UUID{node.ID}, nil, model.ResourcePermissionRead, time.Now())
	require.NoError(t, err)
	require.Empty(t, grants)
}

func TestCreateResourceGrantRevokesExpiredKeyBeforeRecreating(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	node := rootDirectory(ns.ID, "recreate")
	require.NoError(t, repo.CreateNode(context.Background(), node))
	principalID := uuid.New()
	expired := time.Now().Add(-time.Minute)
	first := model.ResourceGrant{
		ID: uuid.New(), NodeID: node.ID, OrganizationID: ns.OrganizationID,
		GranteeID: principalID, PrincipalType: model.PrincipalUser, PrincipalID: principalID,
		Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
		ExpiresAt: &expired, InheritToChildren: true, PolicyVersion: 1,
	}
	require.NoError(t, repo.CreateResourceGrant(context.Background(), first))
	second := first
	second.ID = uuid.New()
	second.ExpiresAt = nil
	require.NoError(t, repo.CreateResourceGrant(context.Background(), second))

	var revokedAt *time.Time
	require.NoError(t, db.Table("filesystem.resource_grants").Where("id = ?", first.ID).Pluck("revoked_at", &revokedAt).Error)
	require.NotNil(t, revokedAt)
	grants, err := repo.ListNodeResourceGrants(context.Background(), node.ID)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, second.ID, grants[0].ID)
}

func TestGetCommandConvertsNotFoundPrecisely(t *testing.T) {
	repo, _ := newTestRepository(t)

	_, err := repo.GetCommand(context.Background(), uuid.New())

	require.ErrorIs(t, err, domain.ErrCommandNotFound)
	require.NotErrorIs(t, err, domain.ErrNodeNotFound)
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

	move := func(namespaceID uuid.UUID, parentID *uuid.UUID, name string) error {
		return postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
			return repo.MoveNodeSafely(ctx, namespaceID, node.ID, parentID, name)
		})
	}
	require.ErrorIs(t, move(ns1.ID, &fileParent.ID, "renamed"), domain.ErrParentNotDirectory)
	require.ErrorIs(t, move(ns1.ID, &foreignParent.ID, "renamed"), domain.ErrParentNotFound)
	require.NoError(t, move(ns1.ID, nil, "  Renamed  "))

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

	first, cursor, err := repo.ListChildren(context.Background(), ns.ID, nil, nil, 2, "")
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.NotEmpty(t, cursor)
	second, next, err := repo.ListChildren(context.Background(), ns.ID, nil, nil, 2, cursor)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Empty(t, next)
	require.NotEqual(t, first[1].ID, second[0].ID)
}

func TestListChildrenFiltersNodeTypeBeforePagination(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	for _, node := range []*model.Node{
		rootDirectory(ns.ID, "docs"),
		rootFile(ns.ID, "notes.txt"),
		rootDirectory(ns.ID, "projects"),
	} {
		require.NoError(t, repo.CreateNode(context.Background(), node))
	}
	nodeType := model.NodeTypeDirectory

	items, next, err := repo.ListChildren(context.Background(), ns.ID, nil, &nodeType, 2, "")

	require.NoError(t, err)
	require.Empty(t, next)
	require.Len(t, items, 2)
	require.Equal(t, model.NodeTypeDirectory, items[0].Type)
	require.Equal(t, model.NodeTypeDirectory, items[1].Type)
}

func TestListChildrenFilteredAppliesQueryBeforeStablePagination(t *testing.T) {
	repo, db := newTestRepository(t)
	filtered, ok := repo.(interface {
		ListChildrenFiltered(context.Context, uuid.UUID, *uuid.UUID, *model.NodeType, string, int, string) ([]model.Node, string, error)
	})
	require.True(t, ok)
	ns := seedNamespace(t, db)
	for _, name := range []string{"design-a.md", "notes.md", "design-b.md"} {
		require.NoError(t, repo.CreateNode(context.Background(), rootFile(ns.ID, name)))
	}

	first, cursor, err := filtered.ListChildrenFiltered(
		context.Background(), ns.ID, nil, nil, "design", 1, "",
	)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Contains(t, first[0].Name, "design")
	require.NotEmpty(t, cursor)

	second, next, err := filtered.ListChildrenFiltered(
		context.Background(), ns.ID, nil, nil, "design", 1, cursor,
	)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Contains(t, second[0].Name, "design")
	require.NotEqual(t, first[0].ID, second[0].ID)
	require.Empty(t, next)
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
		{"unrelated sqlite revision unique", errors.New("UNIQUE constraint failed: file_revisions.future_column"), domain.ErrPersistence},
		{"sqlite revision number unique", errors.New("UNIQUE constraint failed: file_revisions.node_id, file_revisions.revision_no"), domain.ErrRevisionConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, mapUniqueConflict(tt.err), tt.want)
		})
	}
}

func TestClaimCommandUsesConflictFreeInsertBeforeReadingExistingClaim(t *testing.T) {
	_, db := newTestRepository(t)
	commandID := uuid.New()
	workspaceID := uuid.New()
	require.NoError(t, db.Create(&commandDedupRow{
		CommandID: commandID, WorkspaceID: workspaceID, Operation: "put_local",
		RequestHash: "same", State: "COMPLETED",
	}).Error)
	var statements bytes.Buffer
	loggedDB := db.Session(&gorm.Session{Logger: logger.New(
		log.New(&statements, "", 0),
		logger.Config{LogLevel: logger.Info},
	)})
	repo := NewRepository(loggedDB)

	var claimed *repository.CommandDedup
	err := postgres.NewTransactor(loggedDB).WithinTransaction(context.Background(), func(ctx context.Context) error {
		var claimErr error
		claimed, claimErr = repo.ClaimCommand(ctx, repository.CommandDedup{
			CommandID: commandID, WorkspaceID: workspaceID, Operation: "put_local", RequestHash: "same",
		})
		return claimErr
	})

	require.NoError(t, err)
	require.Equal(t, "COMPLETED", claimed.State)
	require.Contains(t, strings.ToUpper(statements.String()), "ON CONFLICT DO NOTHING")
}

func TestClaimCommandConcurrentSQLiteSameAndDifferentHashesAreStable(t *testing.T) {
	tests := []struct {
		name       string
		hashes     [2]string
		secondWant error
	}{
		{name: "same hash", hashes: [2]string{"same", "same"}, secondWant: domain.ErrCommandInProgress},
		{name: "different hash", hashes: [2]string{"first", "different"}, secondWant: domain.ErrIdempotencyConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, verifyDB, mainDSN, filesystemDSN := newConcurrentTestRepository(t)
			commandID := uuid.New()
			workspaceID := uuid.New()
			start := make(chan struct{})
			ready := make(chan error, 2)
			results := make(chan error, 2)
			for _, hash := range tt.hashes {
				hash := hash
				go func() {
					workerDB, openErr := openConcurrentWorkerDB(mainDSN, filesystemDSN)
					ready <- openErr
					if openErr != nil {
						results <- openErr
						return
					}
					sqlDB, _ := workerDB.DB()
					defer sqlDB.Close()
					<-start
					var claimErr error
					for attempt := 0; attempt < 10; attempt++ {
						claimErr = postgres.NewTransactor(workerDB).WithinTransaction(context.Background(), func(ctx context.Context) error {
							_, err := NewRepository(workerDB).ClaimCommand(ctx, repository.CommandDedup{
								CommandID: commandID, WorkspaceID: workspaceID, Operation: "put_local", RequestHash: hash,
							})
							return err
						})
						if !errors.Is(claimErr, domain.ErrTransactionRetry) {
							break
						}
						time.Sleep(time.Duration(attempt+1) * time.Millisecond)
					}
					results <- claimErr
				}()
			}
			require.NoError(t, <-ready)
			require.NoError(t, <-ready)
			close(start)
			first, second := <-results, <-results
			require.True(t,
				(first == nil && errors.Is(second, tt.secondWant)) ||
					(second == nil && errors.Is(first, tt.secondWant)),
				"unexpected results: first=%v second=%v", first, second,
			)
			var count int64
			require.NoError(t, verifyDB.Table("filesystem.command_dedup").Where("command_id = ?", commandID).Count(&count).Error)
			require.Equal(t, int64(1), count)
		})
	}
}

func TestClaimCommandPostgresConflictDoesNotAbortTransaction(t *testing.T) {
	db := openFilesystemPostgres(t)
	repo := NewRepository(db)
	commandID, workspaceID := uuid.New(), uuid.New()
	require.NoError(t, postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		_, err := repo.ClaimCommand(ctx, repository.CommandDedup{
			CommandID: commandID, WorkspaceID: workspaceID, Operation: "put_local", RequestHash: "same",
		})
		if err != nil {
			return err
		}
		return repo.CompleteCommand(ctx, commandID, nil, nil)
	}))
	for _, test := range []struct {
		hash string
		want error
	}{
		{hash: "same"},
		{hash: "different", want: domain.ErrIdempotencyConflict},
	} {
		require.NoError(t, postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
			_, claimErr := repo.ClaimCommand(ctx, repository.CommandDedup{
				CommandID: commandID, WorkspaceID: workspaceID, Operation: "put_local", RequestHash: test.hash,
			})
			if test.want == nil {
				require.NoError(t, claimErr)
			} else {
				require.ErrorIs(t, claimErr, test.want)
			}
			tx, err := postgres.RequireTx(ctx)
			require.NoError(t, err)
			return tx.Exec("SELECT 1").Error
		}))
	}
}

func TestCreateTransferPostgresConflictDoesNotAbortTransaction(t *testing.T) {
	db := openFilesystemPostgres(t)
	migration, err := os.ReadFile("../../../../../migrations/000078_filesystem_transfer.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration)).Error)
	repo := NewRepository(db)
	ns := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}
	require.NoError(t, repo.CreateNamespace(context.Background(), ns))
	parent := rootDirectory(ns.ID, "parent")
	require.NoError(t, repo.CreateNode(context.Background(), parent))
	session := transferSessionFixture(t, ns, &parent.ID, 10)
	require.NoError(t, postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		return repo.CreateTransfer(ctx, session)
	}))

	require.NoError(t, postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		require.ErrorIs(t, repo.CreateTransfer(ctx, session), domain.ErrTransferConflict)
		tx, txErr := postgres.RequireTx(ctx)
		require.NoError(t, txErr)
		return tx.Exec("SELECT 1").Error
	}))
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

func TestTransferRepositoryRoundTripCASAndPartIdempotency(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	parent := seedDirectory(t, db, ns.ID)
	session := transferSessionFixture(t, ns, &parent.ID, 10)

	require.NoError(t, repo.CreateTransfer(context.Background(), session))
	got, err := repo.GetTransferByID(context.Background(), session.ID())
	require.NoError(t, err)
	require.Equal(t, session.ID(), got.ID())
	require.Equal(t, session.TemporaryObjectKey(), got.TemporaryObjectKey())
	byKey, err := repo.GetTransferByIdempotency(context.Background(), ns.WorkspaceID, session.IdempotencyKey())
	require.NoError(t, err)
	require.Equal(t, session.ID(), byKey.ID())

	require.NoError(t, repo.SaveTransferStateCAS(context.Background(), ns.WorkspaceID, session.ID(), 1, repository.TransferStatePatch{
		StatusFrom: model.TransferInitiated, StatusTo: model.TransferUploading,
	}))
	require.ErrorIs(t, repo.SaveTransferStateCAS(context.Background(), ns.WorkspaceID, session.ID(), 1, repository.TransferStatePatch{
		StatusFrom: model.TransferUploading, StatusTo: model.TransferPaused,
	}), domain.ErrTransferVersionConflict)
	require.ErrorIs(t, repo.SaveTransferStateCAS(context.Background(), ns.WorkspaceID, session.ID(), 2, repository.TransferStatePatch{
		StatusFrom: model.TransferInitiated, StatusTo: model.TransferPaused,
	}), domain.ErrTransferStatusConflict)
	require.ErrorIs(t, repo.SaveTransferStateCAS(context.Background(), ns.WorkspaceID, uuid.New(), 1, repository.TransferStatePatch{
		StatusFrom: model.TransferInitiated, StatusTo: model.TransferUploading,
	}), domain.ErrTransferNotFound)
	actualSize, providerID, errorCode := int64(10), uuid.New(), "verify_pending"
	scanCompleted := time.Now().UTC()
	require.NoError(t, repo.SaveTransferStateCAS(context.Background(), ns.WorkspaceID, session.ID(), 2, repository.TransferStatePatch{
		StatusFrom: model.TransferUploading, StatusTo: model.TransferVerifying,
		ActualSize: &actualSize, ActualSHA256: strings.Repeat("a", 64), ProviderID: &providerID,
		ScanResult: &model.TransferScanResult{Clean: true, Engine: "fixture", CompletedAt: scanCompleted},
		ErrorCode:  &errorCode,
	}))
	patched, err := repo.GetTransferByID(context.Background(), session.ID())
	require.NoError(t, err)
	require.Equal(t, model.TransferVerifying, patched.Status())
	require.Equal(t, int64(3), patched.Version())
	require.Equal(t, providerID, *patched.ProviderID())
	require.Equal(t, actualSize, mustActualSize(t, patched))
	require.Equal(t, strings.Repeat("a", 64), patched.ActualSHA256())
	require.Equal(t, "fixture", patched.ScanResult().Engine)
	require.Equal(t, errorCode, patched.ErrorCode())

	first, err := model.NewTransferPart(model.TransferPartInput{
		ID: uuid.New(), SessionID: session.ID(), PartNumber: 1, Offset: 0, Size: 5,
		SessionSize: 10, Checksum: strings.Repeat("a", 64), ETag: "etag-1",
	})
	require.NoError(t, err)
	require.NoError(t, repo.UpsertTransferPart(context.Background(), first))
	require.NoError(t, repo.UpsertTransferPart(context.Background(), first))
	overlap, err := model.NewTransferPart(model.TransferPartInput{
		ID: uuid.New(), SessionID: session.ID(), PartNumber: 2, Offset: 4, Size: 5,
		SessionSize: 10, Checksum: strings.Repeat("b", 64), ETag: "etag-2",
	})
	require.NoError(t, err)
	require.ErrorIs(t, repo.UpsertTransferPart(context.Background(), overlap), domain.ErrTransferPartOverlap)
	conflict, err := model.NewTransferPart(model.TransferPartInput{
		ID: uuid.New(), SessionID: session.ID(), PartNumber: 1, Offset: 0, Size: 5,
		SessionSize: 10, Checksum: strings.Repeat("b", 64), ETag: "changed",
	})
	require.NoError(t, err)
	require.ErrorIs(t, repo.UpsertTransferPart(context.Background(), conflict), domain.ErrTransferPartConflict)
	emptyChecksum, err := model.NewTransferPart(model.TransferPartInput{
		ID: uuid.New(), SessionID: session.ID(), PartNumber: 2, Offset: 5, Size: 5,
		SessionSize: 10, Checksum: "", ETag: "etag-no-checksum",
	})
	require.NoError(t, err)
	require.NoError(t, repo.UpsertTransferPart(context.Background(), emptyChecksum))
	require.NoError(t, repo.UpsertTransferPart(context.Background(), emptyChecksum))
	parts, err := repo.ListTransferParts(context.Background(), session.ID())
	require.NoError(t, err)
	require.Len(t, parts, 2)
	require.Empty(t, parts[1].Checksum())
	var storedChecksum *string
	require.NoError(t, db.Table("filesystem.transfer_parts").
		Select("checksum").Where("session_id = ? AND part_number = ?", session.ID(), 2).
		Scan(&storedChecksum).Error)
	require.Nil(t, storedChecksum)
}

func TestListTransfersScopesFiltersAndPaginatesWithStableBoundary(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	parent := seedDirectory(t, db, ns.ID)
	foreignNS := seedNamespace(t, db)
	foreignParent := seedDirectory(t, db, foreignNS.ID)
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	first := transferSessionFixture(t, ns, &parent.ID, 1)
	second := transferSessionFixture(t, ns, &parent.ID, 2)
	third := transferSessionFixture(t, ns, &parent.ID, 3)
	foreign := transferSessionFixture(t, foreignNS, &foreignParent.ID, 4)
	for index, session := range []*model.TransferSession{first, second, third, foreign} {
		require.NoError(t, repo.CreateTransfer(context.Background(), session))
		require.NoError(t, db.Model(&transferSessionRow{}).Where("id = ?", session.ID()).
			Updates(map[string]any{
				"created_at": createdAt.Add(time.Duration(index/2) * time.Second),
				"status":     model.TransferFailed,
			}).Error)
	}
	otherStatus := transferSessionFixture(t, ns, &parent.ID, 5)
	require.NoError(t, repo.CreateTransfer(context.Background(), otherStatus))
	require.NoError(t, db.Model(&transferSessionRow{}).Where("id = ?", otherStatus.ID()).
		Updates(map[string]any{
			"created_at": createdAt.Add(2 * time.Second),
			"status":     model.TransferUploading,
		}).Error)

	status := model.TransferFailed
	page, next, err := repo.ListTransfers(context.Background(), ns.WorkspaceID, &status, 2, "")
	require.NoError(t, err)
	require.Len(t, page, 2)
	require.NotEmpty(t, next)
	require.Equal(t, third.ID(), page[0].ID())
	tieFirst, tieLast := first.ID(), second.ID()
	if tieFirst.String() < tieLast.String() {
		tieFirst, tieLast = tieLast, tieFirst
	}
	require.Equal(t, tieFirst, page[1].ID())

	last, end, err := repo.ListTransfers(context.Background(), ns.WorkspaceID, &status, 2, next)
	require.NoError(t, err)
	require.Len(t, last, 1)
	require.Empty(t, end)
	require.Equal(t, tieLast, last[0].ID())
	for _, session := range append(page, last...) {
		require.Equal(t, ns.WorkspaceID, session.WorkspaceID())
		require.Equal(t, model.TransferFailed, session.Status())
		require.NotEqual(t, otherStatus.ID(), session.ID())
	}
}

func TestListTransfersRejectsMalformedCursorAndClampsLimits(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	parent := seedDirectory(t, db, ns.ID)
	require.NoError(t, repo.CreateTransfer(
		context.Background(),
		transferSessionFixture(t, ns, &parent.ID, 1),
	))

	_, _, err := repo.ListTransfers(context.Background(), ns.WorkspaceID, nil, 20, "not-base64!")
	require.ErrorIs(t, err, model.ErrInvalidTransfer)

	minimum, _, err := repo.ListTransfers(context.Background(), ns.WorkspaceID, nil, 0, "")
	require.NoError(t, err)
	require.Len(t, minimum, 1)
	maximum, _, err := repo.ListTransfers(context.Background(), ns.WorkspaceID, nil, 101, "")
	require.NoError(t, err)
	require.Len(t, maximum, 1)
}

func TestReserveQuotaHandlesNullBoundariesFilesAndOverflow(t *testing.T) {
	repo, db := newTestRepository(t)
	ns := seedNamespace(t, db)
	account := quotaAccountRow{
		ID: uuid.New(), OrganizationID: ns.OrganizationID, NamespaceID: ns.ID, WorkspaceID: &ns.WorkspaceID,
		UsedBytes: 40, ReservedBytes: 50, UsedFiles: 1, ReservedFiles: 1,
		MaxBytes: ptrInt64(100), MaxFiles: ptrInt64(3), UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(&account).Error)

	accountID, err := repo.ReserveTransferQuota(context.Background(), ns.WorkspaceID, 10, 1)
	require.NoError(t, err)
	require.Equal(t, account.ID, accountID)
	_, err = repo.ReserveTransferQuota(context.Background(), ns.WorkspaceID, 1, 1)
	require.ErrorIs(t, err, domain.ErrQuotaExceeded)

	require.NoError(t, db.Model(&quotaAccountRow{}).Where("id = ?", account.ID).
		Updates(map[string]any{"max_bytes": nil, "max_files": nil, "used_bytes": 0, "reserved_bytes": 0, "used_files": 0, "reserved_files": 0}).Error)
	_, err = repo.ReserveTransferQuota(context.Background(), ns.WorkspaceID, 20, 2)
	require.NoError(t, err)

	require.NoError(t, db.Model(&quotaAccountRow{}).Where("id = ?", account.ID).
		Updates(map[string]any{"max_bytes": nil, "max_files": int64(2), "used_bytes": 0, "reserved_bytes": 0, "used_files": 1, "reserved_files": 1}).Error)
	_, err = repo.ReserveTransferQuota(context.Background(), ns.WorkspaceID, 0, 1)
	require.ErrorIs(t, err, domain.ErrQuotaExceeded)

	require.NoError(t, db.Model(&quotaAccountRow{}).Where("id = ?", account.ID).
		Updates(map[string]any{"max_bytes": nil, "max_files": nil, "used_bytes": int64(^uint64(0) >> 1), "reserved_bytes": 0, "used_files": 0, "reserved_files": 0}).Error)
	_, err = repo.ReserveTransferQuota(context.Background(), ns.WorkspaceID, 1, 1)
	require.ErrorIs(t, err, domain.ErrQuotaExceeded)
}

func transferSessionFixture(t *testing.T, ns *model.Namespace, parentID *uuid.UUID, size int64) *model.TransferSession {
	t.Helper()
	session, err := model.NewTransferSession(model.TransferSessionInput{
		ID: uuid.New(), WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, TargetParentID: parentID,
		TargetName: "payload.bin", CreatedBy: uuid.New(), Protocol: model.TransferProtocolMultipart,
		ExpectedSize: size, ExpectedSHA256: strings.Repeat("a", 64), IdempotencyKey: uuid.NewString(),
		RequestHash:        strings.Repeat("b", 64),
		TemporaryObjectKey: "transfers/" + uuid.NewString(), ExpiresAt: time.Now().Add(time.Hour),
		ProviderID: ptrUUID(uuid.New()), Classification: model.TransferClassificationInternal,
	})
	require.NoError(t, err)
	return session
}

func ptrInt64(value int64) *int64        { return &value }
func ptrUUID(value uuid.UUID) *uuid.UUID { return &value }

func mustActualSize(t *testing.T, session *model.TransferSession) int64 {
	t.Helper()
	size, ok := session.ActualSize()
	require.True(t, ok)
	return size
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

func openConcurrentWorkerDB(mainDSN, filesystemDSN string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(mainDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err := db.Exec("ATTACH DATABASE ? AS filesystem", filesystemDSN).Error; err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := db.Exec("PRAGMA busy_timeout = 5000").Error; err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func seedNamespace(t *testing.T, db *gorm.DB) *model.Namespace {
	t.Helper()
	ns := &model.Namespace{ID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New()}
	require.NoError(t, db.Create(&namespaceRow{ID: ns.ID, OrganizationID: ns.OrganizationID, WorkspaceID: &ns.WorkspaceID, Status: "ACTIVE"}).Error)
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

func openFilesystemPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		if strings.EqualFold(os.Getenv("CI"), "true") || strings.EqualFold(os.Getenv("REQUIRE_INTEGRATION"), "true") {
			t.Fatal("DATABASE_DSN is required when integration tests run in CI")
		}
		t.Skip("DATABASE_DSN is not set")
	}
	admin, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	databaseName := "sparkops_task5_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE DATABASE " + databaseName).Error; err != nil {
		if strings.EqualFold(os.Getenv("CI"), "true") || strings.EqualFold(os.Getenv("REQUIRE_INTEGRATION"), "true") {
			t.Fatalf("cannot create isolated PostgreSQL test database: %v", err)
		}
		t.Skipf("cannot create isolated PostgreSQL test database: %v", err)
	}
	testDSN := dsn
	if strings.Contains(dsn, "://") {
		parsed, parseErr := url.Parse(dsn)
		require.NoError(t, parseErr)
		parsed.Path = "/" + databaseName
		testDSN = parsed.String()
	} else {
		databaseSetting := regexp.MustCompile(`(?i)\b(?:dbname|database)\s*=\s*(?:'[^']*'|\S+)`)
		if databaseSetting.MatchString(dsn) {
			testDSN = databaseSetting.ReplaceAllString(dsn, "dbname="+databaseName)
		} else {
			testDSN = dsn + " dbname=" + databaseName
		}
	}
	db, err := gorm.Open(gormpostgres.Open(testDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		require.NoError(t, dbErr)
		require.NoError(t, sqlDB.Close())
		require.NoError(t, admin.Exec("DROP DATABASE "+databaseName).Error)
		adminDB, adminErr := admin.DB()
		require.NoError(t, adminErr)
		require.NoError(t, adminDB.Close())
	})
	migration, err := os.ReadFile("../../../../../migrations/000077_filesystem_kernel.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration)).Error)
	return db
}
