package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"gopkg.in/yaml.v3"
)

type LogCollectionOverview struct {
	Configured bool   `json:"configured"`
	Path       string `json:"path,omitempty"`
	Pattern    string `json:"pattern,omitempty"`
	FileSuffix string `json:"file_suffix,omitempty"`
}

type HealthProbeOverview struct {
	Configured          bool   `json:"configured"`
	Port                int32  `json:"port,omitempty"`
	Protocol            string `json:"protocol,omitempty"`
	Path                string `json:"path,omitempty"`
	Readiness           bool   `json:"readiness"`
	Liveness            bool   `json:"liveness"`
	InitialDelaySeconds int32  `json:"initial_delay_seconds,omitempty"`
	PeriodSeconds       int32  `json:"period_seconds,omitempty"`
	TimeoutSeconds      int32  `json:"timeout_seconds,omitempty"`
	SuccessThreshold    int32  `json:"success_threshold,omitempty"`
	FailureThreshold    int32  `json:"failure_threshold,omitempty"`
}

type IngressPathOverview struct {
	Path        string `json:"path"`
	ServiceName string `json:"service_name"`
	ServicePort int32  `json:"service_port"`
}

type IngressOverview struct {
	Name      string                `json:"name"`
	Host      string                `json:"host"`
	Paths     []IngressPathOverview `json:"paths,omitempty"`
	CreatedAt string                `json:"created_at,omitempty"`
}

func loadLogCollectionOverview(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, projectName string,
) (*LogCollectionOverview, error) {
	names := []string{filebeatConfigMapName(projectName), projectName}
	for _, name := range names {
		cm, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		if overview := parseFilebeatConfigMap(cm); overview != nil {
			return overview, nil
		}
	}
	return &LogCollectionOverview{Configured: false, FileSuffix: "*.log"}, nil
}

func parseFilebeatConfigMap(cm *corev1.ConfigMap) *LogCollectionOverview {
	if cm == nil || cm.Data == nil {
		return nil
	}
	raw, ok := cm.Data["filebeat.yml"]
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	prospectors, _ := parsed["filebeat.prospectors"].([]any)
	if len(prospectors) == 0 {
		return &LogCollectionOverview{Configured: true, FileSuffix: "*.log"}
	}
	first, _ := prospectors[0].(map[string]any)
	path := ""
	if paths, ok := first["paths"].([]any); ok && len(paths) > 0 {
		path, _ = paths[0].(string)
	}
	path = strings.TrimSuffix(path, "*.log")
	pattern := ""
	if multiline, ok := first["multiline"].(map[string]any); ok {
		pattern, _ = multiline["pattern"].(string)
	}
	return &LogCollectionOverview{
		Configured: true,
		Path:       path,
		Pattern:    pattern,
		FileSuffix: "*.log",
	}
}

func loadHealthProbeOverview(dep *appsv1.Deployment, projectName string) *HealthProbeOverview {
	container := primaryContainer(dep, projectName)
	if container == nil {
		return &HealthProbeOverview{}
	}
	var probe *corev1.Probe
	out := &HealthProbeOverview{
		Readiness: container.ReadinessProbe != nil,
		Liveness:  container.LivenessProbe != nil,
	}
	if container.ReadinessProbe != nil {
		probe = container.ReadinessProbe
	} else if container.LivenessProbe != nil {
		probe = container.LivenessProbe
	}
	if probe == nil {
		return out
	}
	out.Configured = true
	out.InitialDelaySeconds = probe.InitialDelaySeconds
	out.PeriodSeconds = probe.PeriodSeconds
	out.TimeoutSeconds = probe.TimeoutSeconds
	out.SuccessThreshold = probe.SuccessThreshold
	out.FailureThreshold = probe.FailureThreshold
	if probe.HTTPGet != nil {
		out.Protocol = "HTTP"
		out.Path = probe.HTTPGet.Path
		out.Port = probe.HTTPGet.Port.IntVal
		if out.Port == 0 {
			out.Port = probe.HTTPGet.Port.IntVal
		}
	} else if probe.TCPSocket != nil {
		out.Protocol = "TCP"
		out.Port = probe.TCPSocket.Port.IntVal
	}
	return out
}

