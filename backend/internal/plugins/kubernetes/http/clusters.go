package http

import (
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type clusterHandler struct {
	ctx   sparkplugin.PluginContext
	store settingsStore
}

func newClusterHandler(ctx sparkplugin.PluginContext) clusterHandler {
	return clusterHandler{ctx: ctx, store: newSettingsStore(ctx.DB)}
}

type clusterListResponse struct {
	Items []clusterView `json:"items"`
}

type clusterView struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Server             string `json:"server"`
	PrometheusURL      string `json:"prometheus_url"`
	PrometheusUser     string `json:"prometheus_user"`
	PrometheusPassword string `json:"prometheus_password"`
	VelauxURL          string `json:"velaux_url"`
	VelauxUser         string `json:"velaux_user"`
	VelauxPassword     string `json:"velaux_password"`
	LLMBaseURL         string `json:"llm_base_url"`
	LLMAPIKey          string `json:"llm_api_key"`
	LLMModel           string `json:"llm_model"`
}

func (h clusterHandler) listClusters(c *gin.Context) {
	if !h.readReady(c) {
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", stdhttp.StatusBadRequest)
		return
	}
	metas, err := h.ctx.Clusters.ListMeta(c.Request.Context(), wsID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	settings, err := h.store.List(c.Request.Context(), wsID, clusterIDs(metas))
	if err != nil {
		handleClusterError(c, err)
		return
	}
	items := make([]clusterView, 0, len(metas))
	for _, meta := range metas {
		view := viewFromMeta(meta)
		if s, ok := settings[meta.ID]; ok {
			applySettings(&view, s)
		}
		maskViewSecrets(&view)
		items = append(items, view)
	}
	httpx.OK(c, clusterListResponse{Items: items})
}

func (h clusterHandler) putClusterSettings(c *gin.Context) {
	if !h.writeReady(c) {
		return
	}
	wsID, _, ok := requireWrite(c.Request.Context(), c, h.ctx.Writer)
	if !ok {
		return
	}
	clusterID, err := kernel.ParseClusterInfoID(c.Param("clusterId"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_INVALID", "invalid cluster id", stdhttp.StatusBadRequest)
		return
	}
	var req clusterSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_SETTINGS_INVALID", "invalid settings payload", stdhttp.StatusBadRequest)
		return
	}
	if err := validateSettingsURLs(c.Request.Context(), h.ctx.Endpoints, req); err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_SETTINGS_INVALID_URL", err.Error(), stdhttp.StatusBadRequest)
		return
	}
	meta, err := h.authorizedMeta(c, wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	settings, err := h.store.Upsert(c.Request.Context(), wsID, clusterID, req)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	view := viewFromMeta(meta)
	applySettings(&view, settings)
	maskViewSecrets(&view)
	httpx.OK(c, view)
}

func (h clusterHandler) readReady(c *gin.Context) bool {
	if h.ctx.DB == nil || h.ctx.Clusters == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", stdhttp.StatusServiceUnavailable)
		return false
	}
	return true
}

func (h clusterHandler) writeReady(c *gin.Context) bool {
	if !h.readReady(c) {
		return false
	}
	if h.ctx.Writer == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", stdhttp.StatusServiceUnavailable)
		return false
	}
	return true
}

func (h clusterHandler) authorizedMeta(c *gin.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (sparkplugin.ClusterMeta, error) {
	metas, err := h.ctx.Clusters.ListMeta(c.Request.Context(), wsID)
	if err != nil {
		return sparkplugin.ClusterMeta{}, err
	}
	for _, meta := range metas {
		if meta.ID == clusterID.String() {
			return meta, nil
		}
	}
	return sparkplugin.ClusterMeta{}, kernel.NewDomainError("PLUGIN_CLUSTER_FORBIDDEN", "cluster is not granted to workspace")
}

func viewFromMeta(meta sparkplugin.ClusterMeta) clusterView {
	return clusterView{
		ID:                 meta.ID,
		Name:               meta.Name,
		Server:             meta.Server,
		PrometheusURL:      meta.PrometheusURL,
		PrometheusUser:     meta.PrometheusUser,
		PrometheusPassword: meta.PrometheusPassword,
		VelauxURL:          meta.VelauxURL,
		VelauxUser:         meta.VelauxUser,
		VelauxPassword:     meta.VelauxPassword,
		LLMBaseURL:         meta.LLMBaseURL,
		LLMAPIKey:          meta.LLMAPIKey,
		LLMModel:           meta.LLMModel,
	}
}

func applySettings(view *clusterView, settings k8sschema.ClusterSettings) {
	view.PrometheusURL = settings.PrometheusURL
	view.PrometheusUser = settings.PrometheusUser
	view.PrometheusPassword = settings.PrometheusPassword
	view.VelauxURL = settings.VelauxURL
	view.VelauxUser = settings.VelauxUser
	view.VelauxPassword = settings.VelauxPassword
	view.LLMBaseURL = settings.LLMBaseURL
	view.LLMAPIKey = settings.LLMAPIKey
	view.LLMModel = settings.LLMModel
}

func maskViewSecrets(view *clusterView) {
	view.PrometheusPassword = maskSecret(view.PrometheusPassword)
	view.VelauxPassword = maskSecret(view.VelauxPassword)
	view.LLMAPIKey = maskSecret(view.LLMAPIKey)
}

func handleClusterError(c *gin.Context, err error) {
	if de, ok := kernel.IsDomainError(err); ok {
		switch de.Code() {
		case "PLUGIN_CLUSTER_FORBIDDEN":
			httpx.Fail(c, de.Code(), de.Error(), stdhttp.StatusForbidden)
			return
		case "PLUGIN_CLUSTER_SETTINGS_CONFLICT":
			httpx.Fail(c, de.Code(), de.Error(), stdhttp.StatusConflict)
			return
		}
	}
	httpx.HandleError(c, err)
}
