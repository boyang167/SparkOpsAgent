package velaux

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

const MaxResponseBodyBytes int64 = 16 << 20

const streamBufferBytes = 32 << 10

var percentEscape = regexp.MustCompile(`%[0-9a-fA-F]{2}`)

var allowedRequestHeaders = map[string]bool{
	"Accept":              true,
	"Accept-Encoding":     true,
	"Accept-Language":     true,
	"If-Match":            true,
	"If-Modified-Since":   true,
	"If-None-Match":       true,
	"If-Unmodified-Since": true,
	"Content-Type":        true,
}

type Handler struct {
	ctx    sparkplugin.PluginContext
	client *http.Client
}

func NewHandler(ctx sparkplugin.PluginContext) *Handler {
	return NewHandlerWithClient(ctx, nil)
}

func NewHandlerWithClient(ctx sparkplugin.PluginContext, client *http.Client) *Handler {
	if client == nil {
		client = &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &Handler{ctx: ctx, client: client}
}

func (h *Handler) ServeGIN(c *gin.Context) {
	if h == nil || h.ctx.DB == nil || h.ctx.Clusters == nil || h.client == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	if _, ok := authn.MemberIDFromContext(c); !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	clusterID, err := kernel.ParseClusterInfoID(c.Param("clusterId"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_INVALID", "invalid cluster id", http.StatusBadRequest)
		return
	}
	if !isReadMethod(c.Request.Method) {
		if h.ctx.Writer == nil {
			httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
			return
		}
		if _, _, ok := requireWrite(c.Request.Context(), c, h.ctx.Writer); !ok {
			return
		}
	}
	targetPath, err := cleanVelauxPath(c.Param("path"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_VELAUX_PATH_INVALID", "invalid velaux path", http.StatusBadRequest)
		return
	}
	if err := h.authorizeCluster(c.Request.Context(), wsID, clusterID); err != nil {
		handleClusterError(c, err)
		return
	}
	settings, err := h.settings(c.Request.Context(), wsID, clusterID)
	if err != nil {
		httpx.Fail(c, "PLUGIN_VELAUX_NOT_CONFIGURED", "velaux url is not configured", http.StatusBadRequest)
		return
	}
	endpoint, err := buildVelauxURL(settings.VelauxURL, targetPath, c.Request.URL.RawQuery)
	if err != nil {
		httpx.Fail(c, "PLUGIN_VELAUX_NOT_CONFIGURED", "velaux url is not configured", http.StatusBadRequest)
		return
	}
	if h.ctx.Endpoints == nil {
		httpx.Fail(c, "PLUGIN_VELAUX_NOT_CONFIGURED", "velaux url is not configured", http.StatusBadRequest)
		return
	}
	client, err := h.ctx.Endpoints.PinnedClient(c.Request.Context(), endpoint, h.client)
	if err != nil {
		httpx.Fail(c, "PLUGIN_VELAUX_NOT_CONFIGURED", "velaux url is not configured", http.StatusBadRequest)
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, endpoint, http.MaxBytesReader(c.Writer, c.Request.Body, MaxResponseBodyBytes))
	if err != nil {
		httpx.Fail(c, "PLUGIN_VELAUX_UPSTREAM_UNAVAILABLE", "velaux upstream unavailable", http.StatusBadGateway)
		return
	}
	req.Header = allowHeaders(c.Request.Header)
	if settings.VelauxUser != "" || settings.VelauxPassword != "" {
		req.SetBasicAuth(settings.VelauxUser, settings.VelauxPassword)
	}
	resp, err := client.Do(req)
	if err != nil {
		httpx.Fail(c, "PLUGIN_VELAUX_UPSTREAM_UNAVAILABLE", "velaux upstream unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		httpx.Fail(c, "PLUGIN_VELAUX_UPSTREAM_UNAVAILABLE", "velaux upstream unavailable", http.StatusBadGateway)
		return
	}
	if c.Request.Method == http.MethodHead {
		copyResponseHeaders(c.Writer.Header(), resp.Header)
		c.Status(resp.StatusCode)
		return
	}
	if isStreamResponse(resp) {
		copyResponseHeaders(c.Writer.Header(), resp.Header)
		c.Status(resp.StatusCode)
		h.copyStream(c, resp.Body)
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBodyBytes+1))
	if err != nil || int64(len(body)) > MaxResponseBodyBytes {
		httpx.Fail(c, "PLUGIN_VELAUX_UPSTREAM_UNAVAILABLE", "velaux upstream unavailable", http.StatusBadGateway)
		return
	}
	copyResponseHeaders(c.Writer.Header(), resp.Header)
	c.Status(resp.StatusCode)
	_, _ = c.Writer.Write(body)
}

func (h *Handler) copyStream(c *gin.Context, body io.Reader) {
	buf := make([]byte, streamBufferBytes)
	flusher, _ := c.Writer.(http.Flusher)
	for {
		if err := c.Request.Context().Err(); err != nil {
			return
		}
		n, readErr := body.Read(buf)
		if n > 0 {
			if _, err := c.Writer.Write(buf[:n]); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			return
		}
	}
}

func (h *Handler) authorizeCluster(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) error {
	metas, err := h.ctx.Clusters.ListMeta(ctx, wsID)
	if err != nil {
		return err
	}
	for _, meta := range metas {
		if meta.ID == clusterID.String() {
			return nil
		}
	}
	return kernel.NewDomainError("PLUGIN_CLUSTER_FORBIDDEN", "cluster is not granted to workspace")
}

func (h *Handler) settings(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (k8sschema.ClusterSettings, error) {
	var settings k8sschema.ClusterSettings
	err := h.ctx.DB.WithContext(ctx).First(&settings, "workspace_id = ? AND cluster_id = ?", wsID.String(), clusterID.String()).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || strings.TrimSpace(settings.VelauxURL) == "" {
		return k8sschema.ClusterSettings{}, gorm.ErrRecordNotFound
	}
	return settings, err
}

func buildVelauxURL(baseURL, targetPath, rawQuery string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return "", errors.New("invalid velaux url")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("invalid velaux url")
	}
	parsed.Path = path.Join(parsed.Path, targetPath)
	if strings.HasSuffix(targetPath, "/") && !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	parsed.RawPath = ""
	parsed.RawQuery = rawQuery
	parsed.Fragment = ""
	parsed.User = nil
	return parsed.String(), nil
}

func cleanVelauxPath(raw string) (string, error) {
	if raw == "" || raw == "/" {
		return "/", nil
	}
	current := raw
	for i := 0; i < 3; i++ {
		if err := validatePathLayer(current); err != nil {
			return "", err
		}
		decoded, err := url.PathUnescape(current)
		if err != nil {
			return "", err
		}
		if decoded == current {
			break
		}
		current = decoded
	}
	if percentEscape.MatchString(current) {
		return "", errors.New("unsafe velaux path")
	}
	if err := validatePathLayer(current); err != nil {
		return "", err
	}
	return path.Clean("/" + strings.TrimPrefix(current, "/")), nil
}

func validatePathLayer(value string) error {
	if strings.Contains(value, "\x00") || strings.Contains(value, "\\") ||
		strings.HasPrefix(value, "//") || strings.Contains(value, "://") {
		return errors.New("unsafe velaux path")
	}
	for _, segment := range strings.Split(strings.Trim(value, "/"), "/") {
		if segment == "." || segment == ".." {
			return errors.New("unsafe velaux path")
		}
	}
	return nil
}

func allowHeaders(in http.Header) http.Header {
	out := http.Header{}
	for key, values := range in {
		canonical := http.CanonicalHeaderKey(key)
		if !allowedRequestHeaders[canonical] {
			continue
		}
		for _, value := range values {
			out.Add(canonical, value)
		}
	}
	return out
}

func copyResponseHeaders(out, in http.Header) {
	for _, key := range []string{"Content-Type", "Cache-Control", "ETag", "Last-Modified"} {
		for _, value := range in.Values(key) {
			out.Add(key, value)
		}
	}
}

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

func isStreamResponse(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	return err == nil && strings.EqualFold(mediaType, "text/event-stream")
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

func handleClusterError(c *gin.Context, err error) {
	if de, ok := kernel.IsDomainError(err); ok && de.Code() == "PLUGIN_CLUSTER_FORBIDDEN" {
		httpx.Fail(c, de.Code(), "cluster is not granted to workspace", http.StatusForbidden)
		return
	}
	httpx.Fail(c, "PLUGIN_VELAUX_UPSTREAM_UNAVAILABLE", "velaux upstream unavailable", http.StatusBadGateway)
}
