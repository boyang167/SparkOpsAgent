package k8s

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const sparkopsAppLabel = "app"

type ProjectConfigMapVM struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
	CreatedAt string            `json:"created_at,omitempty"`
}

type CreateConfigMapInput struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data,omitempty"`
}

type UpsertConfigMapEntryInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ReplaceConfigMapDataInput struct {
	Data map[string]string `json:"data"`
}

var (
	k8sNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	cmKeyPattern   = regexp.MustCompile(`^[-._a-zA-Z0-9]+$`)
)

func (c *ProjectRuntimeConfigurator) ListConfigMaps(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) ([]ProjectConfigMapVM, error) {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	list, err := session.Client.CoreV1().ConfigMaps(session.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	referenced := referencedConfigMapNames(ctx, session)
	out := make([]ProjectConfigMapVM, 0)
	for i := range list.Items {
		cm := list.Items[i]
		if !configMapBelongsToProject(&cm, session.ProjectName, referenced) {
			continue
		}
		out = append(out, toConfigMapVM(&cm))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *ProjectRuntimeConfigurator) CreateConfigMap(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input CreateConfigMapInput,
) error {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	name := strings.ToLower(strings.TrimSpace(input.Name))
	if name == "" {
		name = session.ProjectName
	}
	if !k8sNamePattern.MatchString(name) || len(name) > 253 {
		return fmt.Errorf("invalid configmap name")
	}
	data := map[string]string{}
	for k, v := range input.Data {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		if !cmKeyPattern.MatchString(key) {
			return fmt.Errorf("invalid configmap key: %s", key)
		}
		data[key] = v
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: session.Namespace,
			Labels: map[string]string{
				sparkopsAppLabel: session.ProjectName,
			},
		},
		Data: data,
	}
	_, err = session.Client.CoreV1().ConfigMaps(session.Namespace).Create(ctx, cm, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("configmap %s already exists", name)
	}
	return err
}

func (c *ProjectRuntimeConfigurator) UpsertConfigMapEntry(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	name string,
	input UpsertConfigMapEntryInput,
) error {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	key := strings.TrimSpace(input.Key)
	if !k8sNamePattern.MatchString(name) {
		return fmt.Errorf("invalid configmap name")
	}
	if !cmKeyPattern.MatchString(key) {
		return fmt.Errorf("invalid configmap key")
	}
	cm, err := session.Client.CoreV1().ConfigMaps(session.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	referenced := referencedConfigMapNames(ctx, session)
	if !configMapBelongsToProject(cm, session.ProjectName, referenced) {
		return fmt.Errorf("configmap does not belong to this project")
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[key] = input.Value
	_, err = session.Client.CoreV1().ConfigMaps(session.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

func (c *ProjectRuntimeConfigurator) ReplaceConfigMapData(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	name string,
	input ReplaceConfigMapDataInput,
) error {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if !k8sNamePattern.MatchString(name) {
		return fmt.Errorf("invalid configmap name")
	}
	cm, err := session.Client.CoreV1().ConfigMaps(session.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	referenced := referencedConfigMapNames(ctx, session)
	if !configMapBelongsToProject(cm, session.ProjectName, referenced) {
		return fmt.Errorf("configmap does not belong to this project")
	}
	data := map[string]string{}
	for k, v := range input.Data {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		if !cmKeyPattern.MatchString(key) {
			return fmt.Errorf("invalid configmap key: %s", key)
		}
		data[key] = v
	}
	cm.Data = data
	_, err = session.Client.CoreV1().ConfigMaps(session.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

func (c *ProjectRuntimeConfigurator) DeleteConfigMapEntry(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	name string,
	key string,
) error {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	key = strings.TrimSpace(key)
	cm, err := session.Client.CoreV1().ConfigMaps(session.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	referenced := referencedConfigMapNames(ctx, session)
	if !configMapBelongsToProject(cm, session.ProjectName, referenced) {
		return fmt.Errorf("configmap does not belong to this project")
	}
	if cm.Data != nil {
		delete(cm.Data, key)
	}
	_, err = session.Client.CoreV1().ConfigMaps(session.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

func toConfigMapVM(cm *corev1.ConfigMap) ProjectConfigMapVM {
	data := map[string]string{}
	for k, v := range cm.Data {
		data[k] = v
	}
	created := ""
	if !cm.CreationTimestamp.IsZero() {
		created = cm.CreationTimestamp.UTC().Format("2006-01-02 15:04:05")
	}
	return ProjectConfigMapVM{
		Name:      cm.Name,
		Namespace: cm.Namespace,
		Labels:    cm.Labels,
		Data:      data,
		CreatedAt: created,
	}
}

func configMapBelongsToProject(cm *corev1.ConfigMap, projectName string, referenced map[string]struct{}) bool {
	if cm == nil || projectName == "" {
		return false
	}
	if cm.Name == projectName {
		return true
	}
	if cm.Labels[sparkopsAppLabel] == projectName {
		return true
	}
	if _, ok := referenced[cm.Name]; ok {
		return true
	}
	return false
}

func referencedConfigMapNames(ctx context.Context, session *ProjectClusterSession) map[string]struct{} {
	out := map[string]struct{}{}
	if session == nil || session.Client == nil {
		return out
	}
	dep, err := session.Client.AppsV1().Deployments(session.Namespace).Get(ctx, session.ProjectName, metav1.GetOptions{})
	if err != nil {
		return out
	}
	collectDeploymentConfigMaps(dep, out)
	return out
}

func collectDeploymentConfigMaps(dep *appsv1.Deployment, out map[string]struct{}) {
	if dep == nil {
		return
	}
	pod := dep.Spec.Template.Spec
	for _, vol := range pod.Volumes {
		if vol.ConfigMap != nil && vol.ConfigMap.Name != "" {
			out[vol.ConfigMap.Name] = struct{}{}
		}
	}
	for _, c := range append(append([]corev1.Container{}, pod.Containers...), pod.InitContainers...) {
		for _, envFrom := range c.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name != "" {
				out[envFrom.ConfigMapRef.Name] = struct{}{}
			}
		}
		for _, env := range c.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil && env.ValueFrom.ConfigMapKeyRef.Name != "" {
				out[env.ValueFrom.ConfigMapKeyRef.Name] = struct{}{}
			}
		}
	}
}
