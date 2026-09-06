package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/observability"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	getSysConfig             *query.GetSysConfigHandler
	updateSysConfig          *command.UpdateSysConfigHandler
	listClusters             *query.ListClustersHandler
	createCluster            *command.CreateClusterHandler
	registerWorkspaceCluster *command.RegisterWorkspaceClusterHandler
	listWorkspaceClusters    *query.ListWorkspaceClustersHandler
	grantWorkspaceCluster    *command.GrantWorkspaceClusterHandler
	revokeWorkspaceCluster   *command.RevokeWorkspaceClusterHandler
	reorderWorkspaceClusters *command.ReorderWorkspaceClustersHandler
	getDashboard             *query.GetDashboardHandler
	getStatistics            *query.GetStatisticsHandler
	getMonitorOverview       *query.GetMonitorOverviewHandler
	listDeploymentStats      *query.ListDeploymentStatsHandler
	listTools                *query.ListToolsHandler
	listProclaims            *query.ListProclaimsHandler
	createProclaim           *command.CreateProclaimHandler
	cloneTool                *command.CloneToolHandler
	getWorkspaceEntitlements *query.GetWorkspaceEntitlementsHandler
}

func NewHandler(
	getSysConfig *query.GetSysConfigHandler,
	updateSysConfig *command.UpdateSysConfigHandler,
	listClusters *query.ListClustersHandler,
	createCluster *command.CreateClusterHandler,
	registerWorkspaceCluster *command.RegisterWorkspaceClusterHandler,
	listWorkspaceClusters *query.ListWorkspaceClustersHandler,
	grantWorkspaceCluster *command.GrantWorkspaceClusterHandler,
	revokeWorkspaceCluster *command.RevokeWorkspaceClusterHandler,
	reorderWorkspaceClusters *command.ReorderWorkspaceClustersHandler,
	getDashboard *query.GetDashboardHandler,
	getStatistics *query.GetStatisticsHandler,
	getMonitorOverview *query.GetMonitorOverviewHandler,
	listDeploymentStats *query.ListDeploymentStatsHandler,
	listTools *query.ListToolsHandler,
	listProclaims *query.ListProclaimsHandler,
	createProclaim *command.CreateProclaimHandler,
	cloneTool *command.CloneToolHandler,
	getWorkspaceEntitlements *query.GetWorkspaceEntitlementsHandler,
) *Handler {
	return &Handler{
		getSysConfig: getSysConfig, updateSysConfig: updateSysConfig,
		listClusters: listClusters, createCluster: createCluster,
		registerWorkspaceCluster: registerWorkspaceCluster,
		listWorkspaceClusters:    listWorkspaceClusters,
		grantWorkspaceCluster:  grantWorkspaceCluster,
		revokeWorkspaceCluster: revokeWorkspaceCluster,
		reorderWorkspaceClusters: reorderWorkspaceClusters,
		getDashboard:           getDashboard, getStatistics: getStatistics,
		getMonitorOverview: getMonitorOverview, listDeploymentStats: listDeploymentStats,
		listTools:     listTools,
		listProclaims: listProclaims, createProclaim: createProclaim,
		cloneTool:                cloneTool,
		getWorkspaceEntitlements: getWorkspaceEntitlements,
	}
}

type updateSysConfigRequest struct {
	Kind         string         `json:"kind"`
	Name         string         `json:"name"`
	WhetherShare bool           `json:"whether_share"`
	Config       map[string]any `json:"config"`
}

type createClusterRequest struct {
	ClusterName              string `json:"cluster_name" binding:"required"`
	Server                   string `json:"server"`
	UserName                 string `json:"user_name"`
	CertificateAuthorityData string `json:"certificate_authority_data"`
	ClientCertificateData    string `json:"client_certificate_data"`
	ClientKeyData            string `json:"client_key_data"`
	BearerToken              string `json:"bearer_token"`
	KubeType                 int    `json:"kube_type"`
	KubeConfig               string `json:"kube_config"`
	APIAddress               string `json:"api_address"`
	IsBuildEnv               bool   `json:"is_build_env"`
	PriorityID               int    `json:"priority_id"`
}

type createProclaimRequest struct {
	Title           string `json:"title" binding:"required"`
	Body            string `json:"body"`
	ProclaimType    string `json:"proclaim_type"`
	ProclaimReceive string `json:"proclaim_receive"`
}

type cloneToolRequest struct {
	SourceURL  string `json:"source_url" binding:"required"`
	TargetPath string `json:"target_path"`
}

func (h *Handler) GetSysConfig(c *gin.Context) {
	item, err := h.getSysConfig.Handle(c.Request.Context())
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) UpdateSysConfig(c *gin.Context) {
	var req updateSysConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.updateSysConfig.Handle(c.Request.Context(), req.Kind, req.Name, req.WhetherShare, req.Config)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": item.ID, "kind": item.Kind, "name": item.Name, "config": item.Config})
}

