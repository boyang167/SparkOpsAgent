package tty

import (
	"context"
	"encoding/json"
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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	ManagedByLabel = "app.kubernetes.io/managed-by"
	ManagedByValue = "sparkops-kubernetes-plugin"
	SessionIDLabel = "session-id"

	CloudTTYGenerateName      = "sparkops-cloudtty-"
	CloudShellGroup           = "cloudshell.cloudtty.io"
	CloudShellVersion         = "v1alpha1"
	CloudShellResource        = "cloudshells"
	CloudShellKind            = "CloudShell"
	CloudShellAPIVersion      = CloudShellGroup + "/" + CloudShellVersion
	CloudShellTTLSeconds      = int64(1800)
	CloudTTYTTL               = 30 * time.Minute
	cleanupTimeout            = 10 * time.Second
	kubeconfigSecretKey       = "config"
	cloudTTYCommandShell      = "sh"
	cloudTTYCommandExecutable = "kubectl"
)

var (
	ErrCloudTTYTargetInvalid           = errors.New("cloudtty target invalid")
	ErrCloudTTYTargetNotFound          = errors.New("cloudtty target not found")
	ErrCloudTTYSecretCreateFailed      = errors.New("cloudtty secret create failed")
	ErrCloudTTYCloudShellCreateFailed  = errors.New("cloudtty cloudshell create failed")
	ErrCloudTTYCRDUnavailable          = errors.New("cloudtty crd unavailable")
	ErrCloudTTYOwnerPatchFailed        = errors.New("cloudtty owner patch failed")
	ErrCloudTTYCredentialConfigInvalid = errors.New("cloudtty credential config invalid")
	ErrTTYClusterUnavailable           = errors.New("kubernetes cluster unavailable")
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
	ID         string       `json:"id"`
	CloudShell *ResourceRef `json:"cloudshell,omitempty"`
	Pod        *ResourceRef `json:"pod,omitempty"`
	Service    *ResourceRef `json:"service,omitempty"`
	ExpiresAt  time.Time    `json:"expires_at"`
}

type Provisioner interface {
	ProvisionCloudTTY(ctx context.Context, client kubernetes.Interface, dynamicClient dynamic.Interface, restConfig *rest.Config, req CloudTTYRequest) (Session, error)
	ProvisionNodeTTY(ctx context.Context, client kubernetes.Interface, dynamicClient dynamic.Interface, restConfig *rest.Config, req NodeTTYRequest) (Session, error)
}

type ProvisionerOptions struct {
	Now          func() time.Time
	NodeTTYImage string
}

type KubernetesProvisioner struct {
	now          func() time.Time
	nodeTTYImage string
}

func NewProvisioner(opts ProvisionerOptions) *KubernetesProvisioner {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	nodeTTYImage := strings.TrimSpace(opts.NodeTTYImage)
	if nodeTTYImage == "" {
		nodeTTYImage = DefaultNodeTTYImage
	}
	return &KubernetesProvisioner{now: now, nodeTTYImage: nodeTTYImage}
}

func (p *KubernetesProvisioner) ProvisionCloudTTY(ctx context.Context, client kubernetes.Interface, dynamicClient dynamic.Interface, restConfig *rest.Config, req CloudTTYRequest) (Session, error) {
	if client == nil || dynamicClient == nil || restConfig == nil || !validDNSLabel(req.Namespace) || !validDNSSubdomain(req.Pod) || !validDNSLabel(req.Container) {
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
	kubeconfig, err := kubeconfigFromRESTConfig(restConfig, req.Namespace)
	if err != nil {
		return Session{}, ErrCloudTTYCredentialConfigInvalid
	}

	sessionID := uuid.NewString()
	labels := sessionLabels(sessionID)
	secret, err := client.CoreV1().Secrets(req.Namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: CloudTTYGenerateName,
			Namespace:    req.Namespace,
			Labels:       labels,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			kubeconfigSecretKey: kubeconfig,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return Session{}, ErrCloudTTYSecretCreateFailed
	}

	cr, err := dynamicClient.Resource(cloudShellGVR()).Namespace(req.Namespace).Create(ctx, cloudShellObject(req, secret.Name, labels), metav1.CreateOptions{})
	if err != nil {
		deleteSecretWithCleanupContext(client, req.Namespace, secret.Name)
		if apierrors.IsNotFound(err) {
			return Session{}, ErrCloudTTYCRDUnavailable
		}
		return Session{}, ErrCloudTTYCloudShellCreateFailed
	}

	if err := patchSecretOwner(ctx, client, req.Namespace, secret.Name, cr); err != nil {
		deleteCloudShellWithCleanupContext(dynamicClient, req.Namespace, cr.GetName())
		deleteSecretWithCleanupContext(client, req.Namespace, secret.Name)
		return Session{}, ErrCloudTTYOwnerPatchFailed
	}

	return Session{
		ID:         sessionID,
		CloudShell: &ResourceRef{Name: cr.GetName(), Namespace: cr.GetNamespace()},
		ExpiresAt:  p.now().UTC().Add(CloudTTYTTL),
	}, nil
}

func cloudShellGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: CloudShellGroup, Version: CloudShellVersion, Resource: CloudShellResource}
}

