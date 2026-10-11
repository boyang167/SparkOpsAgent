package k8s

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	platformk8s "github.com/sparkops/sparkops-agent/backend/internal/platform/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ResourceService struct {
	client  kubernetes.Interface
	checker port.ProjectChecker
}

func NewResourceService(
	_ *config.Config,
	factory *platformk8s.Factory,
	cluster string,
	checker port.ProjectChecker,
) (*ResourceService, error) {
	client, err := factory.Get(cluster)
	if err != nil {
		return nil, err
	}
	return &ResourceService{client: client, checker: checker}, nil
}

func NewMockResourceService(checker port.ProjectChecker) *ResourceService {
	return &ResourceService{client: nil, checker: checker}
}

func (s *ResourceService) ListDeployments(ctx context.Context, projectID kernel.ProjectID, projectName string) ([]map[string]any, error) {
	if s.client == nil {
		return mockDeployments(projectName), nil
	}
	ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	list, err := s.client.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list.Items))
	for i := range list.Items {
		if deploymentBelongsToProject(&list.Items[i], projectName) {
			out = append(out, deploymentView(list.Items[i]))
		}
	}
	return out, nil
}

// Projects may share a namespace, so a deployment is only treated as the
// project's own when its app label or conventional name matches.
func deploymentBelongsToProject(dep *appsv1.Deployment, projectName string) bool {
	if dep == nil || projectName == "" {
		return false
	}
	if dep.Labels[sparkopsAppLabel] == projectName {
		return true
	}
	return dep.Name == projectName || dep.Name == projectName+"-deployment"
}

func (s *ResourceService) projectDeployment(
	ctx context.Context,
	projectID kernel.ProjectID,
	projectName, name string,
) (string, *appsv1.Deployment, error) {
	ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
	if err != nil {
		return "", nil, err
	}
	dep, err := s.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil || !deploymentBelongsToProject(dep, projectName) {
		return "", nil, domain.ErrResourceNotFound
	}
	return ns, dep, nil
}

func (s *ResourceService) GetDeployment(ctx context.Context, projectID kernel.ProjectID, projectName, name string) (map[string]any, error) {
	if s.client == nil {
		for _, item := range mockDeployments(projectName) {
			if item["name"] == name {
				return item, nil
			}
		}
		return nil, domain.ErrResourceNotFound
	}
	_, dep, err := s.projectDeployment(ctx, projectID, projectName, name)
	if err != nil {
		return nil, err
	}
	return deploymentView(*dep), nil
}

func (s *ResourceService) ScaleDeployment(ctx context.Context, projectID kernel.ProjectID, projectName, name string, replicas int32) error {
	if s.client == nil {
		return nil
	}
	ns, dep, err := s.projectDeployment(ctx, projectID, projectName, name)
	if err != nil {
		return err
	}
	dep.Spec.Replicas = &replicas
	_, err = s.client.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (s *ResourceService) ListStorageClasses(ctx context.Context) ([]map[string]any, error) {
	if s.client == nil {
		return []map[string]any{{"name": "standard", "provisioner": "kubernetes.io/mock"}}, nil
	}
	list, err := s.client.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list.Items))
	for _, sc := range list.Items {
		out = append(out, map[string]any{"name": sc.Name, "provisioner": sc.Provisioner})
	}
	return out, nil
}

func (s *ResourceService) GetStorageClass(ctx context.Context, name string) (map[string]any, error) {
	if s.client == nil {
		if name == "standard" {
			return map[string]any{"name": "standard", "provisioner": "kubernetes.io/mock"}, nil
		}
		return nil, domain.ErrResourceNotFound
	}
	sc, err := s.client.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, domain.ErrResourceNotFound
	}
	return map[string]any{"name": sc.Name, "provisioner": sc.Provisioner}, nil
}

func deploymentView(d appsv1.Deployment) map[string]any {
	replicas := int32(0)
	if d.Spec.Replicas != nil {
		replicas = *d.Spec.Replicas
	}
	return map[string]any{
		"name": d.Name, "namespace": d.Namespace,
		"replicas": replicas, "ready": d.Status.ReadyReplicas,
	}
}

func mockDeployments(projectName string) []map[string]any {
	return []map[string]any{{
		"name": projectName + "-api", "namespace": DefaultNamespace(projectName),
		"replicas": 2, "ready": 2,
	}}
}
