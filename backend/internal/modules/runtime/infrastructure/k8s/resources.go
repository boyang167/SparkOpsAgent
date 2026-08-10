package k8s

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	platformk8s "github.com/sparkops/sparkops-agent/backend/internal/platform/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

var virtualServiceGVR = schema.GroupVersionResource{
	Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices",
}

type ResourceService struct {
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
	istioEnabled  bool
	checker       port.ProjectChecker
}

func NewResourceService(
	cfg *config.Config,
	factory *platformk8s.Factory,
	cluster string,
	checker port.ProjectChecker,
) (*ResourceService, error) {
	client, err := factory.Get(cluster)
	if err != nil {
		return nil, err
	}
	svc := &ResourceService{client: client, checker: checker, istioEnabled: cfg.Runtime.IstioEnabled}
	if cfg.Runtime.IstioEnabled {
		dyn, err := factory.Dynamic(cluster)
		if err == nil {
			svc.dynamicClient = dyn
		}
	}
	return svc, nil
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
	for _, d := range list.Items {
		out = append(out, deploymentView(d))
	}
	return out, nil
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
	ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	dep, err := s.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, domain.ErrResourceNotFound
	}
	return deploymentView(*dep), nil
}

func (s *ResourceService) ScaleDeployment(ctx context.Context, projectID kernel.ProjectID, projectName, name string, replicas int32) error {
	if s.client == nil {
		return nil
	}
	ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
	if err != nil {
		return err
	}
	dep, err := s.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	dep.Spec.Replicas = &replicas
	_, err = s.client.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (s *ResourceService) ListIngresses(ctx context.Context, projectID kernel.ProjectID, projectName string) ([]map[string]any, error) {
	if s.client == nil {
		return []map[string]any{{"name": projectName + "-ingress", "host": projectName + ".example.com"}}, nil
	}
	ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	list, err := s.client.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list.Items))
	for _, ing := range list.Items {
		host := ""
		if len(ing.Spec.Rules) > 0 {
			host = ing.Spec.Rules[0].Host
		}
		out = append(out, map[string]any{"name": ing.Name, "host": host, "namespace": ing.Namespace})
	}
	return out, nil
}

func (s *ResourceService) GetIngress(ctx context.Context, projectID kernel.ProjectID, projectName, name string) (map[string]any, error) {
	if s.client == nil {
		if name == projectName+"-ingress" {
			return map[string]any{"name": name, "host": projectName + ".example.com"}, nil
		}
		return nil, domain.ErrResourceNotFound
	}
	ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	ing, err := s.client.NetworkingV1().Ingresses(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, domain.ErrResourceNotFound
	}
	host := ""
	if len(ing.Spec.Rules) > 0 {
		host = ing.Spec.Rules[0].Host
	}
	return map[string]any{"name": ing.Name, "host": host, "namespace": ing.Namespace}, nil
}

func (s *ResourceService) ListConfigMaps(ctx context.Context, projectID kernel.ProjectID, projectName string) ([]map[string]any, error) {
	if s.client == nil {
		return []map[string]any{{"name": projectName + "-config", "keys": 3}}, nil
	}
	ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	list, err := s.client.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list.Items))
	for _, cm := range list.Items {
		out = append(out, map[string]any{"name": cm.Name, "keys": len(cm.Data), "namespace": cm.Namespace})
	}
	return out, nil
}

func (s *ResourceService) GetConfigMap(ctx context.Context, projectID kernel.ProjectID, projectName, name string) (map[string]any, error) {
	if s.client == nil {
		if name == projectName+"-config" {
			return map[string]any{"name": name, "keys": 3}, nil
		}
		return nil, domain.ErrResourceNotFound
	}
	ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	cm, err := s.client.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, domain.ErrResourceNotFound
	}
	return map[string]any{"name": cm.Name, "keys": len(cm.Data), "namespace": cm.Namespace}, nil
}

func (s *ResourceService) ListServices(ctx context.Context, projectID kernel.ProjectID, projectName string) ([]map[string]any, error) {
	if s.client == nil {
		return []map[string]any{{"name": projectName + "-svc", "type": "ClusterIP"}}, nil
	}
	ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	list, err := s.client.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list.Items))
	for _, svc := range list.Items {
		out = append(out, map[string]any{"name": svc.Name, "type": string(svc.Spec.Type), "namespace": svc.Namespace})
	}
	return out, nil
}

