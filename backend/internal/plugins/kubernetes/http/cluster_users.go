package http

import (
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	k8srbac "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/rbac"
	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const serviceAccountTokenTTLSeconds int64 = 24 * 60 * 60

type tokenIssuerFunc func(ctx context.Context, client kubernetes.Interface, namespace, name string) (string, time.Time, error)

var issueServiceAccountToken tokenIssuerFunc = createServiceAccountToken

type clusterUserGrantRequest struct {
	MemberIDs      []string                `json:"member_ids"`
	ClusterRoles   []string                `json:"cluster_roles"`
	NamespaceRoles []k8srbac.NamespaceRole `json:"namespace_roles"`
}

type clusterUserGrantView struct {
	MemberID                string                  `json:"member_id"`
	Email                   string                  `json:"email"`
	DisplayName             string                  `json:"display_name"`
	ServiceAccountName      string                  `json:"service_account_name"`
	ServiceAccountNamespace string                  `json:"service_account_namespace"`
	ClusterRoles            []string                `json:"cluster_roles"`
	NamespaceRoles          []k8srbac.NamespaceRole `json:"namespace_roles"`
}

type clusterUserGrantListResponse struct {
	Items []clusterUserGrantView `json:"items"`
}

type memberKubeconfigView struct {
	Kubeconfig string `json:"kubeconfig"`
	ExpiresAt  string `json:"expires_at"`
}

func (h clusterHandler) listClusterUsers(c *gin.Context) {
	wsID, clusterID, ok := h.readyClusterWrite(c)
	if !ok {
		return
	}
	var rows []k8sschema.ClusterUserGrant
	if err := h.ctx.DB.WithContext(c.Request.Context()).
		Where("workspace_id = ? AND cluster_id = ?", wsID.String(), clusterID.String()).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		handleClusterError(c, err)
		return
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.MemberID)
	}
	profiles := lookupIdentityMembers(h.ctx.DB, ids)
	items := make([]clusterUserGrantView, 0, len(rows))
	for _, row := range rows {
		items = append(items, viewFromGrant(row, profiles[row.MemberID]))
	}
	httpx.OK(c, clusterUserGrantListResponse{Items: items})
}

func (h clusterHandler) createClusterUsers(c *gin.Context) {
	wsID, clusterID, ok := h.readyClusterWrite(c)
	if !ok {
		return
	}
	var req clusterUserGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_USER_INVALID", "invalid user grant payload", stdhttp.StatusBadRequest)
		return
	}
	if len(req.MemberIDs) == 0 {
		httpx.Fail(c, "PLUGIN_CLUSTER_USER_INVALID", "member_ids required", stdhttp.StatusBadRequest)
		return
	}
	if err := validateGrantRoles(req.ClusterRoles, req.NamespaceRoles); err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_USER_INVALID", err.Error(), stdhttp.StatusBadRequest)
		return
	}
	client, _, _, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	if client == nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_KUBECONFIG_FAILED", "kubernetes client unavailable", stdhttp.StatusBadRequest)
		return
	}
	items := make([]clusterUserGrantView, 0, len(req.MemberIDs))
	for _, rawID := range req.MemberIDs {
		memberID, err := kernel.ParseMemberID(rawID)
		if err != nil {
			httpx.Fail(c, "PLUGIN_CLUSTER_USER_INVALID", "invalid member id", stdhttp.StatusBadRequest)
			return
		}
		allowed, err := h.ctx.Members.IsWorkspaceMember(c.Request.Context(), wsID, memberID)
		if err != nil {
			handleClusterError(c, err)
			return
		}
		if !allowed {
			httpx.Fail(c, "PLUGIN_CLUSTER_USER_NOT_MEMBER", "member is not in this workspace", stdhttp.StatusBadRequest)
			return
		}
		row, err := h.upsertGrant(c.Request.Context(), client, wsID, clusterID, memberID, req.ClusterRoles, req.NamespaceRoles)
		if err != nil {
			handleClusterError(c, err)
			return
		}
		profiles := lookupIdentityMembers(h.ctx.DB, []string{row.MemberID})
		items = append(items, viewFromGrant(row, profiles[row.MemberID]))
	}
	httpx.OK(c, clusterUserGrantListResponse{Items: items})
}

func (h clusterHandler) updateClusterUser(c *gin.Context) {
	wsID, clusterID, memberID, ok := h.readyClusterUserWrite(c)
	if !ok {
		return
	}
	var req clusterUserGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_USER_INVALID", "invalid user grant payload", stdhttp.StatusBadRequest)
		return
	}
	if err := validateGrantRoles(req.ClusterRoles, req.NamespaceRoles); err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_USER_INVALID", err.Error(), stdhttp.StatusBadRequest)
		return
	}
	client, _, _, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	if client == nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_KUBECONFIG_FAILED", "kubernetes client unavailable", stdhttp.StatusBadRequest)
		return
	}
	row, err := h.upsertGrant(c.Request.Context(), client, wsID, clusterID, memberID, req.ClusterRoles, req.NamespaceRoles)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	profiles := lookupIdentityMembers(h.ctx.DB, []string{row.MemberID})
	httpx.OK(c, viewFromGrant(row, profiles[row.MemberID]))
}

