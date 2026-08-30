package files

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	maxFileBytes = 20 << 20
)

var errOutputTooLarge = errors.New("pod file output too large")

type ExecRequest struct {
	Namespace string
	Pod       string
	Container string
	Command   []string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
}

type PodExecutor interface {
	Exec(ctx context.Context, client kubernetes.Interface, restCfg *rest.Config, req ExecRequest) error
}

type SPDYExecutor struct{}

func (SPDYExecutor) Exec(ctx context.Context, client kubernetes.Interface, restCfg *rest.Config, req ExecRequest) error {
	if client == nil || restCfg == nil {
		return errors.New("kubernetes client unavailable")
	}
	restReq := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(req.Pod).
		Namespace(req.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: req.Container,
			Command:   append([]string(nil), req.Command...),
			Stdin:     req.Stdin != nil,
			Stdout:    req.Stdout != nil,
			Stderr:    req.Stderr != nil,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(restCfg, http.MethodPost, restReq.URL())
	if err != nil {
		return err
	}
	return executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  req.Stdin,
		Stdout: req.Stdout,
		Stderr: req.Stderr,
	})
}

type Handler struct {
	ctx      sparkplugin.PluginContext
	executor PodExecutor
}

func NewHandler(ctx sparkplugin.PluginContext) *Handler {
	return NewHandlerWithExecutor(ctx, SPDYExecutor{})
}

func NewHandlerWithExecutor(ctx sparkplugin.PluginContext, executor PodExecutor) *Handler {
	return &Handler{ctx: ctx, executor: executor}
}

func (h *Handler) List(c *gin.Context) {
	if c.Query("download") == "true" {
		h.Download(c)
		return
	}
	target, ok := h.readTarget(c, false)
	if !ok {
		return
	}
	stdout, ok := h.execBuffered(c, target, []string{"ls", "-la", "--", target.Path}, nil)
	if !ok {
		return
	}
	httpx.OK(c, gin.H{"output": stdout.String()})
}

func (h *Handler) Download(c *gin.Context) {
	target, ok := h.readTarget(c, false)
	if !ok {
		return
	}
	stdout, ok := h.execBuffered(c, target, []string{"cat", "--", target.Path}, nil)
	if !ok {
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", contentDisposition(target.Path))
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(stdout.Bytes())
}

func (h *Handler) Upload(c *gin.Context) {
	if !h.writeReady(c) {
		return
	}
	if _, _, ok := requireWrite(c.Request.Context(), c, h.ctx.Writer); !ok {
		return
	}
	target, ok := h.target(c, false)
	if !ok {
		return
	}
	archive, ok := uploadArchive(c)
	if !ok {
		return
	}
	if _, ok := h.execBuffered(c, target, []string{"tar", "-xmf", "-", "-C", target.Path}, bytes.NewReader(archive)); !ok {
		return
	}
	httpx.OK(c, gin.H{"ok": true})
}

func (h *Handler) Delete(c *gin.Context) {
	if !h.writeReady(c) {
		return
	}
	if _, _, ok := requireWrite(c.Request.Context(), c, h.ctx.Writer); !ok {
		return
	}
	target, ok := h.target(c, true)
	if !ok {
		return
	}
	if _, ok := h.execBuffered(c, target, []string{"rm", "-rf", "--", target.Path}, nil); !ok {
		return
	}
	httpx.OK(c, gin.H{"ok": true})
}

type fileTarget struct {
	WorkspaceID kernel.WorkspaceID
	ClusterID   kernel.ClusterInfoID
	Namespace   string
	Pod         string
	Container   string
	Path        string
}

func (h *Handler) readTarget(c *gin.Context, destructive bool) (fileTarget, bool) {
	if !h.readReady(c) {
		return fileTarget{}, false
	}
	return h.target(c, destructive)
}

func (h *Handler) target(c *gin.Context, destructive bool) (fileTarget, bool) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return fileTarget{}, false
	}
	if _, ok := authn.MemberIDFromContext(c); !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return fileTarget{}, false
	}
	clusterID, err := kernel.ParseClusterInfoID(c.Param("clusterId"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_INVALID", "invalid cluster id", http.StatusBadRequest)
		return fileTarget{}, false
	}
	namespace := c.Param("namespace")
	pod := c.Param("pod")
	container := c.Query("container")
	if !validDNSLabel(namespace) || !validDNSSubdomain(pod) || (container != "" && !validDNSLabel(container)) {
		httpx.Fail(c, "PLUGIN_KUBERNETES_FILE_TARGET_INVALID", "invalid pod file target", http.StatusBadRequest)
		return fileTarget{}, false
	}
	filePath, ok := queryPath(c, destructive)
	if !ok {
		return fileTarget{}, false
	}
	return fileTarget{
		WorkspaceID: wsID,
		ClusterID:   clusterID,
		Namespace:   namespace,
		Pod:         pod,
		Container:   container,
		Path:        filePath,
	}, true
}

