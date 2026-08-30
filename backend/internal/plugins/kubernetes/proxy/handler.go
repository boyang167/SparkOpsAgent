package proxy

import (
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"k8s.io/client-go/rest"
)

type Handler struct {
	ctx      sparkplugin.PluginContext
	discover *discoveryCache
}

func NewHandler(ctx sparkplugin.PluginContext) *Handler {
	return &Handler{ctx: ctx, discover: defaultDiscoveryCache}
}

func (h *Handler) ServeGIN(c *gin.Context) {
	if h == nil || h.ctx.Clusters == nil {
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
	apiPath, err := cleanAPIPath(c.Param("path"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_PROXY_PATH_INVALID", "invalid kubernetes api path", http.StatusBadRequest)
		return
	}
	if !isReadMethod(c.Request.Method) || isCoreSecretPath(apiPath) {
		if h.ctx.Writer == nil {
			httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
			return
		}
		if _, _, ok := requireProxyWrite(c.Request.Context(), c, h.ctx.Writer); !ok {
			return
		}
	}

	_, _, restCfg, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleProxyClusterError(c, err)
		return
	}
	if restCfg == nil || strings.TrimSpace(restCfg.Host) == "" {
		httpx.Fail(c, "PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE", "cluster credentials are incomplete", http.StatusServiceUnavailable)
		return
	}
	if !isWatch(c.Request.URL.Query()) {
		apiPath = h.compatiblePath(clusterID, restCfg, apiPath)
	}
	if shouldAggregate(c.Request.Method, c.Request.URL.Query()) {
		h.aggregateList(c, clusterID, restCfg, apiPath)
		return
	}
	h.reverseProxy(c, restCfg, apiPath)
}

func (h *Handler) compatiblePath(clusterID kernel.ClusterInfoID, restCfg *rest.Config, apiPath string) string {
	if !isCompatibilityCandidate(apiPath) {
		return apiPath
	}
	minor, ok := h.discover.versionMinor(clusterID, restCfg)
	if !ok {
		return apiPath
	}
	return CompatibleAPIPath(minor, apiPath)
}

func (h *Handler) reverseProxy(c *gin.Context, restCfg *rest.Config, apiPath string) {
	transport, target, err := transportAndTarget(restCfg)
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE", "cluster credentials are incomplete", http.StatusServiceUnavailable)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = transport
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = apiPath
		req.URL.RawPath = ""
		req.URL.RawQuery = c.Request.URL.RawQuery
		req.Host = target.Host
		req.Header = sanitizedHeaders(c.Request.Header)
	}
	proxy.ModifyResponse = sanitizeErrorResponse
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "upstream request failed", http.StatusBadGateway)
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

func isCoreSecretPath(apiPath string) bool {
	parts := strings.Split(strings.Trim(apiPath, "/"), "/")
	if len(parts) < 3 || parts[0] != "api" || parts[1] != "v1" {
		return false
	}
	if parts[2] == "secrets" {
		return true
	}
	return len(parts) >= 5 && parts[2] == "namespaces" && parts[3] != "" && parts[4] == "secrets"
}

func handleProxyClusterError(c *gin.Context, err error) {
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
	httpx.Fail(c, "PLUGIN_KUBERNETES_PROXY_UPSTREAM_UNAVAILABLE", "cluster credentials are unavailable", http.StatusBadGateway)
}