func (h clusterHandler) deleteClusterUser(c *gin.Context) {
	wsID, clusterID, memberID, ok := h.readyClusterUserWrite(c)
	if !ok {
		return
	}
	client, _, _, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	if client != nil {
		if err := k8srbac.Remove(c.Request.Context(), client, wsID.String(), clusterID.String(), memberID.String()); err != nil {
			handleClusterError(c, err)
			return
		}
	}
	if err := h.ctx.DB.WithContext(c.Request.Context()).
		Where("workspace_id = ? AND cluster_id = ? AND member_id = ?", wsID.String(), clusterID.String(), memberID.String()).
		Delete(&k8sschema.ClusterUserGrant{}).Error; err != nil {
		handleClusterError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

func (h clusterHandler) issueClusterUserKubeconfig(c *gin.Context) {
	wsID, clusterID, memberID, ok := h.readyClusterUserWrite(c)
	if !ok {
		return
	}
	var row k8sschema.ClusterUserGrant
	err := h.ctx.DB.WithContext(c.Request.Context()).
		Where("workspace_id = ? AND cluster_id = ? AND member_id = ?", wsID.String(), clusterID.String(), memberID.String()).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.Fail(c, "PLUGIN_CLUSTER_USER_NOT_FOUND", "cluster user grant not found", stdhttp.StatusNotFound)
			return
		}
		handleClusterError(c, err)
		return
	}
	client, _, restCfg, meta, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	if client == nil || restCfg == nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_KUBECONFIG_FAILED", "kubernetes client unavailable", stdhttp.StatusBadRequest)
		return
	}
	token, expiresAt, err := issueServiceAccountToken(c.Request.Context(), client, row.ServiceAccountNamespace, row.ServiceAccountName)
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_KUBECONFIG_FAILED", "failed to issue service account token", stdhttp.StatusBadGateway)
		return
	}
	name := ""
	if meta != nil {
		name = meta.Name
	}
	yaml, err := kubeconfigYAMLFromToken(restCfg, name, token)
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_KUBECONFIG_FAILED", "failed to build kubeconfig", stdhttp.StatusInternalServerError)
		return
	}
	httpx.OK(c, memberKubeconfigView{
		Kubeconfig: yaml,
		ExpiresAt:  expiresAt.UTC().Format(time.RFC3339),
	})
}

