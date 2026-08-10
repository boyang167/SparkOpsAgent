package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type GetSysConfigHandler struct {
	repo repository.AdminRepository
}

func NewGetSysConfigHandler(repo repository.AdminRepository) *GetSysConfigHandler {
	return &GetSysConfigHandler{repo: repo}
}

func (h *GetSysConfigHandler) Handle(ctx context.Context) (map[string]any, error) {
	cfg, err := h.repo.GetSysConfig(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": cfg.ID, "kind": cfg.Kind, "name": cfg.Name,
		"whether_share": cfg.WhetherShare, "config": cfg.Config,
	}, nil
}

type ListClustersHandler struct {
	repo repository.AdminRepository
}

func NewListClustersHandler(repo repository.AdminRepository) *ListClustersHandler {
	return &ListClustersHandler{repo: repo}
}

func (h *ListClustersHandler) Handle(ctx context.Context) ([]map[string]any, error) {
	items, err := h.repo.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, c := range items {
		out = append(out, map[string]any{
			"id": c.ID.String(), "cluster_name": c.ClusterName, "server": c.Server,
			"user_name": c.UserName, "is_build_env": c.IsBuildEnv, "priority_id": c.PriorityID,
		})
	}
	return out, nil
}

type ListSyncJobsHandler struct {
	repo repository.AdminRepository
}

func NewListSyncJobsHandler(repo repository.AdminRepository) *ListSyncJobsHandler {
	return &ListSyncJobsHandler{repo: repo}
}

func (h *ListSyncJobsHandler) Handle(ctx context.Context) ([]map[string]any, error) {
	items, err := h.repo.ListSyncJobs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, job := range items {
		out = append(out, map[string]any{
			"id": job.ID.String(), "name": job.Name, "workspace_id": job.WorkspaceID.String(),
			"version": job.Version, "source_cluster": job.SourceCluster,
			"target_cluster": job.TargetCluster, "status": job.Status, "result": job.Result,
		})
	}
	return out, nil
}

type GetDashboardHandler struct {
	repo repository.AdminRepository
}

func NewGetDashboardHandler(repo repository.AdminRepository) *GetDashboardHandler {
	return &GetDashboardHandler{repo: repo}
}

func (h *GetDashboardHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID) (map[string]any, error) {
	projectCount, err := h.repo.CountProjectsByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	productCount, err := h.repo.CountProductsByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"workspace_id": wsID.String(),
		"projects":     projectCount,
		"products":     productCount,
		"summary":      "workplace dashboard",
	}, nil
}

type GetStatisticsHandler struct {
	repo repository.AdminRepository
}

func NewGetStatisticsHandler(repo repository.AdminRepository) *GetStatisticsHandler {
	return &GetStatisticsHandler{repo: repo}
}

func (h *GetStatisticsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID) (map[string]any, error) {
	cached, err := h.repo.GetStatisticsCache(ctx, wsID, "deployments")
	if err != nil {
		return nil, err
	}
	if cached != nil {
		return cached, nil
	}
	projectCount, err := h.repo.CountProjectsByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	stats := map[string]any{
		"workspace_id": wsID.String(),
		"deployments":  projectCount,
		"generated_at": "live",
	}
	_ = h.repo.UpsertStatisticsCache(ctx, wsID, "deployments", stats)
	return stats, nil
}

type GetMonitorOverviewHandler struct {
	repo repository.AdminRepository
}

func NewGetMonitorOverviewHandler(repo repository.AdminRepository) *GetMonitorOverviewHandler {
	return &GetMonitorOverviewHandler{repo: repo}
}

func (h *GetMonitorOverviewHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID) (map[string]any, error) {
	grafanaURL := ""
	prometheusURL := ""
	if cfg, err := h.repo.GetSysConfig(ctx); err == nil && cfg != nil && cfg.Config != nil {
		if v, ok := cfg.Config["grafana_url"].(string); ok {
			grafanaURL = v
		}
		if v, ok := cfg.Config["prometheus_url"].(string); ok {
			prometheusURL = v
		}
	}
	return map[string]any{
		"workspace_id":   wsID.String(),
		"grafana_url":    grafanaURL,
		"prometheus_url": prometheusURL,
		"panels":         []map[string]any{},
	}, nil
}

type ListDeploymentStatsHandler struct {
	repo repository.AdminRepository
}

func NewListDeploymentStatsHandler(repo repository.AdminRepository) *ListDeploymentStatsHandler {
	return &ListDeploymentStatsHandler{repo: repo}
}

func (h *ListDeploymentStatsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID) ([]map[string]any, error) {
	count, err := h.repo.CountProjectsByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	return []map[string]any{
		{"date": "live", "count": count, "project_name": "all"},
	}, nil
}

type ListToolsHandler struct{}

func NewListToolsHandler() *ListToolsHandler { return &ListToolsHandler{} }

func (h *ListToolsHandler) Handle(_ context.Context) []map[string]any {
	return []map[string]any{
		{"id": "clone", "name": "Git Clone", "description": "Clone repository to workspace", "path": "/admin/tools/clone"},
		{"id": "sync-db", "name": "DB Sync", "description": "Cross-cluster database sync", "path": "/admin/sync/jobs"},
		{"id": "sync-res", "name": "Resource Sync", "description": "Cross-cluster resource sync", "path": "/admin/sync/jobs"},
	}
}

type ListProclaimsHandler struct {
	repo repository.AdminRepository
}

func NewListProclaimsHandler(repo repository.AdminRepository) *ListProclaimsHandler {
	return &ListProclaimsHandler{repo: repo}
}

func (h *ListProclaimsHandler) Handle(ctx context.Context) ([]map[string]any, error) {
	items, err := h.repo.ListProclaims(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, p := range items {
		out = append(out, map[string]any{
			"id": p.ID.String(), "title": p.Title, "body": p.Body,
			"proclaim_type": p.ProclaimType, "proclaim_receive": p.ProclaimReceive,
		})
	}
	return out, nil
}
