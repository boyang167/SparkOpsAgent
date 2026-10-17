package persistence

import (
	"context"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestBeginArchiveCASAllowsOnlyOneGeneration(t *testing.T) {
	repo, project := openProjectLifecycleDB(t)
	generationA, generationB := uuid.NewString(), uuid.NewString()

	first, err := repo.BeginArchive(context.Background(), project.ID, generationA, false)
	require.NoError(t, err)
	second, err := repo.BeginArchive(context.Background(), project.ID, generationB, true)
	require.NoError(t, err)

	require.True(t, first)
	require.False(t, second)
	current := readArchiveLifecycle(t, repo.db, project.ID)
	require.Equal(t, generationA, current.Generation)
	require.Equal(t, model.ArchivePhasePending, current.Phase)
	require.NotNil(t, current.AllowUndeploy)
	require.False(t, *current.AllowUndeploy)
}

func TestConcurrentBeginArchiveCASAllowsOnlyOneWinner(t *testing.T) {
	repo, project := openProjectLifecycleDB(t)
	const contenders = 8
	start := make(chan struct{})
	results := make(chan bool, contenders)
	errors := make(chan error, contenders)

	var workers sync.WaitGroup
	workers.Add(contenders)
	for range contenders {
		go func() {
			defer workers.Done()
			<-start
			begun, err := repo.BeginArchive(
				context.Background(), project.ID, uuid.NewString(), true,
			)
			results <- begun
			errors <- err
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(errors)

	winners := 0
	for begun := range results {
		if begun {
			winners++
		}
	}
	for err := range errors {
		require.NoError(t, err)
	}
	require.Equal(t, 1, winners)

	current := readArchiveLifecycle(t, repo.db, project.ID)
	require.Equal(t, model.ProjectStatusArchiving, current.Status)
	require.NotEmpty(t, current.Generation)
	require.Equal(t, model.ArchivePhasePending, current.Phase)
	require.NotNil(t, current.AllowUndeploy)
	require.True(t, *current.AllowUndeploy)
}

func TestBeginArchiveCASRejectsConcurrentlyBoundProduct(t *testing.T) {
	repo, project := openProjectLifecycleDB(t)
	require.NoError(t, repo.db.Table("workspace_app.projects").
		Where("id = ?", project.ID.String()).
		Update("product_id", kernel.NewProductID().String()).Error)

	begun, err := repo.BeginArchive(context.Background(), project.ID, uuid.NewString(), true)
	require.NoError(t, err)
	require.False(t, begun)

	current := readArchiveLifecycle(t, repo.db, project.ID)
	require.Equal(t, model.ProjectStatusActive, current.Status)
	require.Empty(t, current.Generation)
	require.Equal(t, model.ArchivePhaseNone, current.Phase)
	require.Nil(t, current.AllowUndeploy)
}

func TestStaleActiveUpdateCannotOverwriteClaimedArchiveLifecycle(t *testing.T) {
	repo, stale := openProjectLifecycleDB(t)
	generation := uuid.NewString()
	begun, err := repo.BeginArchive(context.Background(), stale.ID, generation, true)
	require.NoError(t, err)
	require.True(t, begun)
	claimed, err := repo.TransitionArchivePhase(
		context.Background(), stale.ID, generation,
		model.ArchivePhasePending, model.ArchivePhaseUndeployClaimed,
	)
	require.NoError(t, err)
	require.True(t, claimed)

	stale.DisplayName = "stale write"
	require.Error(t, repo.Update(context.Background(), stale))

	current := readArchiveLifecycle(t, repo.db, stale.ID)
	require.Equal(t, model.ProjectStatusArchiving, current.Status)
	require.Equal(t, generation, current.Generation)
	require.Equal(t, model.ArchivePhaseUndeployClaimed, current.Phase)
	require.NotNil(t, current.AllowUndeploy)
	require.True(t, *current.AllowUndeploy)
	require.NotEqual(t, "stale write", current.DisplayName)
}

func TestStaleActiveUpdateCannotOverwriteFinalizingArchiveLifecycle(t *testing.T) {
	repo, stale := openProjectLifecycleDB(t)
	generation := uuid.NewString()
	begun, err := repo.BeginArchive(context.Background(), stale.ID, generation, true)
	require.NoError(t, err)
	require.True(t, begun)
	for _, transition := range [][2]model.ArchivePhase{
		{model.ArchivePhasePending, model.ArchivePhasePipelinePending},
		{model.ArchivePhasePipelinePending, model.ArchivePhaseFinalizing},
	} {
		advanced, err := repo.TransitionArchivePhase(
			context.Background(), stale.ID, generation, transition[0], transition[1],
		)
		require.NoError(t, err)
		require.True(t, advanced)
	}

	stale.Description = "stale write"
	require.Error(t, repo.Update(context.Background(), stale))
	current := readArchiveLifecycle(t, repo.db, stale.ID)
	require.Equal(t, model.ArchivePhaseFinalizing, current.Phase)
	require.Equal(t, generation, current.Generation)
}

func TestProjectTerminalCASTwoTransactionsCannotOverwriteArchiveWinner(t *testing.T) {
	repo, tx, projectID, generation := openProjectTerminalCASDB(t)

	require.NoError(t, setArchivePhase(t, repo.db, projectID, model.ArchivePhasePipelinePending))
	var claimed bool
	require.NoError(t, tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		var err error
		claimed, err = repo.TransitionArchivePhase(
			ctx, projectID, generation, model.ArchivePhasePipelinePending, model.ArchivePhaseFinalizing,
		)
		return err
	}))
	var archived bool
	require.NoError(t, tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		var err error
		archived, err = repo.TransitionArchiving(
			ctx, projectID, generation, model.ArchivePhaseFinalizing, model.ProjectStatusArchived,
		)
		return err
	}))
	var activated bool
	require.NoError(t, tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		var err error
		activated, err = repo.TransitionArchiving(
			ctx, projectID, generation, model.ArchivePhasePending, model.ProjectStatusActive,
		)
		return err
	}))

	require.True(t, claimed)
	require.True(t, archived)
	require.False(t, activated)
	require.Equal(t, model.ProjectStatusArchived, readProjectStatus(t, repo.db, projectID))
	require.Equal(t, model.ArchivePhaseNone, readArchivePhase(t, repo.db, projectID))
}

