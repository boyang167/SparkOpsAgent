package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type DeploymentOverview struct {
	Name                string            `json:"name"`
	Namespace           string            `json:"namespace"`
	Labels              map[string]string `json:"labels,omitempty"`
	Image               string            `json:"image,omitempty"`
	ImageTag            string            `json:"image_tag,omitempty"`
	Strategy            string            `json:"strategy,omitempty"`
	Replicas            int32             `json:"replicas"`
	AvailableReplicas   int32             `json:"available_replicas"`
	UnavailableReplicas int32             `json:"unavailable_replicas"`
	MemoryLimit         string            `json:"memory_limit,omitempty"`
	UID                 string            `json:"uid,omitempty"`
	CreatedAt           string            `json:"created_at,omitempty"`
}

type ServicePortOverview struct {
	Port       int32  `json:"port"`
	TargetPort string `json:"target_port"`
	Protocol   string `json:"protocol"`
	Name       string `json:"name,omitempty"`
}

type ServiceOverview struct {
	Name      string                `json:"name"`
	Namespace string                `json:"namespace"`
	Labels    map[string]string     `json:"labels,omitempty"`
	Annotations map[string]string     `json:"annotations,omitempty"`
	Selector  map[string]string     `json:"selector,omitempty"`
	Ports     []ServicePortOverview `json:"ports,omitempty"`
	Endpoints []string              `json:"endpoints,omitempty"`
	CreatedAt string                `json:"created_at,omitempty"`
}

type ProjectRuntimeOverview struct {
	Deployment *DeploymentOverview `json:"deployment,omitempty"`
	Service    *ServiceOverview    `json:"service,omitempty"`
}

type ProjectRuntimeInspector struct {
	access *ProjectClusterAccess
}

func NewProjectRuntimeInspector(access *ProjectClusterAccess) *ProjectRuntimeInspector {
	return &ProjectRuntimeInspector{access: access}
}

func (i *ProjectRuntimeInspector) Inspect(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) (*ProjectRuntimeOverview, error) {
	session, err := i.access.Open(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	out := &ProjectRuntimeOverview{}
	deployment, err := loadDeploymentOverview(ctx, session.Client, session.Namespace, session.ProjectName)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	if deployment != nil {
		out.Deployment = deployment
	}
	service, err := loadServiceOverview(ctx, session.Client, session.Namespace, session.ProjectName)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	if service != nil {
		out.Service = service
	}
	return out, nil
}

func loadDeploymentOverview(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, projectName string,
) (*DeploymentOverview, error) {
	names := []string{projectName, projectName + "-deployment"}
	var dep *appsv1.Deployment
	var err error
	for _, name := range names {
		dep, err = client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			break
		}
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	if dep == nil {
		return nil, apierrors.NewNotFound(appsv1.Resource("deployments"), projectName)
	}
	replicas := int32(1)
	if dep.Spec.Replicas != nil {
		replicas = *dep.Spec.Replicas
	}
	image, imageTag, memoryLimit := primaryContainerResources(dep, projectName)
	return &DeploymentOverview{
		Name:                dep.Name,
		Namespace:           dep.Namespace,
		Labels:              dep.Labels,
		Image:               image,
		ImageTag:            imageTag,
		Strategy:            string(dep.Spec.Strategy.Type),
		Replicas:            replicas,
		AvailableReplicas:   dep.Status.AvailableReplicas,
		UnavailableReplicas: dep.Status.UnavailableReplicas,
		MemoryLimit:         memoryLimit,
		UID:                 string(dep.UID),
		CreatedAt:           dep.CreationTimestamp.UTC().Format(time.RFC3339),
	}, nil
}

func primaryContainerResources(dep *appsv1.Deployment, projectName string) (image, tag, memoryLimit string) {
	var container *corev1.Container
	for i := range dep.Spec.Template.Spec.Containers {
		item := &dep.Spec.Template.Spec.Containers[i]
		if item.Name == projectName || container == nil {
			container = item
		}
		if item.Name == projectName {
			break
		}
	}
	if container == nil {
		return "", "", ""
	}
	image = container.Image
	if idx := strings.LastIndex(image, ":"); idx > strings.LastIndex(image, "/") {
		tag = image[idx+1:]
	}
	if container.Resources.Limits != nil {
		memoryLimit = container.Resources.Limits.Memory().String()
	}
	return image, tag, memoryLimit
}

func loadServiceOverview(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, projectName string,
) (*ServiceOverview, error) {
	names := []string{projectName, projectName + "-service"}
	var svc *corev1.Service
	var err error
	for _, name := range names {
		svc, err = client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			break
		}
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	if svc == nil {
		return nil, apierrors.NewNotFound(corev1.Resource("services"), projectName)
	}
	ports := make([]ServicePortOverview, 0, len(svc.Spec.Ports))
	endpoints := make([]string, 0, len(svc.Spec.Ports))
	for _, port := range svc.Spec.Ports {
		target := port.TargetPort.String()
		ports = append(ports, ServicePortOverview{
			Port: port.Port, TargetPort: target, Protocol: string(port.Protocol), Name: port.Name,
		})
		endpoints = append(endpoints, fmt.Sprintf("%s:%d %s", svc.Name, port.Port, port.Protocol))
	}
	return &ServiceOverview{
		Name:        svc.Name,
		Namespace:   svc.Namespace,
		Labels:      svc.Labels,
		Annotations: svc.Annotations,
		Selector:    svc.Spec.Selector,
		Ports:     ports,
		Endpoints: endpoints,
		CreatedAt: svc.CreationTimestamp.UTC().Format(time.RFC3339),
	}, nil
}
