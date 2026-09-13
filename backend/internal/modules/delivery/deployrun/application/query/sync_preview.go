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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"gopkg.in/yaml.v3"
)

type SyncPreviewItemView struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	RelatedReason   string `json:"related_reason,omitempty"`
	DefaultSelected bool   `json:"default_selected"`
	Managed         bool   `json:"managed,omitempty"`
	YAML            string `json:"yaml,omitempty"`
	Note            string `json:"note,omitempty"`
}

type SyncPreviewView struct {
	BuildClusterID   string                `json:"build_cluster_id"`
	BuildClusterName string                `json:"build_cluster_name"`
	Namespace        string                `json:"namespace"`
	Items            []SyncPreviewItemView `json:"items"`
}

type SyncPreviewHandler struct {
	reader deployport.ProjectDeploymentReader
}

func NewSyncPreviewHandler(reader deployport.ProjectDeploymentReader) *SyncPreviewHandler {
	return &SyncPreviewHandler{reader: reader}
}

func (h *SyncPreviewHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	imageTag string,
) (SyncPreviewView, error) {
	project, err := h.reader.Load(ctx, wsID, projectID, kernel.ClusterInfoID{})
	if err != nil {
		return SyncPreviewView{}, err
	}
	client, err := runtimek8s.NewTypedClientFromKubeconfig(
		[]byte(project.Kubernetes.Credentials.ExtraConfig),
		project.Cluster.Server,
	)
	if err != nil {
		return SyncPreviewView{}, fmt.Errorf("open build cluster: %w", err)
	}
	items, err := buildSyncPreviewItems(ctx, client, project, strings.TrimSpace(imageTag))
	if err != nil {
		return SyncPreviewView{}, err
	}
	return SyncPreviewView{
		BuildClusterID:   project.Cluster.ID.String(),
		BuildClusterName: project.Cluster.Name,
		Namespace:        project.Namespace,
		Items:            items,
	}, nil
}

func buildSyncPreviewItems(
	ctx context.Context,
	client kubernetes.Interface,
	project deployport.ProjectDeploymentContext,
	imageTag string,
) ([]SyncPreviewItemView, error) {
	ns := project.Namespace
	projectName := project.Name
	items := make([]SyncPreviewItemView, 0, 8)

	dep, depYAML, err := loadPreviewDeployment(ctx, client, ns, projectName, project.DeploymentYAML, project.ImageRepository, imageTag)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(depYAML) != "" {
		name := projectName
		if dep != nil {
			name = dep.Name
		}
		items = append(items, SyncPreviewItemView{
			ID: itemID(model.SyncManifestKindDeployment, name), Kind: model.SyncManifestKindDeployment,
			Name: name, RelatedReason: "core workload", DefaultSelected: true, YAML: depYAML,
		})
	}

	svc, svcYAML, err := loadPreviewService(ctx, client, ns, projectName, project.ServiceYAML)
	if err != nil {
		return nil, err
	}
	serviceName := projectName
	if svc != nil {
		serviceName = svc.Name
	}
	if strings.TrimSpace(svcYAML) != "" {
		items = append(items, SyncPreviewItemView{
			ID: itemID(model.SyncManifestKindService, serviceName), Kind: model.SyncManifestKindService,
			Name: serviceName, RelatedReason: "core workload", DefaultSelected: true, YAML: svcYAML,
		})
	}

	referencedCM, referencedSecret := referencedFromDeployment(dep)
	for _, pull := range imagePullSecretNames(dep) {
		referencedSecret[pull] = "image pull secret"
		items = append(items, SyncPreviewItemView{
			ID: itemID(model.SyncManifestKindSecret, pull), Kind: model.SyncManifestKindSecret,
			Name: pull, RelatedReason: "image pull secret", DefaultSelected: true,
			Managed: true, Note: "目标集群将使用 Harbor 凭据 ensure，不会复制构建集群 dockerconfigjson",
		})
	}

	cms, err := client.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list configmaps: %w", err)
	}
	for i := range cms.Items {
		cm := &cms.Items[i]
		reason := relatedResourceReason(cm.Name, cm.Labels, projectName, referencedCM)
		if reason == "" {
			continue
		}
		yamlDoc, err := runtimek8s.MarshalK8sYAML(cm, "v1", model.SyncManifestKindConfigMap)
		if err != nil {
			return nil, err
		}
		items = append(items, SyncPreviewItemView{
			ID: itemID(model.SyncManifestKindConfigMap, cm.Name), Kind: model.SyncManifestKindConfigMap,
			Name: cm.Name, RelatedReason: reason, YAML: yamlDoc,
		})
	}

	secrets, err := client.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	managedPull := map[string]struct{}{}
	for _, pull := range imagePullSecretNames(dep) {
		managedPull[pull] = struct{}{}
	}
	for i := range secrets.Items {
		secret := &secrets.Items[i]
		if excludeSystemSecret(secret) {
			continue
		}
		if _, ok := managedPull[secret.Name]; ok {
			continue
		}
		reason := relatedResourceReason(secret.Name, secret.Labels, projectName, referencedSecret)
		if reason == "" {
			continue
		}
		yamlDoc, err := runtimek8s.MarshalK8sYAML(secret, "v1", model.SyncManifestKindSecret)
		if err != nil {
			return nil, err
		}
		items = append(items, SyncPreviewItemView{
			ID: itemID(model.SyncManifestKindSecret, secret.Name), Kind: model.SyncManifestKindSecret,
			Name: secret.Name, RelatedReason: reason, YAML: yamlDoc,
		})
	}

	ingresses, err := client.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list ingresses: %w", err)
	}
	for i := range ingresses.Items {
		ing := &ingresses.Items[i]
		reason := relatedIngressReason(ing, projectName, serviceName)
		if reason == "" {
			continue
		}
		yamlDoc, err := runtimek8s.MarshalK8sYAML(ing, "networking.k8s.io/v1", model.SyncManifestKindIngress)
		if err != nil {
			return nil, err
		}
		items = append(items, SyncPreviewItemView{
			ID: itemID(model.SyncManifestKindIngress, ing.Name), Kind: model.SyncManifestKindIngress,
			Name: ing.Name, RelatedReason: reason, YAML: yamlDoc,
		})
	}

	return dedupePreviewItems(items), nil
}

