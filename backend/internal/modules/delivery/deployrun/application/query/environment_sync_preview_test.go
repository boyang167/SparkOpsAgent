package query

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fakeEnvironmentResolver struct {
	targets map[kernel.EnvironmentID]deployport.EnvironmentDeployTarget
}

func (f fakeEnvironmentResolver) Resolve(
	_ context.Context,
	_ kernel.WorkspaceID,
	_ kernel.ProjectID,
	envID kernel.EnvironmentID,
) (deployport.EnvironmentDeployTarget, error) {
	target, ok := f.targets[envID]
	if !ok {
		return deployport.EnvironmentDeployTarget{}, domain.ErrDeploymentEnvironmentNotFound
	}
	return target, nil
}

type fakePreviewReader struct {
	project deployport.ProjectDeploymentContext
}

func (f fakePreviewReader) Load(
	_ context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	clusterID kernel.ClusterInfoID,
) (deployport.ProjectDeploymentContext, error) {
	out := f.project
	out.WorkspaceID = wsID
	out.ProjectID = projectID
	if clusterID != (kernel.ClusterInfoID{}) {
		out.Cluster.ID = clusterID
	}
	return out, nil
}

func (fakePreviewReader) ListAuthorizedClusters(context.Context, kernel.WorkspaceID) ([]deployport.DeploymentCluster, error) {
	return nil, nil
}

type memorySnapshotRepo struct {
	created []*model.EnvironmentSyncSnapshot
}

func (r *memorySnapshotRepo) Create(_ context.Context, snapshot *model.EnvironmentSyncSnapshot) error {
	r.created = append(r.created, snapshot)
	return nil
}

func (*memorySnapshotRepo) FindByID(context.Context, kernel.EnvironmentSyncSnapshotID) (*model.EnvironmentSyncSnapshot, error) {
	return nil, domain.ErrEnvironmentSyncSnapshotNotFound
}

func (*memorySnapshotRepo) BindToRun(context.Context, kernel.EnvironmentSyncSnapshotID, kernel.ProductWorkflowRunID) error {
	return nil
}

func newPreviewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS product").Error)
	require.NoError(t, db.Exec(`CREATE TABLE product.environments (
		id TEXT PRIMARY KEY, product_id TEXT, workspace_id TEXT, status TEXT, deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE product.environment_projects (
		environment_id TEXT, project_id TEXT)`).Error)
	return db
}