func (h clusterHandler) readyClusterWrite(c *gin.Context) (kernel.WorkspaceID, kernel.ClusterInfoID, bool) {
	if !h.writeReady(c) {
		return kernel.WorkspaceID{}, kernel.ClusterInfoID{}, false
	}
	wsID, _, ok := requireWrite(c.Request.Context(), c, h.ctx.Writer)
	if !ok {
		return kernel.WorkspaceID{}, kernel.ClusterInfoID{}, false
	}
	clusterID, err := kernel.ParseClusterInfoID(c.Param("clusterId"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_INVALID", "invalid cluster id", stdhttp.StatusBadRequest)
		return kernel.WorkspaceID{}, kernel.ClusterInfoID{}, false
	}
	if _, err := h.authorizedMeta(c, wsID, clusterID); err != nil {
		handleClusterError(c, err)
		return kernel.WorkspaceID{}, kernel.ClusterInfoID{}, false
	}
	return wsID, clusterID, true
}

func (h clusterHandler) readyClusterUserWrite(c *gin.Context) (kernel.WorkspaceID, kernel.ClusterInfoID, kernel.MemberID, bool) {
	wsID, clusterID, ok := h.readyClusterWrite(c)
	if !ok {
		return kernel.WorkspaceID{}, kernel.ClusterInfoID{}, kernel.MemberID{}, false
	}
	memberID, err := kernel.ParseMemberID(c.Param("memberId"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_USER_INVALID", "invalid member id", stdhttp.StatusBadRequest)
		return kernel.WorkspaceID{}, kernel.ClusterInfoID{}, kernel.MemberID{}, false
	}
	return wsID, clusterID, memberID, true
}

func (h clusterHandler) upsertGrant(
	ctx context.Context,
	client kubernetes.Interface,
	wsID kernel.WorkspaceID,
	clusterID kernel.ClusterInfoID,
	memberID kernel.MemberID,
	clusterRoles []string,
	namespaceRoles []k8srbac.NamespaceRole,
) (k8sschema.ClusterUserGrant, error) {
	spec := k8srbac.Spec{
		WorkspaceID:    wsID.String(),
		ClusterID:      clusterID.String(),
		MemberID:       memberID.String(),
		ClusterRoles:   clusterRoles,
		NamespaceRoles: namespaceRoles,
	}
	if err := k8srbac.Apply(ctx, client, spec); err != nil {
		return k8sschema.ClusterUserGrant{}, err
	}
	clusterJSON, err := json.Marshal(clusterRoles)
	if err != nil {
		return k8sschema.ClusterUserGrant{}, err
	}
	if clusterJSON == nil {
		clusterJSON = []byte("[]")
	}
	nsJSON, err := json.Marshal(namespaceRoles)
	if err != nil {
		return k8sschema.ClusterUserGrant{}, err
	}
	if nsJSON == nil {
		nsJSON = []byte("[]")
	}
	var existing k8sschema.ClusterUserGrant
	err = h.ctx.DB.WithContext(ctx).
		Where("workspace_id = ? AND cluster_id = ? AND member_id = ?", wsID.String(), clusterID.String(), memberID.String()).
		First(&existing).Error
	now := time.Now().UTC()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return k8sschema.ClusterUserGrant{}, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row := k8sschema.ClusterUserGrant{
			ID:                      uuid.NewString(),
			WorkspaceID:             wsID.String(),
			ClusterID:               clusterID.String(),
			MemberID:                memberID.String(),
			ServiceAccountName:      k8srbac.ServiceAccountName(memberID.String()),
			ServiceAccountNamespace: k8srbac.ManagedNamespace,
			ClusterRoles:            string(clusterJSON),
			NamespaceRoles:          string(nsJSON),
			CreatedAt:               now,
			UpdatedAt:               now,
		}
		if err := h.ctx.DB.WithContext(ctx).Create(&row).Error; err != nil {
			return k8sschema.ClusterUserGrant{}, err
		}
		return row, nil
	}
	existing.ClusterRoles = string(clusterJSON)
	existing.NamespaceRoles = string(nsJSON)
	existing.ServiceAccountName = k8srbac.ServiceAccountName(memberID.String())
	existing.ServiceAccountNamespace = k8srbac.ManagedNamespace
	existing.UpdatedAt = now
	if err := h.ctx.DB.WithContext(ctx).Save(&existing).Error; err != nil {
		return k8sschema.ClusterUserGrant{}, err
	}
	return existing, nil
}

func validateGrantRoles(clusterRoles []string, namespaceRoles []k8srbac.NamespaceRole) error {
	if len(clusterRoles) == 0 && !hasNamespaceRoles(namespaceRoles) {
		return kernel.NewDomainError("PLUGIN_CLUSTER_USER_INVALID", "at least one cluster or namespace role is required")
	}
	return nil
}

func hasNamespaceRoles(items []k8srbac.NamespaceRole) bool {
	for _, item := range items {
		if item.Namespace != "" && len(item.Roles) > 0 {
			return true
		}
	}
	return false
}

func viewFromGrant(row k8sschema.ClusterUserGrant, profile identityMemberProfile) clusterUserGrantView {
	return clusterUserGrantView{
		MemberID:                row.MemberID,
		Email:                   profile.Email,
		DisplayName:             profile.DisplayName,
		ServiceAccountName:      row.ServiceAccountName,
		ServiceAccountNamespace: row.ServiceAccountNamespace,
		ClusterRoles:            decodeStringSlice(row.ClusterRoles),
		NamespaceRoles:          decodeNamespaceRoles(row.NamespaceRoles),
	}
}

func decodeStringSlice(raw string) []string {
	if raw == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return []string{}
	}
	return out
}

func decodeNamespaceRoles(raw string) []k8srbac.NamespaceRole {
	if raw == "" {
		return []k8srbac.NamespaceRole{}
	}
	var out []k8srbac.NamespaceRole
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return []k8srbac.NamespaceRole{}
	}
	return out
}

type identityMemberProfile struct {
	Email       string
	DisplayName string
}

func lookupIdentityMembers(db *gorm.DB, ids []string) map[string]identityMemberProfile {
	out := map[string]identityMemberProfile{}
	if db == nil || len(ids) == 0 {
		return out
	}
	var rows []struct {
		ID          string `gorm:"column:id"`
		Email       string `gorm:"column:email"`
		DisplayName string `gorm:"column:display_name"`
	}
	if err := db.Table("identity.members").Select("id, email, display_name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return out
	}
	for _, row := range rows {
		out[row.ID] = identityMemberProfile{Email: row.Email, DisplayName: row.DisplayName}
	}
	return out
}

func createServiceAccountToken(ctx context.Context, client kubernetes.Interface, namespace, name string) (string, time.Time, error) {
	expire := serviceAccountTokenTTLSeconds
	out, err := client.CoreV1().ServiceAccounts(namespace).CreateToken(ctx, name, &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{ExpirationSeconds: &expire},
	}, metav1.CreateOptions{})
	if err != nil {
		return "", time.Time{}, err
	}
	return out.Status.Token, out.Status.ExpirationTimestamp.Time, nil
}

func kubeconfigYAMLFromToken(cfg *rest.Config, clusterName, token string) (string, error) {
	if cfg == nil {
		return "", kernel.NewDomainError("PLUGIN_CLUSTER_KUBECONFIG_FAILED", "failed to build kubeconfig")
	}
	copyCfg := rest.CopyConfig(cfg)
	copyCfg.BearerToken = token
	copyCfg.BearerTokenFile = ""
	copyCfg.CertData = nil
	copyCfg.KeyData = nil
	copyCfg.CertFile = ""
	copyCfg.KeyFile = ""
	return kubeconfigYAMLFromREST(copyCfg, clusterName)
}