func itemID(kind, name string) string { return kind + "/" + name }

func loadPreviewDeployment(
	ctx context.Context,
	client kubernetes.Interface,
	ns, projectName, templateYAML, imageRepo, imageTag string,
) (*appsv1.Deployment, string, error) {
	if client != nil {
		for _, name := range []string{projectName, projectName + "-deployment"} {
			dep, err := client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
			if err == nil && dep != nil {
				yamlDoc, err := runtimek8s.MarshalK8sYAML(dep, "apps/v1", model.SyncManifestKindDeployment)
				if err != nil {
					return nil, "", err
				}
				return dep, rewriteContainerImageTags(yamlDoc, imageRepo, imageTag), nil
			}
		}
	}
	if strings.TrimSpace(templateYAML) == "" {
		return nil, "", nil
	}
	return nil, rewriteContainerImageTags(templateYAML, imageRepo, imageTag), nil
}

func loadPreviewService(
	ctx context.Context,
	client kubernetes.Interface,
	ns, projectName, templateYAML string,
) (*corev1.Service, string, error) {
	if client != nil {
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
	}
	if strings.TrimSpace(templateYAML) == "" {
		return nil, "", nil
	}
	return nil, templateYAML, nil
}

func referencedFromDeployment(dep *appsv1.Deployment) (map[string]string, map[string]string) {
	cms := map[string]string{}
	secrets := map[string]string{}
	if dep == nil {
		return cms, secrets
	}
	pod := dep.Spec.Template.Spec
	for _, vol := range pod.Volumes {
		if vol.ConfigMap != nil && vol.ConfigMap.Name != "" {
			cms[vol.ConfigMap.Name] = "referenced by Deployment volume"
		}
		if vol.Secret != nil && vol.Secret.SecretName != "" {
			secrets[vol.Secret.SecretName] = "referenced by Deployment volume"
		}
	}
	for _, c := range append(append([]corev1.Container{}, pod.Containers...), pod.InitContainers...) {
		for _, envFrom := range c.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name != "" {
				cms[envFrom.ConfigMapRef.Name] = "referenced by Deployment envFrom"
			}
			if envFrom.SecretRef != nil && envFrom.SecretRef.Name != "" {
				secrets[envFrom.SecretRef.Name] = "referenced by Deployment envFrom"
			}
		}
		for _, env := range c.Env {
			if env.ValueFrom == nil {
				continue
			}
			if env.ValueFrom.ConfigMapKeyRef != nil && env.ValueFrom.ConfigMapKeyRef.Name != "" {
				cms[env.ValueFrom.ConfigMapKeyRef.Name] = "referenced by Deployment env"
			}
			if env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.Name != "" {
				secrets[env.ValueFrom.SecretKeyRef.Name] = "referenced by Deployment env"
			}
		}
	}
	return cms, secrets
}