func (h *Handler) ListClusters(c *gin.Context) {
	items, err := h.listClusters.Handle(c.Request.Context())
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetWorkspaceEntitlements(c *gin.Context) {
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	item, err := h.getWorkspaceEntitlements.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) CreateCluster(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	var req createClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	cluster, err := h.createCluster.Handle(c.Request.Context(), memberID, req.ClusterName, req.Server, req.UserName, req.CertificateAuthorityData, req.ClientCertificateData, req.ClientKeyData, req.BearerToken, req.IsBuildEnv, req.PriorityID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": cluster.ID.String(), "cluster_name": cluster.ClusterName})
}

func (h *Handler) RegisterWorkspaceCluster(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	var req createClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	var creds *command.ParsedClusterCredentials
	if strings.TrimSpace(req.KubeConfig) != "" || strings.TrimSpace(req.BearerToken) != "" {
		parsed, err := command.ParseClusterCredentials(command.ClusterCredentialInput{
			KubeType:   req.KubeType,
			KubeConfig: firstNonEmpty(req.KubeConfig, req.BearerToken),
			APIAddress: firstNonEmpty(req.APIAddress, req.Server),
			UserName:   req.UserName,
		})
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
			return
		}
		creds = parsed
	} else if strings.TrimSpace(req.CertificateAuthorityData) != "" || strings.TrimSpace(req.ClientCertificateData) != "" || strings.TrimSpace(req.BearerToken) != "" {
		creds = &command.ParsedClusterCredentials{
			Server:                   firstNonEmpty(req.APIAddress, req.Server),
			UserName:                 req.UserName,
			CertificateAuthorityData: req.CertificateAuthorityData,
			ClientCertificateData:    req.ClientCertificateData,
			ClientKeyData:            req.ClientKeyData,
			BearerToken:              req.BearerToken,
		}
	}
	cluster, err := h.registerWorkspaceCluster.Handle(
		c.Request.Context(), memberID, wsID, req.ClusterName, firstNonEmpty(req.APIAddress, req.Server), creds,
	)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id":           cluster.ID.String(),
		"cluster_name": cluster.ClusterName,
		"server":       cluster.Server,
		"workspace_id": wsID.String(),
	})
}

func (h *Handler) ListWorkspaceClusters(c *gin.Context) {
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	items, err := h.listWorkspaceClusters.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GrantWorkspaceCluster(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	var req struct {
		ClusterID string `json:"cluster_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	clusterID, err := kernel.ParseClusterInfoID(req.ClusterID)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid cluster id", http.StatusBadRequest)
		return
	}
	if err := h.grantWorkspaceCluster.Handle(c.Request.Context(), memberID, wsID, clusterID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"granted": true, "workspace_id": wsID.String(), "cluster_id": clusterID.String()})
}

func (h *Handler) RevokeWorkspaceCluster(c *gin.Context) {
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	clusterID, err := kernel.ParseClusterInfoID(c.Param("clusterId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid cluster id", http.StatusBadRequest)
		return
	}
	if err := h.revokeWorkspaceCluster.Handle(c.Request.Context(), wsID, clusterID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"revoked": true, "workspace_id": wsID.String(), "cluster_id": clusterID.String()})
}

func (h *Handler) ReorderWorkspaceClusters(c *gin.Context) {
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	var req struct {
		ClusterIDs []string `json:"cluster_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	clusterIDs := make([]kernel.ClusterInfoID, 0, len(req.ClusterIDs))
	for _, raw := range req.ClusterIDs {
		clusterID, parseErr := kernel.ParseClusterInfoID(raw)
		if parseErr != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid cluster id", http.StatusBadRequest)
			return
		}
		clusterIDs = append(clusterIDs, clusterID)
	}
	if err := h.reorderWorkspaceClusters.Handle(c.Request.Context(), wsID, clusterIDs); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"reordered": true, "workspace_id": wsID.String()})
}

func (h *Handler) GetDashboard(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	item, err := h.getDashboard.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) GetStatistics(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	item, err := h.getStatistics.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) GetMonitorOverview(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	item, err := h.getMonitorOverview.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) ListDeploymentStats(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	items, err := h.listDeploymentStats.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) ListTools(c *gin.Context) {
	items := h.listTools.Handle(c.Request.Context())
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) ListProclaims(c *gin.Context) {
	items, err := h.listProclaims.Handle(c.Request.Context())
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) CreateProclaim(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	var req createProclaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	proclaim, err := h.createProclaim.Handle(c.Request.Context(), memberID, req.Title, req.Body, req.ProclaimType, req.ProclaimReceive)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": proclaim.ID.String(), "title": proclaim.Title})
}

func (h *Handler) CloneTool(c *gin.Context) {
	var req cloneToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	httpx.OK(c, h.cloneTool.Handle(c.Request.Context(), req.SourceURL, req.TargetPath))
}

func (h *Handler) GetServerStatus(c *gin.Context) {
	httpx.OK(c, observability.GetServerStatus())
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
