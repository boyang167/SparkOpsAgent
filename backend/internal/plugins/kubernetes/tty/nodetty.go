package tty

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	NodeTTYGenerateName = "sparkops-nodetty-"
	NodeTTYNamespace    = "kube-system"
	DefaultNodeTTYImage = "alpine:latest"
	NodeTTYTTL          = 30 * time.Minute
)

var (
	ErrNodeTTYTargetInvalid        = errors.New("nodetty target invalid")
	ErrNodeTTYTargetNotFound       = errors.New("nodetty target not found")
	ErrNodeTTYPodCreateFailed      = errors.New("nodetty pod create failed")
	ErrNodeTTYNamespaceUnavailable = errors.New("nodetty namespace unavailable")
)

type NodeTTYRequest struct {
	Node string `json:"node"`
}

// ProvisionNodeTTY creates a short-lived privileged nsenter pod on one node.
// HostPID plus Privileged access exposes host namespaces, so callers must
// enforce admin authorization before reaching this method.
func (p *KubernetesProvisioner) ProvisionNodeTTY(ctx context.Context, client kubernetes.Interface, _ dynamic.Interface, _ *rest.Config, req NodeTTYRequest) (Session, error) {
	if client == nil || !validDNSSubdomain(req.Node) {
		return Session{}, ErrNodeTTYTargetInvalid
	}
	if _, err := client.CoreV1().Nodes().Get(ctx, req.Node, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return Session{}, ErrNodeTTYTargetNotFound
		}
		return Session{}, ErrTTYClusterUnavailable
	}

	sessionID := uuidString()
	deadline := int64(NodeTTYTTL / time.Second)
	zero := int64(0)
	trueValue := true
	falseValue := false
	pod, err := client.CoreV1().Pods(NodeTTYNamespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: NodeTTYGenerateName,
			Namespace:    NodeTTYNamespace,
			Labels:       sessionLabels(sessionID),
		},
		Spec: corev1.PodSpec{
			ActiveDeadlineSeconds:         &deadline,
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: &zero,
			NodeName:                      req.Node,
			HostPID:                       true,
			HostNetwork:                   false,
			Tolerations: []corev1.Toleration{{
				Operator: corev1.TolerationOpExists,
			}},
			Containers: []corev1.Container{{
				Name:            "node-shell",
				Image:           p.nodeTTYImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"nsenter"},
				Args:            []string{"-t", "1", "-m", "-u", "-i", "-n", "sleep", "1800"},
				SecurityContext: &corev1.SecurityContext{
					Privileged:               &trueValue,
					AllowPrivilegeEscalation: &trueValue,
				},
			}},
			EnableServiceLinks: &falseValue,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
			return Session{}, ErrNodeTTYNamespaceUnavailable
		}
		return Session{}, ErrNodeTTYPodCreateFailed
	}

	return Session{
		ID:        sessionID,
		Pod:       &ResourceRef{Name: pod.Name, Namespace: pod.Namespace},
		ExpiresAt: p.now().UTC().Add(NodeTTYTTL),
	}, nil
}

func (h *Handler) NodeTTY(c *gin.Context) {
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
	var req NodeTTYRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_TTY_REQUEST_INVALID", "invalid tty request", http.StatusBadRequest)
		return
	}
	if !validDNSSubdomain(req.Node) {
		httpx.Fail(c, "PLUGIN_KUBERNETES_TTY_TARGET_INVALID", "invalid tty target", http.StatusBadRequest)
		return
	}
	client, dynamicClient, restCfg, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	session, err := h.provisioner.ProvisionNodeTTY(c.Request.Context(), client, dynamicClient, restCfg, req)
	if err != nil {
		handleTTYError(c, err)
		return
	}
	httpx.OK(c, session)
}

func uuidString() string {
	return uuid.NewString()
}
