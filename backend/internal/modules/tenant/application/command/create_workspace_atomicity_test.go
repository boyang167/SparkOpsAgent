package command_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/model"
	tenantrepository "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type atomicWorkspaceOrgRepo struct {
	tenantrepository.OrganizationRepository
	db  *gorm.DB
	org *model.Organization
}

func (r *atomicWorkspaceOrgRepo) FindByID(context.Context, kernel.OrgID) (*model.Organization, error) {
	return r.org, nil
}
func (*atomicWorkspaceOrgRepo) GetUsage(context.Context, kernel.OrgID) (*tenantrepository.OrgUsage, error) {
	return &tenantrepository.OrgUsage{}, nil
}
func (r *atomicWorkspaceOrgRepo) AddMember(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID, role string) error {
	return postgres.DB(ctx, r.db).Table("tenant.organization_members").Create(map[string]any{
		"org_id": orgID.String(), "member_id": memberID.String(), "role": role,
	}).Error
}

type atomicWorkspaceRepo struct {
	tenantrepository.WorkspaceRepository
	db *gorm.DB
}

func (*atomicWorkspaceRepo) ExistsByOrgSlug(context.Context, kernel.OrgID, string) (bool, error) {
	return false, nil
}
func (r *atomicWorkspaceRepo) Save(ctx context.Context, workspace *model.Workspace) error {
	return postgres.DB(ctx, r.db).Table("tenant.workspaces").Create(map[string]any{
		"id": workspace.ID.String(), "org_id": workspace.OrgID.String(), "name": workspace.Name, "slug": workspace.Slug,
	}).Error
}
func (r *atomicWorkspaceRepo) AddMember(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID, role string) error {
	return postgres.DB(ctx, r.db).Table("tenant.workspace_members").Create(map[string]any{
		"workspace_id": workspaceID.String(), "member_id": memberID.String(), "role": role,
	}).Error
}

func TestWorkspaceMembershipsAndOutboxAreAtomic(t *testing.T) {
	db := openTenantAtomicityDB(t)
	org := model.NewOrganization("Acme", "acme")
	orgRepo := &atomicWorkspaceOrgRepo{db: db, org: org}
	workspaceRepo := &atomicWorkspaceRepo{db: db}
	tx := postgres.NewTransactor(db)

	handler := command.NewCreateWorkspaceHandler(
		orgRepo,
		workspaceRepo,
		messaging.NewReliableBus(
			sharedevents.NewRegistry(),
			messagingpersistence.NewOutboxStore(db),
			nil, nil, nil,
		),
		tx,
	)
	_, err := handler.Handle(context.Background(), org.ID, kernel.NewMemberID(), "Platform", "platform")
	require.Error(t, err)
	requireTenantAtomicityCount(t, db, "tenant.workspaces", 0)
	requireTenantAtomicityCount(t, db, "tenant.workspace_members", 0)
	requireTenantAtomicityCount(t, db, "tenant.organization_members", 0)
	requireTenantAtomicityCount(t, db, "platform.event_outbox", 0)

	registry := sharedevents.NewRegistry()
	require.NoError(t, integration.RegisterWorkspaceCreated(registry))
	handler = command.NewCreateWorkspaceHandler(
		orgRepo,
		workspaceRepo,
		messaging.NewReliableBus(
			registry,
			messagingpersistence.NewOutboxStore(db),
			nil, nil, nil,
		),
		tx,
	)
	_, err = handler.Handle(context.Background(), org.ID, kernel.NewMemberID(), "Platform", "platform")
	require.NoError(t, err)
	requireTenantAtomicityCount(t, db, "tenant.workspaces", 1)
	requireTenantAtomicityCount(t, db, "tenant.workspace_members", 1)
	requireTenantAtomicityCount(t, db, "tenant.organization_members", 1)
	requireTenantAtomicityCount(t, db, "platform.event_outbox", 1)
}

func openTenantAtomicityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS tenant").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE tenant.workspaces (id TEXT PRIMARY KEY, org_id TEXT, name TEXT, slug TEXT);
		CREATE TABLE tenant.workspace_members (workspace_id TEXT, member_id TEXT, role TEXT);
		CREATE TABLE tenant.organization_members (org_id TEXT, member_id TEXT, role TEXT)
	`).Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	return db
}

func requireTenantAtomicityCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Count(&count).Error)
	require.Equal(t, want, count)
}
