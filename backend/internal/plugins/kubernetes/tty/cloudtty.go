package tty

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
)

const (
	ManagedByLabel = "app.kubernetes.io/managed-by"
	ManagedByValue = "sparkops-kubernetes-plugin"
	SessionIDLabel = "session-id"

	CloudTTYGenerateName = "sparkops-cloudtty-"
	DefaultCloudTTYImage = "ghcr.io/cloudtty/cloudtty:latest"
	CloudTTYPort         = int32(7681)
	CloudTTYTTL          = time.Hour
)

var (
	ErrCloudTTYTargetInvalid       = errors.New("cloudtty target invalid")
	ErrCloudTTYTargetNotFound      = errors.New("cloudtty target not found")
	ErrCloudTTYPodCreateFailed     = errors.New("cloudtty pod create failed")
	ErrCloudTTYServiceCreateFailed = errors.New("cloudtty service create failed")
	ErrTTYClusterUnavailable       = errors.New("kubernetes cluster unavailable")
)

type CloudTTYRequest struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
}

type ResourceRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type Session struct {
	ID        string       `json:"id"`
	Pod       ResourceRef  `json:"pod"`
	Service   *ResourceRef `json:"service,omitempty"`
	ExpiresAt time.Time    `json:"expires_at"`
}

type Provisioner interface {
	ProvisionCloudTTY(ctx context.Context, client kubernetes.Interface, req CloudTTYRequest) (Session, error)
	ProvisionNodeTTY(ctx context.Context, client kubernetes.Interface, req NodeTTYRequest) (Session, error)
}

type ProvisionerOptions struct {
	Now           func() time.Time
	CloudTTYImage string
	NodeTTYImage  string
}

type KubernetesProvisioner struct {
	now           func() time.Time
	cloudTTYImage string
	nodeTTYImage  string
}

func NewProvisioner(opts ProvisionerOptions) *KubernetesProvisioner {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	cloudTTYImage := strings.TrimSpace(opts.CloudTTYImage)
	if cloudTTYImage == "" {
		cloudTTYImage = DefaultCloudTTYImage
	}
	nodeTTYImage := strings.TrimSpace(opts.NodeTTYImage)
	if nodeTTYImage == "" {
		nodeTTYImage = DefaultNodeTTYImage
	}
	return &KubernetesProvisioner{now: now, cloudTTYImage: cloudTTYImage, nodeTTYImage: nodeTTYImage}
}

func (p *KubernetesProvisioner) ProvisionCloudTTY(ctx context.Context, client kubernetes.Interface, req CloudTTYRequest) (Session, error) {
	if client == nil || !validDNSLabel(req.Namespace) || !validDNSSubdomain(req.Pod) || !validDNSLabel(req.Container) {
		return Session{}, ErrCloudTTYTargetInvalid
	}
	target, err := client.CoreV1().Pods(req.Namespace).Get(ctx, req.Pod, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return Session{}, ErrCloudTTYTargetNotFound
		}
		return Session{}, ErrTTYClusterUnavailable
	}
	if !podHasContainer(target, req.Container) {
		return Session{}, ErrCloudTTYTargetInvalid
	}

	sessionID := uuid.NewString()
	labels := sessionLabels(sessionID)
	deadline := int64(CloudTTYTTL / time.Second)
	falseValue := false
	pod, err := client.CoreV1().Pods(req.Namespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: CloudTTYGenerateName,
			Namespace:    req.Namespace,
			Labels:       labels,
		},
		Spec: corev1.PodSpec{
			ActiveDeadlineSeconds: &deadline,
			RestartPolicy:         corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:            "web-tty",
				Image:           p.cloudTTYImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Args: []string{
					"--command",
					"kubectl", "exec", "-it", "-n", req.Namespace, req.Pod, "-c", req.Container, "--", "sh",
				},
				Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: CloudTTYPort}},
				SecurityContext: &corev1.SecurityContext{
					Privileged:               &falseValue,
					AllowPrivilegeEscalation: &falseValue,
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
					SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
				},
			}},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return Session{}, ErrCloudTTYPodCreateFailed
	}

	service, err := client.CoreV1().Services(req.Namespace).Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: CloudTTYGenerateName,
			Namespace:    req.Namespace,
			Labels:       labels,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "v1",
				Kind:               "Pod",
				Name:               pod.Name,
				UID:                pod.UID,
				Controller:         &falseValue,
				BlockOwnerDeletion: &falseValue,
			}},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       80,
				TargetPort: intstr.FromInt32(CloudTTYPort),
			}},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		_ = deletePodNow(ctx, client, req.Namespace, pod.Name)
		return Session{}, ErrCloudTTYServiceCreateFailed
	}

	return Session{
		ID:        sessionID,
		Pod:       ResourceRef{Name: pod.Name, Namespace: pod.Namespace},
		Service:   &ResourceRef{Name: service.Name, Namespace: service.Namespace},
		ExpiresAt: p.now().UTC().Add(CloudTTYTTL),
	}, nil
}

type Handler struct {
	ctx         sparkplugin.PluginContext
	provisioner Provisioner
}

func NewHandler(ctx sparkplugin.PluginContext) *Handler {
	return NewHandlerWithProvisioner(ctx, NewProvisioner(ProvisionerOptions{}))
}

