package authorization

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type grantAuditRecorder struct{ entries []audit.Entry }

func (r *grantAuditRecorder) Log(_ context.Context, entry audit.Entry) error {
	r.entries = append(r.entries, entry)
	return nil
}

func TestGrantServiceOwnerCanCreateListAndRevokeTransactionally(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	repo := persistence.NewRepository(db)
	owner := uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}
	require.NoError(t, repo.CreateNamespace(context.Background(), namespace))
	node, err := model.NewRootDirectory(namespace.ID, owner, ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), node))
	require.NoError(t, repo.SetNamespaceRoot(context.Background(), namespace.WorkspaceID, node.ID))
	registry := sharedevents.NewRegistry()
	require.NoError(t, filesystemevent.Register(registry))
	bus := messaging.NewReliableBus(registry, messagingpersistence.NewOutboxStore(db), nil, nil, nil)
	audits := &grantAuditRecorder{}
	service := NewGrantService(repo, NewService(repo, nil), postgres.NewTransactor(db), bus, audits)
	createCommandID := uuid.New()
	request := GrantRequest{
		CommandID: createCommandID,
		ActorID:   owner, OrganizationID: namespace.OrganizationID, WorkspaceID: namespace.WorkspaceID,
		NamespaceID: namespace.ID, NodeID: node.ID,
		Grant: model.ResourceGrant{GranteeID: uuid.New(), Permission: model.ResourcePermissionShare, Effect: model.GrantEffectAllow},
	}
	grant, err := service.Create(context.Background(), request)
	require.NoError(t, err)
	replayed, err := service.Create(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, grant.ID, replayed.ID)
	conflicting := request
	conflicting.Grant.GranteeID = uuid.New()
	_, err = service.Create(context.Background(), conflicting)
	require.ErrorIs(t, err, repository.ErrIdempotencyConflict)
	grants, err := service.List(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	request.CommandID = uuid.New()
	require.NoError(t, service.Revoke(context.Background(), request, grant.ID))
	require.NoError(t, service.Revoke(context.Background(), request, grant.ID))
	grants, err = service.List(context.Background(), request)
	require.NoError(t, err)
	require.Empty(t, grants)
	var count int64
	require.NoError(t, db.Table("platform.event_outbox").Where("event_type = ?", filesystemevent.PermissionChangedName).Count(&count).Error)
	require.Equal(t, int64(2), count)
	require.Len(t, audits.entries, 2)
	require.Equal(t, namespace.OrganizationID, audits.entries[0].OrgID.UUID)
}

func TestGrantServiceRejectsForgedCrossTenantScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	repo := persistence.NewRepository(db)
	owner := uuid.New()
	first := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}
	second := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}
	require.NoError(t, repo.CreateNamespace(context.Background(), first))
	require.NoError(t, repo.CreateNamespace(context.Background(), second))
	node, err := model.NewRootDirectory(first.ID, owner, ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), node))
	require.NoError(t, repo.SetNamespaceRoot(context.Background(), first.WorkspaceID, node.ID))
	audits := &grantAuditRecorder{}
	service := NewGrantService(repo, NewService(repo, func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
		return true, nil
	}), postgres.NewTransactor(db), nil, audits)
	request := GrantRequest{
		CommandID: uuid.New(), ActorID: owner, OrganizationID: second.OrganizationID,
		WorkspaceID: first.WorkspaceID, NamespaceID: second.ID, NodeID: node.ID,
		Grant: model.ResourceGrant{GranteeID: uuid.New(), Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow},
	}
	_, err = service.Create(context.Background(), request)
	require.ErrorIs(t, err, domain.ErrResourceDenied)
	require.Len(t, audits.entries, 1)
	require.Equal(t, first.OrganizationID, audits.entries[0].OrgID.UUID)

	request.CommandID = uuid.New()
	request.NamespaceID = first.ID
	_, err = service.Create(context.Background(), request)
	require.ErrorIs(t, err, domain.ErrResourceDenied)
	require.Len(t, audits.entries, 2)
	require.Equal(t, first.OrganizationID, audits.entries[1].OrgID.UUID)

	request.CommandID = uuid.New()
	request.OrganizationID = first.OrganizationID
	request.WorkspaceID = second.WorkspaceID
	_, err = service.Create(context.Background(), request)
	require.ErrorIs(t, err, domain.ErrResourceDenied)
	require.Len(t, audits.entries, 3)
	require.Equal(t, second.OrganizationID, audits.entries[2].OrgID.UUID)
}

func TestGrantServiceSupportsTypedPrincipalsAndRevokeReuse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	repo := persistence.NewRepository(db)
	owner := uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}
	require.NoError(t, repo.CreateNamespace(context.Background(), namespace))
	node, err := model.NewRootDirectory(namespace.ID, owner, ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), node))
	require.NoError(t, repo.SetNamespaceRoot(context.Background(), namespace.WorkspaceID, node.ID))
	service := NewGrantService(repo, NewService(repo, nil), postgres.NewTransactor(db), nil, nil)
	principalID := uuid.New()
	request := GrantRequest{
		CommandID: uuid.New(), ActorID: owner, OrganizationID: namespace.OrganizationID,
		WorkspaceID: namespace.WorkspaceID, NamespaceID: namespace.ID, NodeID: node.ID,
		GranteeOrganizationID: namespace.OrganizationID,
		Grant: model.ResourceGrant{
			PrincipalType: model.PrincipalWorkspace, PrincipalID: principalID,
			Permission: model.ResourcePermissionShare, Effect: model.GrantEffectAllow,
		},
	}
	grant, err := service.Create(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, principalID, grant.GranteeID)
	require.Equal(t, principalID, grant.PrincipalID)
	require.Equal(t, model.PrincipalWorkspace, grant.PrincipalType)

	roleRequest := request
	roleRequest.CommandID = uuid.New()
	roleRequest.Grant.PrincipalType = model.PrincipalRole
	roleGrant, err := service.Create(context.Background(), roleRequest)
	require.NoError(t, err)
	require.Equal(t, principalID, roleGrant.PrincipalID)
	require.Equal(t, model.PrincipalRole, roleGrant.PrincipalType)

	request.CommandID = uuid.New()
	require.NoError(t, service.Revoke(context.Background(), request, grant.ID))
	request.CommandID = uuid.New()
	recreated, err := service.Create(context.Background(), request)
	require.NoError(t, err)
	require.NotEqual(t, grant.ID, recreated.ID)

	var revokedAt *string
	require.NoError(t, db.Table("filesystem.resource_grants").Select("revoked_at").Where("id = ?", grant.ID).Scan(&revokedAt).Error)
	require.NotNil(t, revokedAt)
}