func TestProjectTerminalCASTwoTransactionsCannotOverwriteAbortWinner(t *testing.T) {
	repo, tx, projectID, generation := openProjectTerminalCASDB(t)

	var activated bool
	require.NoError(t, tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		var err error
		activated, err = repo.TransitionArchiving(
			ctx, projectID, generation, model.ArchivePhasePending, model.ProjectStatusActive,
		)
		return err
	}))
	var claimed bool
	require.NoError(t, tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		var err error
		claimed, err = repo.TransitionArchivePhase(
			ctx, projectID, generation, model.ArchivePhasePending, model.ArchivePhaseUndeployClaimed,
		)
		return err
	}))

	require.True(t, activated)
	require.False(t, claimed)
	require.Equal(t, model.ProjectStatusActive, readProjectStatus(t, repo.db, projectID))
	require.Equal(t, model.ArchivePhaseNone, readArchivePhase(t, repo.db, projectID))
}

func TestProjectTerminalCASConcurrentClaimAndAbortHaveSingleWinner(t *testing.T) {
	archiveDB := t.TempDir() + "/workspace.db"
	open := func(name string) *gorm.DB {
		db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(1)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		require.NoError(t, db.Exec("ATTACH DATABASE ? AS workspace_app", archiveDB).Error)
		require.NoError(t, db.Exec("PRAGMA workspace_app.busy_timeout = 5000").Error)
		return db
	}
	dbA := open(t.Name() + "-a")
	require.NoError(t, dbA.Exec(`
		CREATE TABLE workspace_app.projects (
			id TEXT PRIMARY KEY, status TEXT NOT NULL, archive_generation TEXT,
			archive_phase TEXT NOT NULL, archive_allow_undeploy BOOLEAN,
			updated_at DATETIME, deleted_at DATETIME
		)
	`).Error)
	dbB := open(t.Name() + "-b")
	projectID := kernel.NewProjectID()
	generation := uuid.NewString()
	require.NoError(t, dbA.Exec(
		"INSERT INTO workspace_app.projects (id, status, archive_generation, archive_phase, archive_allow_undeploy) VALUES (?, ?, ?, ?, ?)",
		projectID.String(), model.ProjectStatusArchiving, generation, model.ArchivePhasePending, true,
	).Error)
	repoA, repoB := NewProjectRepository(dbA), NewProjectRepository(dbB)
	type result struct {
		won bool
		err error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	go func() {
		<-start
		won, err := repoA.TransitionArchivePhase(
			context.Background(), projectID, generation,
			model.ArchivePhasePending, model.ArchivePhaseUndeployClaimed,
		)
		results <- result{won: won, err: err}
	}()
	go func() {
		<-start
		won, err := repoB.TransitionArchiving(
			context.Background(), projectID, generation,
			model.ArchivePhasePending, model.ProjectStatusActive,
		)
		results <- result{won: won, err: err}
	}()
	close(start)
	first, second := <-results, <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.NotEqual(t, first.won, second.won)

	status := readProjectStatus(t, dbA, projectID)
	phase := readArchivePhase(t, dbA, projectID)
	if status == model.ProjectStatusActive {
		require.Equal(t, model.ArchivePhaseNone, phase)
	} else {
		require.Equal(t, model.ProjectStatusArchiving, status)
		require.Equal(t, model.ArchivePhaseUndeployClaimed, phase)
	}
}

func TestProjectModelGORMFallbackIncludesArchiveFence(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.Table("projects").AutoMigrate(&projectModel{}))
	var columns []struct{ Name string }
	require.NoError(t, db.Raw("PRAGMA table_info(projects)").Scan(&columns).Error)
	require.Contains(t, columns, struct{ Name string }{Name: "archive_generation"})
	require.Contains(t, columns, struct{ Name string }{Name: "archive_phase"})
}

