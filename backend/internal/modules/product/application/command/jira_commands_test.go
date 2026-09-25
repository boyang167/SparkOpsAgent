package command_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type stubJiraConnector struct {
	issues []port.PmIssue
}

type stubPmAdapter struct {
	inner stubJiraConnector
}

func (s stubPmAdapter) Provider() string { return port.PmProviderJira }

func (s stubPmAdapter) GetIterationIssues(ctx context.Context, wsID kernel.WorkspaceID, _, iterationID string, types []string) ([]port.PmIssue, error) {
	return s.inner.GetSprintIssues(ctx, wsID, iterationID, types)
}

func (s stubPmAdapter) GetIssue(ctx context.Context, wsID kernel.WorkspaceID, _, issueKey string) (*port.PmIssue, error) {
	return s.inner.GetIssue(ctx, wsID, issueKey)
}

func (s stubPmAdapter) TransitionIssue(ctx context.Context, wsID kernel.WorkspaceID, _, issueKey, target string) error {
	return s.inner.TransitionIssue(ctx, wsID, issueKey, target)
}

func (s stubJiraConnector) GetSprintIssues(_ context.Context, _ kernel.WorkspaceID, _ string, _ []string) ([]port.PmIssue, error) {
	return s.issues, nil
}

func (s stubJiraConnector) GetIssue(_ context.Context, _ kernel.WorkspaceID, issueKey string) (*port.PmIssue, error) {
	return &port.PmIssue{Key: issueKey, Summary: issueKey + " summary", IssueType: "Story", URL: "https://jira.example/browse/" + issueKey}, nil
}

func (s stubJiraConnector) TransitionIssue(context.Context, kernel.WorkspaceID, string, string) error {
	return nil
}

func openJiraCommandTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS product").Error)
	require.NoError(t, db.Exec(`CREATE TABLE product.products (
		id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, display_name TEXT,
		description TEXT, version TEXT, next_version TEXT, created_by TEXT,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE product.sprint_processes (
		id TEXT PRIMARY KEY, product_id TEXT, name TEXT, template_key TEXT, description TEXT,
		version INT, is_default INT, stages TEXT, created_by TEXT, created_at DATETIME, updated_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE product.sprints (
		id TEXT PRIMARY KEY, product_id TEXT, process_id TEXT, process_version INT, stages TEXT,
		name TEXT, goal TEXT, status TEXT, planned_start DATETIME, planned_end DATETIME,
		actual_start DATETIME, actual_end DATETIME, owner_id TEXT, external_ref TEXT,
		legacy_plan_id TEXT, created_by TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE product.work_items (
		id TEXT PRIMARY KEY, sprint_id TEXT, stage_id TEXT, parent_id TEXT, sort_order INT,
		type TEXT, title TEXT, description TEXT, priority TEXT, status TEXT,
		assignee_id TEXT, reporter_id TEXT, project_ids TEXT, labels TEXT, external_ref TEXT,
		delivery TEXT, blocked_reason TEXT, version INT, created_by TEXT,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	return db
}

func TestImportJiraSprintSkipsDuplicates(t *testing.T) {
	ctx := context.Background()
	db := openJiraCommandTestDB(t)
	productRepo := persistence.NewProductRepository(db)
	sprintRepo := persistence.NewSprintRepository(db)

	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	name, err := model.NewProductName("jira-product")
	require.NoError(t, err)
	product := model.NewProduct(wsID, name, "Jira 测试", "", "1.0.0", "1.0.0", memberID)
	require.NoError(t, productRepo.SaveProduct(ctx, product))

	stages := model.DefaultSprintStages()
	process := model.NewSprintProcess(product.ID, "标准流程", stages, memberID)
	require.NoError(t, sprintRepo.SaveProcess(ctx, process))
	sprint := model.NewSprint(product.ID, process, "v1.0", "", nil, memberID)
	require.NoError(t, sprintRepo.SaveSprint(ctx, sprint))

	existing := model.NewWorkItem(sprint.ID, stages[0].ID, "已有 Story", model.WorkItemTypeStory, model.WorkItemPriorityMedium, memberID)
	existing.ExternalRef = model.BuildPmExternalRef(port.PmProviderJira, port.PmIssue{Key: "PROJ-1"}, "link_only", "")
	require.NoError(t, sprintRepo.SaveWorkItem(ctx, existing))

	jira := stubJiraConnector{issues: []port.PmIssue{
		{Key: "PROJ-1", Summary: "重复", IssueType: "Story"},
		{Key: "PROJ-2", Summary: "新 Story", IssueType: "Story", Priority: "High"},
	}}
	pm := port.NewPmConnectorRouter(stubPmAdapter{inner: jira})
	handler := command.NewImportPmSprintHandler(productRepo, sprintRepo, pm, nil)
	result, err := handler.Handle(ctx, product.ID, sprint.ID, memberID, command.ImportPmSprintInput{
		Provider: port.PmProviderJira, IterationID: "42", SyncMode: "link_only",
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.ImportedCount)
	require.Equal(t, 1, result.SkippedCount)
	require.Len(t, result.WorkItemIDs, 1)

	items, err := sprintRepo.ListWorkItemsBySprint(ctx, sprint.ID)
	require.NoError(t, err)
	require.Len(t, items, 2)
}