func (s *ResourceService) GetService(ctx context.Context, projectID kernel.ProjectID, projectName, name string) (map[string]any, error) {
	if s.client == nil {
		if name == projectName+"-svc" {
			return map[string]any{"name": name, "type": "ClusterIP"}, nil
		}
		return nil, domain.ErrResourceNotFound
	}
	ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	svc, err := s.client.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, domain.ErrResourceNotFound
	}
	return map[string]any{"name": svc.Name, "type": string(svc.Spec.Type), "namespace": svc.Namespace}, nil
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

func (s *ResourceService) ListPVCs(ctx context.Context, projectID kernel.ProjectID, projectName string) ([]map[string]any, error) {
	if s.client == nil {
		return []map[string]any{{"name": projectName + "-data", "status": "Bound", "size": "10Gi"}}, nil
	}
	ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	list, err := s.client.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list.Items))
	for _, pvc := range list.Items {
		size := ""
		if req, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			size = req.String()
		}
		out = append(out, map[string]any{
			"name": pvc.Name, "status": string(pvc.Status.Phase), "size": size, "namespace": pvc.Namespace,
		})
	}
	return out, nil
}

func (s *ResourceService) GetPVC(ctx context.Context, projectID kernel.ProjectID, projectName, name string) (map[string]any, error) {
	if s.client == nil {
		if name == projectName+"-data" {
			return map[string]any{"name": name, "status": "Bound", "size": "10Gi"}, nil
		}
		return nil, domain.ErrResourceNotFound
	}
	ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
	if err != nil {
		return nil, err
	}
	pvc, err := s.client.CoreV1().PersistentVolumeClaims(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, domain.ErrResourceNotFound
	}
	size := ""
	if req, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		size = req.String()
	}
	return map[string]any{
		"name": pvc.Name, "status": string(pvc.Status.Phase), "size": size, "namespace": pvc.Namespace,
	}, nil
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

func (s *ResourceService) ListVirtualServices(ctx context.Context, projectID kernel.ProjectID, projectName string) ([]map[string]any, error) {
	if s.dynamicClient != nil && s.istioEnabled {
		ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
		if err != nil {
			return nil, err
		}
		list, err := s.dynamicClient.Resource(virtualServiceGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return mockVirtualServices(projectName), nil
		}
		out := make([]map[string]any, 0, len(list.Items))
		for _, item := range list.Items {
			out = append(out, virtualServiceView(item.Object))
		}
		if len(out) == 0 {
			return mockVirtualServices(projectName), nil
		}
		return out, nil
	}
	if s.client == nil {
		return mockVirtualServices(projectName), nil
	}
	return mockVirtualServices(projectName), nil
}

func (s *ResourceService) GetVirtualService(ctx context.Context, projectID kernel.ProjectID, projectName, name string) (map[string]any, error) {
	if s.dynamicClient != nil && s.istioEnabled {
		ns, err := s.checker.ProjectNamespace(ctx, projectID, projectName)
		if err != nil {
			return nil, err
		}
		item, err := s.dynamicClient.Resource(virtualServiceGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, domain.ErrResourceNotFound
		}
		return virtualServiceView(item.Object), nil
	}
	for _, item := range mockVirtualServices(projectName) {
		if item["name"] == name {
			return item, nil
		}
	}
	return nil, domain.ErrResourceNotFound
}

func virtualServiceView(obj map[string]any) map[string]any {
	meta, _ := obj["metadata"].(map[string]any)
	spec, _ := obj["spec"].(map[string]any)
	name, _ := meta["name"].(string)
	namespace, _ := meta["namespace"].(string)
	hosts, _ := spec["hosts"].([]any)
	gateways, _ := spec["gateways"].([]any)
	return map[string]any{
		"name": name, "namespace": namespace, "hosts": hosts, "gateways": gateways, "spec": spec,
	}
}

func mockVirtualServices(projectName string) []map[string]any {
	ns := DefaultNamespace(projectName)
	return []map[string]any{{
		"name": projectName + "-vs", "namespace": ns,
		"hosts": []string{projectName + ".sparkops.local"},
		"gateways": []string{"sparkops-gateway"},
	}}
}
