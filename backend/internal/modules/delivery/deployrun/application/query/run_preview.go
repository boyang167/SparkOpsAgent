package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	runtimek8s "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type RunPreviewItemView struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	RenderedYAML string `json:"rendered_yaml"`
	CurrentYAML  string `json:"current_yaml"`
	CurrentFound bool   `json:"current_found"`
}

type RunPreviewView struct {
	EnvironmentID string               `json:"environment_id"`
	ClusterID     string               `json:"cluster_id"`
	ClusterName   string               `json:"cluster_name"`
	Namespace     string               `json:"namespace"`
	Items         []RunPreviewItemView `json:"items"`
}

type RunPreviewHandler struct {
	reader       deployport.ProjectDeploymentReader
	environments deployport.EnvironmentResolver
}

func NewRunPreviewHandler(
	reader deployport.ProjectDeploymentReader,
	environments deployport.EnvironmentResolver,
) *RunPreviewHandler {
	return &RunPreviewHandler{reader: reader, environments: environments}
}

func (h *RunPreviewHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	environmentID string,
	imageTag string,
) (RunPreviewView, error) {
	envID, err := kernel.ParseEnvironmentID(strings.TrimSpace(environmentID))
	if err != nil {
		return RunPreviewView{}, fmt.Errorf("invalid environment_id")
	}
	target, err := h.environments.Resolve(ctx, wsID, projectID, envID)
	if err != nil {
		return RunPreviewView{}, err
	}
	project, err := h.reader.Load(ctx, wsID, projectID, target.ClusterID)
	if err != nil {
		return RunPreviewView{}, err
	}
	if ns := strings.TrimSpace(target.Namespace); ns != "" {
		project.Namespace = ns
	}

	client, err := runtimek8s.NewTypedClientFromKubeconfig(
		[]byte(project.Kubernetes.Credentials.ExtraConfig),
		project.Cluster.Server,
	)
	if err != nil {
		return RunPreviewView{}, fmt.Errorf("open target cluster: %w", err)
	}

	items, err := buildRunPreviewItems(ctx, client, project, strings.TrimSpace(imageTag))
	if err != nil {
		return RunPreviewView{}, err
	}
	return RunPreviewView{
		EnvironmentID: envID.String(),
		ClusterID:     target.ClusterID.String(),
		ClusterName:   project.Cluster.Name,
		Namespace:     project.Namespace,
		Items:         items,
	}, nil
}

func buildRunPreviewItems(
	ctx context.Context,
	client kubernetes.Interface,
	project deployport.ProjectDeploymentContext,
	imageTag string,
) ([]RunPreviewItemView, error) {
	ns := project.Namespace
	projectName := project.Name
	items := make([]RunPreviewItemView, 0, 2)

	dep, currentDepYAML, err := loadLiveDeployment(ctx, client, ns, projectName)
	if err != nil {
		return nil, err
	}
	_, renderedDepYAML, err := loadPreviewDeployment(
		ctx, client, ns, projectName, project.DeploymentYAML, project.ImageRepository, imageTag,
	)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(renderedDepYAML) != "" {
		name := projectName
		if dep != nil {
			name = dep.Name
		}
		items = append(items, RunPreviewItemView{
			Kind:         model.SyncManifestKindDeployment,
			Name:         name,
			RenderedYAML: renderedDepYAML,
			CurrentYAML:  currentDepYAML,
			CurrentFound: strings.TrimSpace(currentDepYAML) != "",
		})
	}

	svc, currentSvcYAML, err := loadLiveService(ctx, client, ns, projectName)
	if err != nil {
		return nil, err
	}
	_, renderedSvcYAML, err := loadPreviewService(ctx, client, ns, projectName, project.ServiceYAML)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(renderedSvcYAML) != "" {
		name := projectName
		if svc != nil {
			name = svc.Name
		}
		items = append(items, RunPreviewItemView{
			Kind:         model.SyncManifestKindService,
			Name:         name,
			RenderedYAML: renderedSvcYAML,
			CurrentYAML:  currentSvcYAML,
			CurrentFound: strings.TrimSpace(currentSvcYAML) != "",
		})
	}
	return items, nil
}

func loadLiveDeployment(
	ctx context.Context,
	client kubernetes.Interface,
	ns, projectName string,
) (*appsv1.Deployment, string, error) {
	if client == nil {
		return nil, "", nil
	}
	for _, name := range []string{projectName, projectName + "-deployment"} {
		dep, err := client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err == nil && dep != nil {
			yamlDoc, err := runtimek8s.MarshalK8sYAML(dep, "apps/v1", model.SyncManifestKindDeployment)
			if err != nil {
				return nil, "", err
			}
			return dep, yamlDoc, nil
		}
	}
	return nil, "", nil
}

func loadLiveService(
	ctx context.Context,
	client kubernetes.Interface,
	ns, projectName string,
) (*corev1.Service, string, error) {
	if client == nil {
		return nil, "", nil
	}
	for _, name := range []string{projectName, projectName + "-service"} {
		svc, err := client.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
		if err == nil && svc != nil {
			yamlDoc, err := runtimek8s.MarshalK8sYAML(svc, "v1", model.SyncManifestKindService)
			if err != nil {
				return nil, "", err
			}
			return svc, yamlDoc, nil
		}
	}
	return nil, "", nil
}