func loadIngressOverview(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, projectName string,
) (*IngressOverview, error) {
	list, err := client.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=" + projectName,
	})
	if err != nil {
		return nil, err
	}
	var ing *networkingv1.Ingress
	for i := range list.Items {
		item := &list.Items[i]
		if item.Name == projectName || strings.HasPrefix(item.Name, projectName+"-") {
			ing = item
			break
		}
	}
	if ing == nil && len(list.Items) > 0 {
		ing = &list.Items[0]
	}
	if ing == nil {
		ing, err = client.NetworkingV1().Ingresses(namespace).Get(ctx, projectName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
	}
	if ing == nil || len(ing.Spec.Rules) == 0 {
		return nil, nil
	}
	rule := ing.Spec.Rules[0]
	out := &IngressOverview{
		Name:      ing.Name,
		Host:      rule.Host,
		CreatedAt: ing.CreationTimestamp.UTC().Format(time.RFC3339),
	}
	if rule.HTTP != nil {
		for _, path := range rule.HTTP.Paths {
			item := IngressPathOverview{Path: path.Path}
			if path.Backend.Service != nil {
				item.ServiceName = path.Backend.Service.Name
				item.ServicePort = path.Backend.Service.Port.Number
			}
			out.Paths = append(out.Paths, item)
		}
	}
	return out, nil
}

func filebeatConfigMapName(projectName string) string {
	return projectName + "-filebeat"
}

func buildFilebeatYAML(path, pattern string) string {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	pattern = strings.ReplaceAll(pattern, "'", "''")
	return fmt.Sprintf(`filebeat.prospectors:
- type: log
  paths:
    - %s*.log
  multiline:
    pattern: '%s'
    negate: true
    match: after
`, path, pattern)
}

func getDeployment(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, projectName string,
) (*appsv1.Deployment, error) {
	names := []string{projectName, projectName + "-deployment"}
	for _, name := range names {
		dep, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			return dep, nil
		}
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	return nil, apierrors.NewNotFound(appsv1.Resource("deployments"), projectName)
}

func getService(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, projectName string,
) (*corev1.Service, error) {
	names := []string{projectName, projectName + "-service"}
	for _, name := range names {
		svc, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			return svc, nil
		}
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	return nil, apierrors.NewNotFound(corev1.Resource("services"), projectName)
}

func primaryContainer(dep *appsv1.Deployment, projectName string) *corev1.Container {
	if dep == nil {
		return nil
	}
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
	return container
}

func primaryContainerIndex(dep *appsv1.Deployment, projectName string) int {
	if dep == nil {
		return -1
	}
	fallback := -1
	for i := range dep.Spec.Template.Spec.Containers {
		if dep.Spec.Template.Spec.Containers[i].Name == projectName {
			return i
		}
		if fallback < 0 {
			fallback = i
		}
	}
	return fallback
}

func buildProbe(port int32, path string, initialDelay, period, timeout, success, failure int32) *corev1.Probe {
	probe := &corev1.Probe{
		InitialDelaySeconds: initialDelay,
		PeriodSeconds:       period,
		TimeoutSeconds:      timeout,
		SuccessThreshold:    success,
		FailureThreshold:    failure,
	}
	portVal := intstr.FromInt32(port)
	if strings.TrimSpace(path) != "" {
		probe.HTTPGet = &corev1.HTTPGetAction{
			Path: path,
			Port: portVal,
		}
	} else {
		probe.TCPSocket = &corev1.TCPSocketAction{Port: portVal}
	}
	return probe
}

func rolloutRestartDeployment(ctx context.Context, client kubernetes.Interface, dep *appsv1.Deployment) error {
	if dep == nil {
		return fmt.Errorf("deployment is required")
	}
	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = map[string]string{}
	}
	dep.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().UTC().Format(time.RFC3339Nano)
	_, err := client.AppsV1().Deployments(dep.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}
