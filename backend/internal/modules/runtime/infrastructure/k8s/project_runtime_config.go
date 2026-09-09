package k8s

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type LogCollectionInput struct {
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
}

type HealthProbeInput struct {
	Port                int32    `json:"port"`
	Probe               []string `json:"probe"`
	Path                string   `json:"path,omitempty"`
	InitialDelaySeconds int32    `json:"initial_delay_seconds"`
	PeriodSeconds       int32    `json:"period_seconds"`
	TimeoutSeconds      int32    `json:"timeout_seconds"`
	SuccessThreshold    int32    `json:"success_threshold"`
	FailureThreshold    int32    `json:"failure_threshold"`
}

type ServicePortInput struct {
	Port       int32  `json:"port"`
	TargetPort int32  `json:"target_port,omitempty"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
}

type IngressInput struct {
	Host string `json:"host,omitempty"`
	Path string `json:"path,omitempty"`
}

type DeploymentResourcesInput struct {
	CPURequest    string `json:"cpu_request"`
	CPULimit      string `json:"cpu_limit"`
	MemoryRequest string `json:"memory_request"`
	MemoryLimit   string `json:"memory_limit"`
}

type ProjectRuntimeConfigurator struct {
	access *ProjectClusterAccess
}

func NewProjectRuntimeConfigurator(access *ProjectClusterAccess) *ProjectRuntimeConfigurator {
	return &ProjectRuntimeConfigurator{access: access}
}

func (c *ProjectRuntimeConfigurator) UpsertLogCollection(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input LogCollectionInput,
) error {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	path := strings.TrimSpace(input.Path)
	pattern := strings.TrimSpace(input.Pattern)
	if path == "" {
		return fmt.Errorf("log path is required")
	}
	if pattern == "" {
		pattern = `^20[0-9]{2}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}`
	}
	name := filebeatConfigMapName(session.ProjectName)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: session.Namespace,
			Labels:    map[string]string{"app": session.ProjectName},
		},
		Data: map[string]string{
			"filebeat.yml": buildFilebeatYAML(path, pattern),
		},
	}
	existing, getErr := session.Client.CoreV1().ConfigMaps(session.Namespace).Get(ctx, name, metav1.GetOptions{})
	if getErr != nil && !apierrors.IsNotFound(getErr) {
		return getErr
	}
	if existing != nil && getErr == nil {
		cm.ResourceVersion = existing.ResourceVersion
		_, err = session.Client.CoreV1().ConfigMaps(session.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	} else {
		_, err = session.Client.CoreV1().ConfigMaps(session.Namespace).Create(ctx, cm, metav1.CreateOptions{})
	}
	if err != nil {
		return err
	}
	dep, err := getDeployment(ctx, session.Client, session.Namespace, session.ProjectName)
	if err != nil {
		return err
	}
	idx := primaryContainerIndex(dep, session.ProjectName)
	if idx < 0 {
		return fmt.Errorf("deployment container not found")
	}
	mountPath := path
	if !strings.HasSuffix(mountPath, "/") {
		mountPath += "/"
	}
	container := &dep.Spec.Template.Spec.Containers[idx]
	if !hasVolumeMount(container, mountPath) {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "app-logs",
			MountPath: mountPath,
		})
	}
	if !hasVolume(dep, "app-logs") {
		dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "app-logs",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}
	return rolloutRestartDeployment(ctx, session.Client, dep)
}

func (c *ProjectRuntimeConfigurator) UpsertHealthProbe(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input HealthProbeInput,
) error {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	if input.Port <= 0 {
		return fmt.Errorf("probe port is required")
	}
	dep, err := getDeployment(ctx, session.Client, session.Namespace, session.ProjectName)
	if err != nil {
		return err
	}
	idx := primaryContainerIndex(dep, session.ProjectName)
	if idx < 0 {
		return fmt.Errorf("deployment container not found")
	}
	probe := buildProbe(
		input.Port,
		input.Path,
		defaultInt32(input.InitialDelaySeconds, 10),
		defaultInt32(input.PeriodSeconds, 10),
		defaultInt32(input.TimeoutSeconds, 1),
		defaultInt32(input.SuccessThreshold, 1),
		defaultInt32(input.FailureThreshold, 3),
	)
	container := &dep.Spec.Template.Spec.Containers[idx]
	container.ReadinessProbe = nil
	container.LivenessProbe = nil
	for _, kind := range input.Probe {
		switch strings.ToLower(strings.TrimSpace(kind)) {
		case "readiness":
			container.ReadinessProbe = probe.DeepCopy()
		case "liveness":
			container.LivenessProbe = probe.DeepCopy()
		}
	}
	if container.ReadinessProbe == nil && container.LivenessProbe == nil {
		container.ReadinessProbe = probe
	}
	return rolloutRestartDeployment(ctx, session.Client, dep)
}

func (c *ProjectRuntimeConfigurator) UpdateServicePorts(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	ports []ServicePortInput,
) error {
	if len(ports) == 0 {
		return fmt.Errorf("at least one port is required")
	}
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	svc, err := getService(ctx, session.Client, session.Namespace, session.ProjectName)
	if err != nil {
		return err
	}
	svcPorts := make([]corev1.ServicePort, 0, len(ports))
	containerPorts := make([]corev1.ContainerPort, 0, len(ports))
	for _, item := range ports {
		if item.Port <= 0 {
			return fmt.Errorf("invalid service port")
		}
		target := item.TargetPort
		if target <= 0 {
			target = item.Port
		}
		protocol := corev1.ProtocolTCP
		if strings.EqualFold(item.Protocol, "UDP") {
			protocol = corev1.ProtocolUDP
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = fmt.Sprintf("http-%d", item.Port)
		}
		svcPorts = append(svcPorts, corev1.ServicePort{
			Name:       name,
			Port:       item.Port,
			TargetPort: intstr.FromInt32(target),
			Protocol:   protocol,
		})
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          name,
			ContainerPort: target,
			Protocol:      protocol,
		})
	}
	svc.Spec.Ports = svcPorts
	if _, err = session.Client.CoreV1().Services(session.Namespace).Update(ctx, svc, metav1.UpdateOptions{}); err != nil {
		return err
	}
	dep, err := getDeployment(ctx, session.Client, session.Namespace, session.ProjectName)
	if err != nil {
		return nil
	}
	idx := primaryContainerIndex(dep, session.ProjectName)
	if idx >= 0 {
		dep.Spec.Template.Spec.Containers[idx].Ports = containerPorts
		_ = rolloutRestartDeployment(ctx, session.Client, dep)
	}
	return nil
}

func (c *ProjectRuntimeConfigurator) CreateService(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) error {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	if _, err = getService(ctx, session.Client, session.Namespace, session.ProjectName); err == nil {
		return fmt.Errorf("service already exists")
	} else if !apierrors.IsNotFound(err) {
		return err
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      session.ProjectName,
			Namespace: session.Namespace,
			Labels:    map[string]string{"app": session.ProjectName},
			Annotations: map[string]string{
				"sidecar.istio.io/inject": "false",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": session.ProjectName},
			Ports: []corev1.ServicePort{{
				Name:       "http-8080",
				Port:       8080,
				TargetPort: intstr.FromInt32(8080),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
	_, err = session.Client.CoreV1().Services(session.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	return err
}

func (c *ProjectRuntimeConfigurator) CreateIngress(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input IngressInput,
) error {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	svc, err := getService(ctx, session.Client, session.Namespace, session.ProjectName)
	if err != nil {
		return fmt.Errorf("service is required before creating ingress: %w", err)
	}
	if len(svc.Spec.Ports) == 0 {
		return fmt.Errorf("service has no ports")
	}
	host := strings.TrimSpace(input.Host)
	if host == "" {
		host = fmt.Sprintf("%s.%s.local", session.ProjectName, session.Namespace)
	}
	path := strings.TrimSpace(input.Path)
	if path == "" {
		path = "/"
	}
	pathType := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      session.ProjectName,
			Namespace: session.Namespace,
			Labels:    map[string]string{"app": session.ProjectName},
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     path,
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: svc.Name,
									Port: networkingv1.ServiceBackendPort{
										Number: svc.Spec.Ports[0].Port,
									},
								},
							},
						}},
					},
				},
			}},
		},
	}
	_, err = session.Client.NetworkingV1().Ingresses(session.Namespace).Create(ctx, ing, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		existing, getErr := session.Client.NetworkingV1().Ingresses(session.Namespace).Get(ctx, ing.Name, metav1.GetOptions{})
		if getErr != nil {
			return getErr
		}
		ing.ResourceVersion = existing.ResourceVersion
		_, err = session.Client.NetworkingV1().Ingresses(session.Namespace).Update(ctx, ing, metav1.UpdateOptions{})
	}
	return err
}

func hasVolumeMount(container *corev1.Container, mountPath string) bool {
	for _, mount := range container.VolumeMounts {
		if mount.MountPath == mountPath {
			return true
		}
	}
	return false
}

func (c *ProjectRuntimeConfigurator) UpdateDeploymentResources(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input DeploymentResourcesInput,
) error {
	cpuRequest := strings.TrimSpace(input.CPURequest)
	cpuLimit := strings.TrimSpace(input.CPULimit)
	memoryRequest := strings.TrimSpace(input.MemoryRequest)
	memoryLimit := strings.TrimSpace(input.MemoryLimit)
	if cpuRequest == "" || cpuLimit == "" || memoryRequest == "" || memoryLimit == "" {
		return fmt.Errorf("cpu and memory request/limit are required")
	}
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	dep, err := getDeployment(ctx, session.Client, session.Namespace, session.ProjectName)
	if err != nil {
		return err
	}
	container := primaryContainer(dep, session.ProjectName)
	if container == nil {
		return fmt.Errorf("deployment container not found")
	}
	for i := range dep.Spec.Template.Spec.Containers {
		if dep.Spec.Template.Spec.Containers[i].Name != container.Name {
			continue
		}
		item := &dep.Spec.Template.Spec.Containers[i]
		if item.Resources.Requests == nil {
			item.Resources.Requests = corev1.ResourceList{}
		}
		if item.Resources.Limits == nil {
			item.Resources.Limits = corev1.ResourceList{}
		}
		reqCPU, err := resource.ParseQuantity(cpuRequest)
		if err != nil {
			return fmt.Errorf("invalid cpu request")
		}
		limCPU, err := resource.ParseQuantity(cpuLimit)
		if err != nil {
			return fmt.Errorf("invalid cpu limit")
		}
		reqMem, err := resource.ParseQuantity(memoryRequest)
		if err != nil {
			return fmt.Errorf("invalid memory request")
		}
		limMem, err := resource.ParseQuantity(memoryLimit)
		if err != nil {
			return fmt.Errorf("invalid memory limit")
		}
		item.Resources.Requests[corev1.ResourceCPU] = reqCPU
		item.Resources.Limits[corev1.ResourceCPU] = limCPU
		item.Resources.Requests[corev1.ResourceMemory] = reqMem
		item.Resources.Limits[corev1.ResourceMemory] = limMem
		break
	}
	_, err = session.Client.AppsV1().Deployments(session.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (c *ProjectRuntimeConfigurator) ScaleDeployment(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	replicas int32,
) error {
	if replicas < 0 || replicas > 1000 {
		return fmt.Errorf("replicas must be between 0 and 1000")
	}
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	dep, err := getDeployment(ctx, session.Client, session.Namespace, session.ProjectName)
	if err != nil {
		return err
	}
	dep.Spec.Replicas = &replicas
	_, err = session.Client.AppsV1().Deployments(session.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func hasVolume(dep *appsv1.Deployment, name string) bool {
	for _, vol := range dep.Spec.Template.Spec.Volumes {
		if vol.Name == name {
			return true
		}
	}
	return false
}

func defaultInt32(value, fallback int32) int32 {
	if value <= 0 {
		return fallback
	}
	return value
}
