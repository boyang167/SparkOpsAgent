package command_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	workspaceevent "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	workspacerepository "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type atomicProjectRepo struct {
	workspacerepository.ProjectRepository
	db *gorm.DB
}

func (*atomicProjectRepo) FindByWorkspaceAndName(context.Context, kernel.WorkspaceID, model.ProjectName) (*model.Project, error) {
	return nil, domain.ErrProjectNotFound
}
func (*atomicProjectRepo) CountByWorkspace(context.Context, kernel.WorkspaceID) (int, error) {
	return 0, nil
}
func (r *atomicProjectRepo) Save(ctx context.Context, project *model.Project) error {
	return postgres.DB(ctx, r.db).Table("workspace_app.projects").Create(map[string]any{
		"id": project.ID.String(), "workspace_id": project.WorkspaceID.String(),
		"name": project.Name.String(), "created_by": project.CreatedBy.String(),
	}).Error
}

type atomicProjectMemberRepo struct {
	workspacerepository.ProjectMemberRepository
	db *gorm.DB
}

func (r *atomicProjectMemberRepo) Save(ctx context.Context, member *model.ProjectMember) error {
	return postgres.DB(ctx, r.db).Table("workspace_app.project_members").Create(map[string]any{
		"project_id": member.ProjectID.String(), "member_id": member.MemberID.String(), "role": member.Role,
	}).Error
}

func TestProjectLeadAndOutboxAreAtomic(t *testing.T) {
	db := openProjectAtomicityDB(t)
	repo := &atomicProjectRepo{db: db}
	memberRepo := &atomicProjectMemberRepo{db: db}
	tx := postgres.NewTransactor(db)

	handler := command.NewCreateProjectHandler(
		repo, memberRepo, updateWorkspaceChecker{}, nil, nil, nil,
		messaging.NewReliableBus(
			sharedevents.NewRegistry(),
			messagingpersistence.NewOutboxStore(db),
			nil, nil, nil,
		),
		tx,
	)
	_, err := handler.Handle(
		context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"},
	)
	require.Error(t, err)
	requireProjectAtomicityCount(t, db, "workspace_app.projects", 0)
	requireProjectAtomicityCount(t, db, "workspace_app.project_members", 0)
	requireProjectAtomicityCount(t, db, "platform.event_outbox", 0)

	registry := sharedevents.NewRegistry()
	require.NoError(t, workspaceevent.Register(registry))
	handler = command.NewCreateProjectHandler(
		repo, memberRepo, updateWorkspaceChecker{}, nil, nil, nil,
		messaging.NewReliableBus(
			registry,
			messagingpersistence.NewOutboxStore(db),
			nil, nil, nil,
		),
		tx,
	)
	_, err = handler.Handle(
		context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"},
	)
	require.NoError(t, err)
	requireProjectAtomicityCount(t, db, "workspace_app.projects", 1)
	requireProjectAtomicityCount(t, db, "workspace_app.project_members", 1)
	requireProjectAtomicityCount(t, db, "platform.event_outbox", 1)
}

func openProjectAtomicityDB(t *testing.T) *gorm.DB {
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
		CREATE TABLE workspace_app.projects (
			id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, created_by TEXT
		);
		CREATE TABLE workspace_app.project_members (
			project_id TEXT, member_id TEXT, role TEXT
		)
	`).Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	return db
}

func requireProjectAtomicityCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Count(&count).Error)
	require.Equal(t, want, count)
}