func cloudShellObject(req CloudTTYRequest, secretName string, labels map[string]string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": CloudShellAPIVersion,
			"kind":       CloudShellKind,
			"metadata": map[string]any{
				"generateName": CloudTTYGenerateName,
				"namespace":    req.Namespace,
				"labels":       stringMapAny(labels),
			},
			"spec": map[string]any{
				"secretRef": map[string]any{
					"name": secretName,
				},
				"commandAction":          cloudTTYCommand(req),
				"cleanup":                true,
				"ttlSecondsAfterStarted": CloudShellTTLSeconds,
			},
		},
	}
}

func stringMapAny(values map[string]string) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloudTTYCommand(req CloudTTYRequest) string {
	return cloudTTYCommandExecutable + " exec -it -n " + req.Namespace + " " + req.Pod + " -c " + req.Container + " -- " + cloudTTYCommandShell
}

func patchSecretOwner(ctx context.Context, client kubernetes.Interface, namespace, name string, cr *unstructured.Unstructured) error {
	falseValue := false
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"ownerReferences": []map[string]any{{
				"apiVersion":         CloudShellAPIVersion,
				"kind":               CloudShellKind,
				"name":               cr.GetName(),
				"uid":                cr.GetUID(),
				"controller":         falseValue,
				"blockOwnerDeletion": falseValue,
			}},
		},
	})
	if err != nil {
		return err
	}
	_, err = client.CoreV1().Secrets(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func kubeconfigFromRESTConfig(restConfig *rest.Config, namespace string) ([]byte, error) {
	if restConfig == nil || strings.TrimSpace(restConfig.Host) == "" {
		return nil, errors.New("missing host")
	}
	authInfo := &clientcmdapi.AuthInfo{
		Token:                 restConfig.BearerToken,
		ClientCertificateData: restConfig.TLSClientConfig.CertData,
		ClientKeyData:         restConfig.TLSClientConfig.KeyData,
	}
	if authInfo.Token == "" && (len(authInfo.ClientCertificateData) == 0 || len(authInfo.ClientKeyData) == 0) {
		return nil, errors.New("missing credentials")
	}
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["sparkops"] = &clientcmdapi.Cluster{
		Server:                   strings.TrimSpace(restConfig.Host),
		CertificateAuthorityData: restConfig.TLSClientConfig.CAData,
	}
	cfg.AuthInfos["sparkops"] = authInfo
	cfg.Contexts["sparkops"] = &clientcmdapi.Context{Cluster: "sparkops", AuthInfo: "sparkops", Namespace: namespace}
	cfg.CurrentContext = "sparkops"
	return clientcmd.Write(*cfg)
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
	client, dynamicClient, restCfg, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	session, err := h.provisioner.ProvisionCloudTTY(c.Request.Context(), client, dynamicClient, restCfg, req)
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
	case errors.Is(err, ErrCloudTTYTargetInvalid), errors.Is(err, ErrNodeTTYTargetInvalid), errors.Is(err, ErrCloudTTYCredentialConfigInvalid):
		httpx.Fail(c, "PLUGIN_KUBERNETES_TTY_TARGET_INVALID", "invalid tty target", http.StatusBadRequest)
	case errors.Is(err, ErrCloudTTYTargetNotFound), errors.Is(err, ErrNodeTTYTargetNotFound):
		httpx.Fail(c, "PLUGIN_KUBERNETES_TTY_TARGET_NOT_FOUND", "tty target not found", http.StatusNotFound)
	case errors.Is(err, ErrCloudTTYCRDUnavailable):
		httpx.Fail(c, "PLUGIN_CLOUDTTY_CRD_UNAVAILABLE", "CloudTTY CRD is unavailable", http.StatusBadGateway)
	case errors.Is(err, ErrNodeTTYNamespaceUnavailable):
		httpx.Fail(c, "PLUGIN_KUBERNETES_TTY_NAMESPACE_FORBIDDEN", "tty namespace is not writable", http.StatusForbidden)
	case errors.Is(err, ErrCloudTTYSecretCreateFailed), errors.Is(err, ErrCloudTTYCloudShellCreateFailed), errors.Is(err, ErrCloudTTYOwnerPatchFailed), errors.Is(err, ErrNodeTTYPodCreateFailed):
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

func cleanupContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), cleanupTimeout)
}

func deleteSecretWithCleanupContext(client kubernetes.Interface, namespace, name string) {
	ctx, cancel := cleanupContext()
	defer cancel()
	_ = deleteSecretNow(ctx, client, namespace, name)
}

func deleteCloudShellWithCleanupContext(dynamicClient dynamic.Interface, namespace, name string) {
	ctx, cancel := cleanupContext()
	defer cancel()
	_ = deleteCloudShellNow(ctx, dynamicClient, namespace, name)
}

func deleteSecretNow(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	grace := int64(0)
	propagation := metav1.DeletePropagationBackground
	return client.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: &grace,
		PropagationPolicy:  &propagation,
	})
}

func deleteCloudShellNow(ctx context.Context, dynamicClient dynamic.Interface, namespace, name string) error {
	grace := int64(0)
	propagation := metav1.DeletePropagationBackground
	return dynamicClient.Resource(cloudShellGVR()).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{
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