func seedEnvironment(t *testing.T, db *gorm.DB, envID, productID, wsID string) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO product.environments (id, product_id, workspace_id, status) VALUES (?, ?, ?, 'active')`,
		envID, productID, wsID,
	).Error)
}

func deploymentYAML(name, image string) string {
	return "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: " + name + "\nspec:\n  replicas: 2\n  template:\n    spec:\n      containers:\n        - name: app\n          image: " + image + "\n"
}

func secretYAML(name string) string {
	return "apiVersion: v1\nkind: Secret\nmetadata:\n  name: " + name + "\ndata:\n  token: c2VjcmV0\n"
}

func TestCreateEnvironmentSyncPreviewNormalizesFirstSync(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	productID := kernel.NewProductID()
	sourceID := kernel.NewEnvironmentID()
	targetID := kernel.NewEnvironmentID()
	projectID := kernel.NewProjectID()
	db := newPreviewTestDB(t)
	seedEnvironment(t, db, sourceID.String(), productID.String(), wsID.String())
	seedEnvironment(t, db, targetID.String(), productID.String(), wsID.String())

	repo := &memorySnapshotRepo{}
	handler := NewCreateEnvironmentSyncPreviewHandler(
		fakePreviewReader{project: deployport.ProjectDeploymentContext{Name: "demo"}},
		fakeEnvironmentResolver{targets: map[kernel.EnvironmentID]deployport.EnvironmentDeployTarget{
			sourceID: {EnvironmentID: sourceID, ProductID: productID, ClusterID: kernel.NewClusterInfoID(), Namespace: "source-ns"},
			targetID: {EnvironmentID: targetID, ProductID: productID, ClusterID: kernel.NewClusterInfoID(), Namespace: "target-ns"},
		}},
		repo,
		db,
	)
	handler.capture = func(_ context.Context, project deployport.ProjectDeploymentContext) ([]model.EnvironmentSyncResource, error) {
		if project.Namespace == "source-ns" {
			return []model.EnvironmentSyncResource{{
				Kind: model.SyncManifestKindDeployment, Name: "demo",
				YAML: deploymentYAML("demo", "registry/source/demo:v2"),
			}}, nil
		}
		return nil, nil
	}

	preview, err := handler.Handle(context.Background(), wsID, productID, kernel.NewMemberID(), CreateEnvironmentSyncPreviewInput{
		WorkflowID:          kernel.NewProductWorkflowID(),
		JobName:             "promote",
		SourceEnvironmentID: sourceID,
		TargetEnvironmentID: targetID,
		Projects: []model.EnvironmentSyncProjectOption{{
			ProjectID: projectID, SyncImage: true, SyncYAML: false,
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, preview)
	require.Len(t, preview.Projects, 1)
	require.True(t, preview.Projects[0].Option.FirstSync)
	require.True(t, preview.Projects[0].Option.SyncYAML)
	require.Len(t, repo.created, 1)
}

func TestCreateEnvironmentSyncPreviewRejectsSameEnvironment(t *testing.T) {
	envID := kernel.NewEnvironmentID()
	handler := NewCreateEnvironmentSyncPreviewHandler(nil, nil, &memorySnapshotRepo{}, nil)
	_, err := handler.Handle(context.Background(), kernel.NewWorkspaceID(), kernel.NewProductID(), kernel.NewMemberID(), CreateEnvironmentSyncPreviewInput{
		WorkflowID:          kernel.NewProductWorkflowID(),
		JobName:             "promote",
		SourceEnvironmentID: envID,
		TargetEnvironmentID: envID,
		Projects: []model.EnvironmentSyncProjectOption{{
			ProjectID: kernel.NewProjectID(), SyncImage: true,
		}},
	})
	require.ErrorIs(t, err, domain.ErrEnvironmentSyncSameEnvironment)
}

func TestCreateEnvironmentSyncPreviewDoesNotExposeSecretData(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	productID := kernel.NewProductID()
	sourceID := kernel.NewEnvironmentID()
	targetID := kernel.NewEnvironmentID()
	projectID := kernel.NewProjectID()
	db := newPreviewTestDB(t)
	seedEnvironment(t, db, sourceID.String(), productID.String(), wsID.String())
	seedEnvironment(t, db, targetID.String(), productID.String(), wsID.String())
	require.NoError(t, db.Exec(
		`INSERT INTO product.environment_projects (environment_id, project_id) VALUES (?, ?)`,
		targetID.String(), projectID.String(),
	).Error)

	handler := NewCreateEnvironmentSyncPreviewHandler(
		fakePreviewReader{project: deployport.ProjectDeploymentContext{Name: "demo"}},
		fakeEnvironmentResolver{targets: map[kernel.EnvironmentID]deployport.EnvironmentDeployTarget{
			sourceID: {EnvironmentID: sourceID, ProductID: productID, ClusterID: kernel.NewClusterInfoID(), Namespace: "source-ns"},
			targetID: {EnvironmentID: targetID, ProductID: productID, ClusterID: kernel.NewClusterInfoID(), Namespace: "target-ns"},
		}},
		&memorySnapshotRepo{},
		db,
	)
	handler.capture = func(_ context.Context, project deployport.ProjectDeploymentContext) ([]model.EnvironmentSyncResource, error) {
		if project.Namespace == "source-ns" {
			return []model.EnvironmentSyncResource{
				{Kind: model.SyncManifestKindDeployment, Name: "demo", YAML: deploymentYAML("demo", "registry/source/demo:v2")},
				{Kind: model.SyncManifestKindSecret, Name: "app-secret", YAML: secretYAML("app-secret")},
			}, nil
		}
		return []model.EnvironmentSyncResource{
			{Kind: model.SyncManifestKindDeployment, Name: "demo", YAML: deploymentYAML("demo", "registry/target/demo:v1")},
		}, nil
	}

	preview, err := handler.Handle(context.Background(), wsID, productID, kernel.NewMemberID(), CreateEnvironmentSyncPreviewInput{
		WorkflowID:          kernel.NewProductWorkflowID(),
		JobName:             "promote",
		SourceEnvironmentID: sourceID,
		TargetEnvironmentID: targetID,
		Projects: []model.EnvironmentSyncProjectOption{{
			ProjectID: projectID, SyncImage: false, SyncYAML: true,
			SyncResourceKinds: []string{model.SyncManifestKindSecret},
		}},
	})
	require.NoError(t, err)
	view := ToEnvironmentSyncPreviewView(preview)
	foundSecret := false
	for _, project := range view.Projects {
		for _, diff := range project.Diffs {
			if diff.Kind == model.SyncManifestKindSecret {
				foundSecret = true
				require.NotContains(t, diff.After, "\ndata:")
				require.NotContains(t, diff.After, "stringData:")
				require.NotContains(t, diff.Before, "\ndata:")
				require.Contains(t, diff.Note, "secret data is not copied")
			}
		}
	}
	require.True(t, foundSecret)
}