func NewHandlerWithProvisioner(ctx sparkplugin.PluginContext, provisioner Provisioner) *Handler {
	return &Handler{ctx: ctx, provisioner: provisioner}
}

func (h *Handler) CloudTTY(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	wsID, _, ok := requireWrite(c.Request.Context(), c, h.ctx.Writer)
	if !ok {
		return
	}
	clusterID, ok := clusterIDFromParam(c)
	if !ok {
		return
	}
	var req CloudTTYRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_TTY_REQUEST_INVALID", "invalid tty request", http.StatusBadRequest)
		return
	}
	if !validDNSLabel(req.Namespace) || !validDNSSubdomain(req.Pod) || !validDNSLabel(req.Container) {
		httpx.Fail(c, "PLUGIN_KUBERNETES_TTY_TARGET_INVALID", "invalid tty target", http.StatusBadRequest)
		return
	}
	client, _, _, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	session, err := h.provisioner.ProvisionCloudTTY(c.Request.Context(), client, req)
	if err != nil {
		handleTTYError(c, err)
		return
	}
	httpx.OK(c, session)
}

func (h *Handler) ready(c *gin.Context) bool {
	if h == nil || h.ctx.Clusters == nil || h.ctx.Writer == nil || h.provisioner == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return false
	}
	return true
}

func requireWrite(ctx context.Context, c *gin.Context, writer sparkplugin.WriteAuthorizer) (kernel.WorkspaceID, kernel.MemberID, bool) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	allowed, err := writer.CanWrite(ctx, wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	if !allowed {
		httpx.Fail(c, "GOVERNANCE_API_ACCESS_DENIED", "governance admin permission required", http.StatusForbidden)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	return wsID, memberID, true
}

func clusterIDFromParam(c *gin.Context) (kernel.ClusterInfoID, bool) {
	clusterID, err := kernel.ParseClusterInfoID(c.Param("clusterId"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_INVALID", "invalid cluster id", http.StatusBadRequest)
		return kernel.ClusterInfoID{}, false
	}
	return clusterID, true
}

func handleClusterError(c *gin.Context, err error) {
	if de, ok := kernel.IsDomainError(err); ok {
		switch de.Code() {
		case "PLUGIN_CLUSTER_FORBIDDEN":
			httpx.Fail(c, de.Code(), "cluster is not granted to workspace", http.StatusForbidden)
			return
		case "PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE":
			httpx.Fail(c, de.Code(), "cluster credentials are incomplete", http.StatusServiceUnavailable)
			return
		}
	}
	if apierrors.IsUnauthorized(err) || apierrors.IsForbidden(err) {
		httpx.Fail(c, "PLUGIN_CLUSTER_FORBIDDEN", "cluster is not granted to workspace", http.StatusForbidden)
		return
	}
	httpx.Fail(c, "PLUGIN_KUBERNETES_TTY_UPSTREAM_UNAVAILABLE", "kubernetes tty upstream unavailable", http.StatusBadGateway)
}

func handleTTYError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrCloudTTYTargetInvalid), errors.Is(err, ErrNodeTTYTargetInvalid):
		httpx.Fail(c, "PLUGIN_KUBERNETES_TTY_TARGET_INVALID", "invalid tty target", http.StatusBadRequest)
	case errors.Is(err, ErrCloudTTYTargetNotFound), errors.Is(err, ErrNodeTTYTargetNotFound):
		httpx.Fail(c, "PLUGIN_KUBERNETES_TTY_TARGET_NOT_FOUND", "tty target not found", http.StatusNotFound)
	case errors.Is(err, ErrNodeTTYNamespaceUnavailable):
		httpx.Fail(c, "PLUGIN_KUBERNETES_TTY_NAMESPACE_FORBIDDEN", "tty namespace is not writable", http.StatusForbidden)
	case errors.Is(err, ErrCloudTTYPodCreateFailed), errors.Is(err, ErrCloudTTYServiceCreateFailed), errors.Is(err, ErrNodeTTYPodCreateFailed):
		httpx.Fail(c, "PLUGIN_KUBERNETES_TTY_CREATE_FAILED", "tty resource create failed", http.StatusBadGateway)
	default:
		httpx.Fail(c, "PLUGIN_KUBERNETES_TTY_UPSTREAM_UNAVAILABLE", "kubernetes tty upstream unavailable", http.StatusBadGateway)
	}
}

func sessionLabels(sessionID string) map[string]string {
	return map[string]string{
		ManagedByLabel: ManagedByValue,
		SessionIDLabel: sessionID,
	}
}

func podHasContainer(pod *corev1.Pod, container string) bool {
	if pod == nil {
		return false
	}
	for _, c := range pod.Spec.Containers {
		if c.Name == container {
			return true
		}
	}
	return false
}

func deletePodNow(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	grace := int64(0)
	propagation := metav1.DeletePropagationBackground
	return client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: &grace,
		PropagationPolicy:  &propagation,
	})
}

func validDNSLabel(value string) bool {
	return len(validation.IsDNS1123Label(value)) == 0
}

func validDNSSubdomain(value string) bool {
	return len(validation.IsDNS1123Subdomain(value)) == 0
}
