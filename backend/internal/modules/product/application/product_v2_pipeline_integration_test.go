package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	pwfcommand "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/command"
	pwfdomain "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	pwfmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	pwfport "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/port"
	pwfpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type stubWorkspaceChecker struct {
	wsID kernel.WorkspaceID
}

func (s stubWorkspaceChecker) MemberInWorkspace(context.Context, kernel.MemberID, kernel.WorkspaceID) (bool, error) {
	return true, nil
}

type stubProductWorkflowChecker struct {
	wsID      kernel.WorkspaceID
	productID kernel.ProductID
}

func (s stubProductWorkflowChecker) ProductWorkspace(_ context.Context, productID kernel.ProductID) (kernel.WorkspaceID, error) {
	if productID == s.productID {
		return s.wsID, nil
	}
	return kernel.WorkspaceID{}, pwfdomain.ErrProductNotInWorkspace
}

func (s stubProductWorkflowChecker) ProductName(_ context.Context, productID kernel.ProductID) (string, error) {
	if productID == s.productID {
		return "集成测试产品", nil
	}
	return "", pwfdomain.ErrProductNotInWorkspace
}

func createProductV2TestTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE product.products (
			id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, display_name TEXT,
			description TEXT, version TEXT, next_version TEXT, created_by TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE product.product_plans (
			id TEXT PRIMARY KEY, product_id TEXT, name TEXT, version TEXT, status TEXT,
			description TEXT, submitted INTEGER, other_info TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE product.product_requirements (
			id TEXT PRIMARY KEY, product_id TEXT, product_plan_id TEXT, project_id TEXT,
			project_name TEXT, project_tag TEXT, build_id TEXT, description TEXT,
			status TEXT, multicluster_status TEXT, operator TEXT, other_info TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE product.release_plans (
			id TEXT PRIMARY KEY, product_id TEXT, product_plan_id TEXT, sprint_id TEXT, name TEXT,
			description TEXT, manager_id TEXT, approver_id TEXT, status TEXT,
			release_window_start DATETIME, release_window_end DATETIME,
			schedule_execute_time DATETIME, approval_config TEXT, hook_settings TEXT, hook_state TEXT, parameters TEXT DEFAULT '{}',
			version INTEGER DEFAULT 1, created_by TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE product.release_plan_jobs (
			id TEXT PRIMARY KEY, release_plan_id TEXT, sort_order INTEGER, name TEXT,
			type TEXT, owner_id TEXT, status TEXT, text_content TEXT, text_format TEXT, text_checked INTEGER,
			workflow_ref TEXT, execution_ref TEXT, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE product.release_plan_audit_logs (
			id TEXT PRIMARY KEY, release_plan_id TEXT, actor_id TEXT, action TEXT,
			detail TEXT, created_at DATETIME)`,
		`CREATE TABLE product.environments (
			id TEXT PRIMARY KEY, product_id TEXT, workspace_id TEXT, name TEXT,
			env_type TEXT, cluster_id TEXT, namespace TEXT, variables TEXT, status TEXT,
			created_by TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE product.environment_projects (
			environment_id TEXT, project_id TEXT, enabled INTEGER, current_image_tag TEXT,
			current_deployment_run_id TEXT, last_deployed_at DATETIME,
			PRIMARY KEY (environment_id, project_id))`,
		`CREATE TABLE product.workflow_templates (
			id TEXT PRIMARY KEY, scope TEXT, workspace_id TEXT, name TEXT, "key" TEXT,
			description TEXT, category TEXT, product_types TEXT, status TEXT, version TEXT,
			definition TEXT, slot_schema TEXT, key_vals TEXT, runtime TEXT, created_by TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE product.product_workflows (
			id TEXT PRIMARY KEY, product_id TEXT, workspace_id TEXT, name TEXT, "key" TEXT,
			description TEXT, status TEXT, stages TEXT, template_binding TEXT, origin TEXT,
			created_by TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE product.product_workflow_runs (
			id TEXT PRIMARY KEY, workflow_id TEXT, product_id TEXT, status TEXT,
			trigger TEXT, input TEXT, stage_runs TEXT, current_stage INTEGER,
			error_message TEXT, started_at DATETIME, completed_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE product.workflow_build_refs (
			build_id TEXT PRIMARY KEY, run_id TEXT, job_name TEXT, project_id TEXT,
			created_at DATETIME)`,
		`CREATE TABLE product.workflow_deploy_refs (
			deployment_run_id TEXT PRIMARY KEY, run_id TEXT, job_name TEXT, project_id TEXT,
			created_at DATETIME)`,
	}
	for _, stmt := range stmts {
		require.NoError(t, db.Exec(stmt).Error)
	}
}

func openProductV2TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS product").Error)
	createProductV2TestTables(t, db)
	require.NoError(t, pwfcommand.SyncBuiltinTemplates(context.Background(), db))
	return db
}

func TestProductV2ReleasePlanTextPipeline(t *testing.T) {
	ctx := context.Background()
	db := openProductV2TestDB(t)
	productRepo := persistence.NewProductRepository(db)
	releasePlanRepo := persistence.NewReleasePlanRepository(db)
	sprintRepo := persistence.NewSprintRepository(db)
	executor := service.NewReleasePlanExecutor(releasePlanRepo, nil, nil)

	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	name, err := model.NewProductName("integration-product")
	require.NoError(t, err)
	product := model.NewProduct(wsID, name, "集成测试", "", "1.0.0", "1.1.0", memberID)
	require.NoError(t, productRepo.SaveProduct(ctx, product))

	now := time.Now().UTC()
	createPlan := command.NewCreateReleasePlanHandler(productRepo, releasePlanRepo, sprintRepo)
	addJob := command.NewAddReleasePlanJobHandler(productRepo, releasePlanRepo)
	finish := command.NewFinishReleasePlanPlanningHandler(productRepo, releasePlanRepo, executor, nil, nil, nil)
	markDone := command.NewMarkReleasePlanJobDoneHandler(productRepo, releasePlanRepo, executor)

	plan, err := createPlan.Handle(ctx, product.ID, memberID, command.CreateReleasePlanInput{
		Name: "v2.3 集成发布", ReleaseWindowStart: now, ReleaseWindowEnd: now.Add(24 * time.Hour),
	})
	require.NoError(t, err)

	job, err := addJob.Handle(ctx, product.ID, plan.ID, memberID, command.AddReleasePlanJobInput{
		Name: "发布检查", Type: model.ReleasePlanJobTypeText, TextContent: "检查清单", SortOrder: 0,
	})
	require.NoError(t, err)

	plan, err = finish.Handle(ctx, product.ID, plan.ID, memberID)
	require.NoError(t, err)
	require.Equal(t, model.ReleasePlanStatusExecuting, plan.Status)

	plan, err = markDone.Handle(ctx, product.ID, plan.ID, job.ID, memberID, "")
	require.NoError(t, err)
	require.Equal(t, model.ReleasePlanStatusSuccess, plan.Status)
}

func TestCreateRequirementAdapterCreatesReleasePlan(t *testing.T) {
	ctx := context.Background()
	db := openProductV2TestDB(t)
	productRepo := persistence.NewProductRepository(db)
	releasePlanRepo := persistence.NewReleasePlanRepository(db)

	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	name, err := model.NewProductName("req-adapter-product")
	require.NoError(t, err)
	product := model.NewProduct(wsID, name, "适配层测试", "", "1.0.0", "", memberID)
	require.NoError(t, productRepo.SaveProduct(ctx, product))

	plan := model.NewProductPlan(product.ID, "plan-a", "1.0", "desc", nil)
	require.NoError(t, productRepo.SavePlan(ctx, plan))

	createReq := command.NewCreateProductRequirementHandler(productRepo, releasePlanRepo)
	req, err := createReq.Handle(ctx, product.ID, plan.ID, kernel.NewProjectID(), "svc-a", "main", "发布说明", "ops", nil)
	require.NoError(t, err)
	require.NotEmpty(t, req.OtherInfo["release_plan_id"])

	planID, err := kernel.ParseReleasePlanID(req.OtherInfo["release_plan_id"].(string))
	require.NoError(t, err)
	rp, err := releasePlanRepo.FindPlanByID(ctx, planID)
	require.NoError(t, err)
	require.Equal(t, product.ID, rp.ProductID)
	require.Len(t, rp.Jobs, 1)
	require.Equal(t, model.ReleasePlanJobTypeText, rp.Jobs[0].Type)
}

func TestProductWorkflowFromBuiltinTemplate(t *testing.T) {
	ctx := context.Background()
	db := openProductV2TestDB(t)
	productRepo := persistence.NewProductRepository(db)
	templateRepo := pwfpersistence.NewWorkflowTemplateRepository(db)
	workflowRepo := pwfpersistence.NewProductWorkflowRepository(db)

	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	name, err := model.NewProductName("workflow-product")
	require.NoError(t, err)
	product := model.NewProduct(wsID, name, "工作流测试", "", "1.0.0", "", memberID)
	require.NoError(t, productRepo.SaveProduct(ctx, product))

	templates, err := templateRepo.ListForWorkspace(ctx, wsID)
	require.NoError(t, err)
	require.NotEmpty(t, templates)

	var stdTpl *pwfmodel.WorkflowTemplate
	for _, tpl := range templates {
		if tpl.Key == "std-build-deploy-uat" {
			stdTpl = tpl
			break
		}
	}
	require.NotNil(t, stdTpl)

	checker := stubProductWorkflowChecker{wsID: wsID, productID: product.ID}
	createWF := pwfcommand.NewCreateWorkflowFromTemplateHandler(templateRepo, workflowRepo, checker)
	wf, err := createWF.Handle(ctx, wsID, product.ID, memberID, pwfcommand.CreateWorkflowFromTemplateInput{
		TemplateID: stdTpl.ID,
		ProjectIDs: []kernel.ProjectID{kernel.NewProjectID()},
		KeyVals:    map[string]string{"build_branch": "main", "uat_env_id": kernel.NewEnvironmentID().String()},
	})
	require.NoError(t, err)
	require.NotEmpty(t, wf.Stages)
	require.Equal(t, product.ID, wf.ProductID)
}

// compile-time interface checks
var (
	_ port.WorkspaceChecker  = stubWorkspaceChecker{}
	_ pwfport.ProductChecker = stubProductWorkflowChecker{}
)