func openProjectTerminalCASDB(t *testing.T) (*ProjectRepository, postgres.Transactor, kernel.ProjectID, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS workspace_app").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE workspace_app.projects (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			archive_generation TEXT,
			archive_phase TEXT NOT NULL DEFAULT 'none',
			archive_allow_undeploy BOOLEAN,
			updated_at DATETIME,
			deleted_at DATETIME
		)
	`).Error)
	projectID := kernel.NewProjectID()
	generation := "generation-a"
	require.NoError(t, db.Exec(
		"INSERT INTO workspace_app.projects (id, status, archive_generation, archive_phase, archive_allow_undeploy) VALUES (?, ?, ?, ?, ?)",
		projectID.String(), model.ProjectStatusArchiving, generation, model.ArchivePhasePending, true,
	).Error)
	return &ProjectRepository{db: db}, postgres.NewTransactor(db), projectID, generation
}

func setArchivePhase(t *testing.T, db *gorm.DB, projectID kernel.ProjectID, phase model.ArchivePhase) error {
	t.Helper()
	return db.Table("workspace_app.projects").Where("id = ?", projectID.String()).
		Update("archive_phase", string(phase)).Error
}

func readArchivePhase(t *testing.T, db *gorm.DB, projectID kernel.ProjectID) model.ArchivePhase {
	t.Helper()
	var phase string
	require.NoError(t, db.Table("workspace_app.projects").
		Where("id = ?", projectID.String()).Pluck("archive_phase", &phase).Error)
	return model.ArchivePhase(phase)
}

type archiveLifecycleRow struct {
	Status        model.ProjectStatus
	Generation    string
	Phase         model.ArchivePhase
	AllowUndeploy *bool
	DisplayName   string
}

func readArchiveLifecycle(t *testing.T, db *gorm.DB, projectID kernel.ProjectID) archiveLifecycleRow {
	t.Helper()
	var row struct {
		Status               string
		ArchiveGeneration    string
		ArchivePhase         string
		ArchiveAllowUndeploy *bool
		DisplayName          string
	}
	require.NoError(t, db.Table("workspace_app.projects").
		Where("id = ?", projectID.String()).Take(&row).Error)
	return archiveLifecycleRow{
		Status: model.ProjectStatus(row.Status), Generation: row.ArchiveGeneration,
		Phase:         model.ArchivePhase(row.ArchivePhase),
		AllowUndeploy: row.ArchiveAllowUndeploy, DisplayName: row.DisplayName,
	}
}

func openProjectLifecycleDB(t *testing.T) (*ProjectRepository, *model.Project) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS workspace_app").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE workspace_app.projects (
			id TEXT PRIMARY KEY, workspace_id TEXT, product_id TEXT, name TEXT,
			display_name TEXT, description TEXT, namespace TEXT, language TEXT,
			git_url TEXT, secret_id TEXT, git_ref_type TEXT, git_ref_value TEXT,
			dockerfile_path TEXT, template_id TEXT, default_cluster_id TEXT,
			jenkins_secret_id TEXT, kubernetes_secret_id TEXT, resources BLOB,
			status TEXT, archive_generation TEXT, archive_phase TEXT,
			archive_allow_undeploy BOOLEAN, review_status TEXT,
			submitted_by TEXT, submitted_at DATETIME, reviewed_by TEXT,
			reviewed_at DATETIME, review_comment TEXT, created_by TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		)
	`).Error)

	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(
		kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"},
	)
	repo := NewProjectRepository(db)
	require.NoError(t, repo.Save(context.Background(), project))
	stale := *project
	return repo, &stale
}

func readProjectStatus(t *testing.T, db *gorm.DB, projectID kernel.ProjectID) model.ProjectStatus {
	t.Helper()
	var status string
	require.NoError(t, db.Table("workspace_app.projects").
		Where("id = ?", projectID.String()).Pluck("status", &status).Error)
	return model.ProjectStatus(status)
}
