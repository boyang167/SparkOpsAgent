package authorization

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

func TestGrantServiceOwnerCanCreateListAndRevokeTransactionally(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	repo := persistence.NewRepository(db)
	owner := uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}
	require.NoError(t, repo.CreateNamespace(context.Background(), namespace))
	node, err := model.NewRootDirectory(namespace.ID, owner, ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), node))
	require.NoError(t, repo.SetNamespaceRoot(context.Background(), namespace.WorkspaceID, node.ID))
	registry := sharedevents.NewRegistry()
	require.NoError(t, filesystemevent.Register(registry))
	bus := messaging.NewReliableBus(registry, messagingpersistence.NewOutboxStore(db), nil, nil, nil)
	service := NewGrantService(repo, NewService(repo, nil), postgres.NewTransactor(db), bus, nil)
	request := GrantRequest{
		ActorID: owner, OrganizationID: namespace.OrganizationID, WorkspaceID: namespace.WorkspaceID,
		NamespaceID: namespace.ID, NodeID: node.ID,
		Grant: model.ResourceGrant{GranteeID: uuid.New(), Permission: model.ResourcePermissionShare, Effect: model.GrantEffectAllow},
	}
	grant, err := service.Create(context.Background(), request)
	require.NoError(t, err)
	grants, err := service.List(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.NoError(t, service.Revoke(context.Background(), request, grant.ID))
	grants, err = service.List(context.Background(), request)
	require.NoError(t, err)
	require.Empty(t, grants)
	var count int64
	require.NoError(t, db.Table("platform.event_outbox").Where("event_type = ?", filesystemevent.PermissionChangedName).Count(&count).Error)
	require.Equal(t, int64(2), count)
}
