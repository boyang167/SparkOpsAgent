package command

import (
	"context"

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
	if kind != "" {
		current.Kind = kind
	}
	if name != "" {
		current.Name = name
	}
	current.WhetherShare = whetherShare
	current.Config = config
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

func (h *CreateClusterHandler) Handle(ctx context.Context, memberID kernel.MemberID, clusterName, server, userName, caData, certData, keyData string, isBuildEnv bool, priorityID int) (*repository.ClusterInfo, error) {
	cluster := &repository.ClusterInfo{
		ID: kernel.NewClusterInfoID(), ClusterName: clusterName, Server: server, UserName: userName,
		CertificateAuthorityData: caData, ClientCertificateData: certData, ClientKeyData: keyData,
		IsBuildEnv: isBuildEnv, PriorityID: priorityID, CreatedBy: memberID,
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
	return h.repo.GrantWorkspaceCluster(ctx, wsID, clusterID, memberID)
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