func (h *Handler) execBuffered(c *gin.Context, target fileTarget, command []string, stdin io.Reader) (*limitedBuffer, bool) {
	client, _, restCfg, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), target.WorkspaceID, target.ClusterID)
	if err != nil {
		handleClusterError(c, err)
		return nil, false
	}
	if restCfg == nil || strings.TrimSpace(restCfg.Host) == "" {
		httpx.Fail(c, "PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE", "cluster credentials are incomplete", http.StatusServiceUnavailable)
		return nil, false
	}
	stdout := &limitedBuffer{limit: maxFileBytes}
	err = h.executor.Exec(c.Request.Context(), client, restCfg, ExecRequest{
		Namespace: target.Namespace,
		Pod:       target.Pod,
		Container: target.Container,
		Command:   append([]string(nil), command...),
		Stdin:     stdin,
		Stdout:    stdout,
		Stderr:    io.Discard,
	})
	if errors.Is(err, errOutputTooLarge) || errors.Is(stdout.err, errOutputTooLarge) {
		httpx.Fail(c, "PLUGIN_KUBERNETES_FILE_OUTPUT_TOO_LARGE", "pod file output exceeds 20MiB", http.StatusBadGateway)
		return nil, false
	}
	if err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_FILE_EXEC_FAILED", "pod file command failed", http.StatusBadGateway)
		return nil, false
	}
	return stdout, true
}

func (h *Handler) readReady(c *gin.Context) bool {
	if h == nil || h.ctx.Clusters == nil || h.executor == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return false
	}
	return true
}

func (h *Handler) writeReady(c *gin.Context) bool {
	if !h.readReady(c) {
		return false
	}
	if h.ctx.Writer == nil {
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

func queryPath(c *gin.Context, destructive bool) (string, bool) {
	raw, exists := c.GetQuery("path")
	if !exists {
		httpx.Fail(c, "PLUGIN_KUBERNETES_FILE_PATH_INVALID", "invalid pod file path", http.StatusBadRequest)
		return "", false
	}
	if err := validateContainerPath(raw, destructive); err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_FILE_PATH_INVALID", "invalid pod file path", http.StatusBadRequest)
		return "", false
	}
	return raw, true
}

func validateContainerPath(value string, destructive bool) error {
	if value == "" || strings.ContainsRune(value, 0) || strings.Contains(value, "\\") {
		return errors.New("invalid path")
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return errors.New("invalid path")
		}
	}
	if destructive {
		cleaned := path.Clean(value)
		if cleaned == "/" || cleaned == "." {
			return errors.New("invalid path")
		}
	}
	return nil
}

func validDNSLabel(value string) bool {
	return len(validation.IsDNS1123Label(value)) == 0
}

func validDNSSubdomain(value string) bool {
	return len(validation.IsDNS1123Subdomain(value)) == 0
}

func uploadArchive(c *gin.Context) ([]byte, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFileBytes)
	if err := c.Request.ParseMultipartForm(maxFileBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httpx.Fail(c, "PLUGIN_KUBERNETES_FILE_UPLOAD_TOO_LARGE", "pod file upload exceeds 20MiB", http.StatusRequestEntityTooLarge)
			return nil, false
		}
		httpx.Fail(c, "PLUGIN_KUBERNETES_FILE_UPLOAD_INVALID", "invalid pod file upload", http.StatusBadRequest)
		return nil, false
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_FILE_UPLOAD_INVALID", "invalid pod file upload", http.StatusBadRequest)
		return nil, false
	}
	defer func() { _ = file.Close() }()

	content, err := readUploadFile(file)
	if err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_FILE_UPLOAD_TOO_LARGE", "pod file upload exceeds 20MiB", http.StatusRequestEntityTooLarge)
		return nil, false
	}
	name, err := safeUploadName(header)
	if err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_FILE_UPLOAD_INVALID", "invalid pod file upload", http.StatusBadRequest)
		return nil, false
	}
	archive, err := tarUpload(name, content)
	if err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_FILE_UPLOAD_INVALID", "invalid pod file upload", http.StatusBadRequest)
		return nil, false
	}
	return archive, true
}

func readUploadFile(file multipart.File) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxFileBytes {
		return nil, errors.New("upload too large")
	}
	return content, nil
}

func safeUploadName(header *multipart.FileHeader) (string, error) {
	if header == nil {
		return "", errors.New("missing file")
	}
	name := path.Base(header.Filename)
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, 0) ||
		strings.ContainsAny(name, `/\`) {
		return "", errors.New("invalid filename")
	}
	return name, nil
}

func tarUpload(name string, content []byte) ([]byte, error) {
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	if err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Mode:     0o600,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		return nil, err
	}
	if _, err := tw.Write(content); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return archive.Bytes(), nil
}

func contentDisposition(filePath string) string {
	name := path.Base(filePath)
	if name == "" || name == "." || name == "/" {
		name = "download"
	}
	ascii := safeDispositionFilename(name)
	return `attachment; filename="` + ascii + `"; filename*=UTF-8''` + url.PathEscape(name)
}

func safeDispositionFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '"' || r == '\\' || r == '/' || r < 0x20 || r == 0x7f:
			b.WriteByte('_')
		case r > 0x7e:
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "download"
	}
	return out
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
	httpx.Fail(c, "PLUGIN_KUBERNETES_FILE_UPSTREAM_UNAVAILABLE", "cluster credentials are unavailable", http.StatusBadGateway)
}

type limitedBuffer struct {
	bytes.Buffer
	limit int
	err   error
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	if b.Buffer.Len()+len(p) > b.limit {
		b.err = errOutputTooLarge
		return 0, b.err
	}
	return b.Buffer.Write(p)
}