func imagePullSecretNames(dep *appsv1.Deployment) []string {
	if dep == nil {
		return nil
	}
	out := make([]string, 0, len(dep.Spec.Template.Spec.ImagePullSecrets))
	seen := map[string]struct{}{}
	for _, ref := range dep.Spec.Template.Spec.ImagePullSecrets {
		name := strings.TrimSpace(ref.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func relatedResourceReason(name string, labels map[string]string, projectName string, referenced map[string]string) string {
	if reason, ok := referenced[name]; ok {
		return reason
	}
	if projectName == "" {
		return ""
	}
	if name == projectName || strings.HasPrefix(name, projectName+"-") {
		return "name prefix"
	}
	if labels != nil {
		if labels["app"] == projectName || labels["app.kubernetes.io/name"] == projectName {
			return "label match"
		}
	}
	return ""
}

func relatedIngressReason(ing *networkingv1.Ingress, projectName, serviceName string) string {
	if ing == nil {
		return ""
	}
	if reason := relatedResourceReason(ing.Name, ing.Labels, projectName, nil); reason != "" {
		return reason
	}
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil && path.Backend.Service.Name == serviceName {
				return "ingress backend"
			}
		}
	}
	return ""
}

func excludeSystemSecret(secret *corev1.Secret) bool {
	if secret == nil {
		return true
	}
	if strings.HasPrefix(secret.Name, "default-token-") {
		return true
	}
	if secret.Type == corev1.SecretTypeServiceAccountToken {
		return true
	}
	if secret.Type == "helm.sh/release.v1" || strings.HasPrefix(secret.Name, "sh.helm.release.") {
		return true
	}
	return false
}

func rewriteContainerImageTags(raw, imageRepo, imageTag string) string {
	imageTag = strings.TrimSpace(imageTag)
	if imageTag == "" || strings.TrimSpace(raw) == "" {
		return raw
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return raw
	}
	containers, ok := nestedSlice(doc, "spec", "template", "spec", "containers")
	if !ok {
		return raw
	}
	repo := strings.TrimSuffix(strings.TrimSpace(imageRepo), "/")
	changed := false
	for i, item := range containers {
		container, ok := item.(map[string]any)
		if !ok {
			continue
		}
		image, _ := container["image"].(string)
		if image == "" {
			continue
		}
		if repo != "" && strings.HasPrefix(image, repo+":") {
			container["image"] = repo + ":" + imageTag
			containers[i] = container
			changed = true
			continue
		}
		if idx := strings.LastIndex(image, ":"); idx > 0 {
			container["image"] = image[:idx+1] + imageTag
			containers[i] = container
			changed = true
		}
	}
	if !changed {
		return raw
	}
	_ = setNestedSlice(doc, containers, "spec", "template", "spec", "containers")
	out, err := yaml.Marshal(doc)
	if err != nil {
		return raw
	}
	return string(out)
}

func nestedSlice(doc map[string]any, path ...string) ([]any, bool) {
	cur := any(doc)
	for _, key := range path[:len(path)-1] {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur = m[key]
	}
	m, ok := cur.(map[string]any)
	if !ok {
		return nil, false
	}
	slice, ok := m[path[len(path)-1]].([]any)
	return slice, ok
}

func setNestedSlice(doc map[string]any, value []any, path ...string) error {
	cur := any(doc)
	for _, key := range path[:len(path)-1] {
		m, ok := cur.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid path")
		}
		next, ok := m[key]
		if !ok {
			next = map[string]any{}
			m[key] = next
		}
		cur = next
	}
	m, ok := cur.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid path")
	}
	m[path[len(path)-1]] = value
	return nil
}

func dedupePreviewItems(items []SyncPreviewItemView) []SyncPreviewItemView {
	seen := map[string]struct{}{}
	out := make([]SyncPreviewItemView, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	return out
}
