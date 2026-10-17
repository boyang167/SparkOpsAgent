package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type UpdateSysConfigHandler struct {
	repo repository.AdminRepository
}

func NewUpdateSysConfigHandler(repo repository.AdminRepository) *UpdateSysConfigHandler {
	return &UpdateSysConfigHandler{repo: repo}
}

func (h *UpdateSysConfigHandler) Handle(ctx context.Context, kind, name string, whetherShare bool, config map[string]any) (*repository.SysConfig, error) {
	if config == nil {
		config = map[string]any{}
	}
	current, err := h.repo.GetSysConfig(ctx)
	if err != nil {
		return nil, err
	}
	nextConfig := make(map[string]any, len(config)+1)
	for key, value := range config {
		if key != "sso" {
			nextConfig[key] = value
		}
	}
	if current.Config != nil {
		if sso, ok := current.Config["sso"]; ok {
			nextConfig["sso"] = sso
		}
	}
	if kind != "" {
		current.Kind = kind
	}
	if name != "" {
		current.Name = name
	}
	current.WhetherShare = whetherShare
	current.Config = nextConfig
	if err := h.repo.UpsertSysConfig(ctx, current); err != nil {
		return nil, err
	}
	entitlements.InvalidateCache()
	return current, nil
}

type CreateClusterHandler struct {
	repo repository.AdminRepository
}

func NewCreateClusterHandler(repo repository.AdminRepository) *CreateClusterHandler {
	return &CreateClusterHandler{repo: repo}
}

func (h *CreateClusterHandler) Handle(ctx context.Context, memberID kernel.MemberID, clusterName, server, userName, caData, certData, keyData, bearerToken string, isBuildEnv bool, priorityID int) (*repository.ClusterInfo, error) {
	clusterName = strings.TrimSpace(clusterName)
	if clusterName == "" {
		return nil, domain.ErrClusterNameRequired
	}
	cluster := &repository.ClusterInfo{
		ID: kernel.NewClusterInfoID(), ClusterName: clusterName, Server: strings.TrimSpace(server), UserName: userName,
		CertificateAuthorityData: caData, ClientCertificateData: certData, ClientKeyData: keyData,
		BearerToken: strings.TrimSpace(bearerToken),
		IsBuildEnv:  isBuildEnv, PriorityID: priorityID, CreatedBy: memberID,
	}
	if err := h.repo.SaveCluster(ctx, cluster); err != nil {
		return nil, err
	}
	entitlements.InvalidateCache()
	return cluster, nil
}

type GrantWorkspaceClusterHandler struct {
	repo repository.AdminRepository
}

func NewGrantWorkspaceClusterHandler(repo repository.AdminRepository) *GrantWorkspaceClusterHandler {
	return &GrantWorkspaceClusterHandler{repo: repo}
}

func (h *GrantWorkspaceClusterHandler) Handle(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) error {
	if err := h.repo.GrantWorkspaceCluster(ctx, wsID, clusterID, memberID); err != nil {
		return err
	}
	entitlements.InvalidateCache()
	return nil
}

type ReorderWorkspaceClustersHandler struct {
	repo repository.AdminRepository
}

func NewReorderWorkspaceClustersHandler(repo repository.AdminRepository) *ReorderWorkspaceClustersHandler {
	return &ReorderWorkspaceClustersHandler{repo: repo}
}

func (h *ReorderWorkspaceClustersHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	clusterIDs []kernel.ClusterInfoID,
) error {
	if err := h.repo.ReorderWorkspaceClusters(ctx, wsID, clusterIDs); err != nil {
		return err
	}
	entitlements.InvalidateCache()
	return nil
}

type RegisterWorkspaceClusterHandler struct {
	repo   repository.AdminRepository
	create *CreateClusterHandler
	grant  *GrantWorkspaceClusterHandler
}

func NewRegisterWorkspaceClusterHandler(repo repository.AdminRepository) *RegisterWorkspaceClusterHandler {
	return &RegisterWorkspaceClusterHandler{
		repo:   repo,
		create: NewCreateClusterHandler(repo),
		grant:  NewGrantWorkspaceClusterHandler(repo),
	}
}

func (h *RegisterWorkspaceClusterHandler) Handle(
	ctx context.Context,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	clusterName, server string,
	creds *ParsedClusterCredentials,
) (*repository.ClusterInfo, error) {
	clusterName = strings.TrimSpace(clusterName)
	if clusterName == "" {
		return nil, domain.ErrClusterNameRequired
	}
	userName, caData, certData, keyData, bearerToken := "", "", "", "", ""
	if creds != nil {
		if strings.TrimSpace(server) == "" {
			server = creds.Server
		}
		userName = creds.UserName
		caData = creds.CertificateAuthorityData
		certData = creds.ClientCertificateData
		keyData = creds.ClientKeyData
		bearerToken = creds.BearerToken
	}
	server = strings.TrimSpace(server)

	existing, err := h.repo.FindClusterByName(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	var cluster *repository.ClusterInfo
	if existing != nil {
		cluster = existing
		if server != "" {
			cluster.Server = server
		}
		if userName != "" {
			cluster.UserName = userName
		}
		if caData != "" || certData != "" || keyData != "" || bearerToken != "" {
			cluster.CertificateAuthorityData = caData
			cluster.ClientCertificateData = certData
			cluster.ClientKeyData = keyData
			cluster.BearerToken = bearerToken
		}
		if err := h.repo.SaveCluster(ctx, cluster); err != nil {
			return nil, err
		}
		entitlements.InvalidateCache()
	} else {
		cluster, err = h.create.Handle(ctx, memberID, clusterName, server, userName, caData, certData, keyData, bearerToken, false, 0)
		if err != nil {
			return nil, err
		}
	}
	if err := h.grant.Handle(ctx, memberID, wsID, cluster.ID); err != nil {
		return nil, err
	}
	return cluster, nil
}

type RevokeWorkspaceClusterHandler struct {
	repo repository.AdminRepository
}

func NewRevokeWorkspaceClusterHandler(repo repository.AdminRepository) *RevokeWorkspaceClusterHandler {
	return &RevokeWorkspaceClusterHandler{repo: repo}
}

func (h *RevokeWorkspaceClusterHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) error {
	return h.repo.RevokeWorkspaceCluster(ctx, wsID, clusterID)
}

type CreateProclaimHandler struct {
	repo repository.AdminRepository
}

func NewCreateProclaimHandler(repo repository.AdminRepository) *CreateProclaimHandler {
	return &CreateProclaimHandler{repo: repo}
}

func (h *CreateProclaimHandler) Handle(ctx context.Context, memberID kernel.MemberID, title, body, proclaimType, proclaimReceive string) (*repository.Proclaim, error) {
	proclaim := &repository.Proclaim{
		ID: kernel.NewProclaimID(), Title: title, Body: body,
		ProclaimType: proclaimType, ProclaimReceive: proclaimReceive,
		Status: "active", CreatedBy: memberID,
	}
	if err := h.repo.SaveProclaim(ctx, proclaim); err != nil {
		return nil, err
	}
	return proclaim, nil
}

type CloneToolHandler struct{}

func NewCloneToolHandler() *CloneToolHandler { return &CloneToolHandler{} }

func (h *CloneToolHandler) Handle(_ context.Context, sourceURL, targetPath string) map[string]any {
	return map[string]any{
		"status":      "stub",
		"source_url":  sourceURL,
		"target_path": targetPath,
		"message":     "clone tool not implemented yet",
	}
}
