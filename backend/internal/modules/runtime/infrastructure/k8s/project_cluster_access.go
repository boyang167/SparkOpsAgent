package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type ProjectClusterSession struct {
	Client     kubernetes.Interface
	RESTConfig *rest.Config
	Namespace  string
	ProjectName string
	Server     string
}

type ProjectClusterAccess struct {
	db  *gorm.DB
	box *crypto.SecretBox
}

func NewProjectClusterAccess(db *gorm.DB, box *crypto.SecretBox) *ProjectClusterAccess {
	return &ProjectClusterAccess{db: db, box: box}
}

type projectClusterRow struct {
	Name               string
	Namespace          string
	DefaultClusterID   *string
	KubernetesSecretID *string
}

func (a *ProjectClusterAccess) Open(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) (*ProjectClusterSession, error) {
	if a.db == nil || a.box == nil {
		return nil, fmt.Errorf("project cluster access is not configured")
	}
	var row projectClusterRow
	err := a.db.WithContext(ctx).Table("workspace_app.projects").
		Select("name, namespace, default_cluster_id, kubernetes_secret_id").
		Where("id = ? AND workspace_id = ?", projectID.String(), wsID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	clusterID, server, err := a.resolveClusterID(ctx, wsID, row.DefaultClusterID)
	if err != nil {
		return nil, err
	}
	kubeconfig, err := a.loadKubeconfig(ctx, wsID, clusterID, clusterSecretValue(row.KubernetesSecretID))
	if err != nil {
		return nil, err
	}
	client, err := NewTypedClientFromKubeconfig(kubeconfig, server)
	if err != nil {
		return nil, err
	}
	restCfg, err := RESTConfigFromKubeconfig(kubeconfig, server)
	if err != nil {
		return nil, err
	}
	namespace := strings.TrimSpace(row.Namespace)
	if namespace == "" {
		namespace = DefaultNamespace(row.Name)
	}
	return &ProjectClusterSession{
		Client: client, RESTConfig: restCfg, Namespace: namespace, ProjectName: row.Name, Server: server,
	}, nil
}

func (a *ProjectClusterAccess) resolveClusterID(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	defaultClusterID *string,
) (kernel.ClusterInfoID, string, error) {
	if id := strings.TrimSpace(clusterSecretValue(defaultClusterID)); id != "" {
		clusterID, err := kernel.ParseClusterInfoID(id)
		if err != nil {
			return kernel.ClusterInfoID{}, "", err
		}
		server, err := a.clusterServer(ctx, wsID, clusterID)
		return clusterID, server, err
	}
	var row struct {
		ID     string
		Server string
	}
	err := a.db.WithContext(ctx).Raw(`
		SELECT c.id, c.server
		FROM platform_admin.cluster_info AS c
		INNER JOIN platform_admin.workspace_clusters AS g
			ON g.cluster_id = c.id AND g.workspace_id = ?
		ORDER BY g.priority_id ASC NULLS LAST, g.created_at ASC
		LIMIT 1`, wsID.String()).Scan(&row).Error
	if err != nil {
		return kernel.ClusterInfoID{}, "", err
	}
	if row.ID == "" {
		return kernel.ClusterInfoID{}, "", fmt.Errorf("no authorized cluster for workspace")
	}
	clusterID, err := kernel.ParseClusterInfoID(row.ID)
	if err != nil {
		return kernel.ClusterInfoID{}, "", err
	}
	return clusterID, row.Server, nil
}

func (a *ProjectClusterAccess) clusterServer(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	clusterID kernel.ClusterInfoID,
) (string, error) {
	var row struct {
		Server string
	}
	err := a.db.WithContext(ctx).Raw(`
		SELECT c.server
		FROM platform_admin.cluster_info AS c
		INNER JOIN platform_admin.workspace_clusters AS g
			ON g.cluster_id = c.id AND g.workspace_id = ?
		WHERE c.id = ?`, wsID.String(), clusterID.String()).Scan(&row).Error
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(row.Server) == "" {
		return "", fmt.Errorf("cluster server is empty")
	}
	return row.Server, nil
}

func (a *ProjectClusterAccess) loadKubeconfig(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	clusterID kernel.ClusterInfoID,
	secretID string,
) ([]byte, error) {
	if strings.TrimSpace(secretID) != "" {
		return a.kubeconfigFromSecret(ctx, wsID, secretID)
	}
	return a.kubeconfigFromCluster(ctx, wsID, clusterID)
}

func (a *ProjectClusterAccess) kubeconfigFromSecret(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	secretID string,
) ([]byte, error) {
	var row struct {
		CredentialBlob string
	}
	err := a.db.WithContext(ctx).Table("workspace_app.secrets").
		Select("credential_blob").
		Where("id = ? AND workspace_id = ? AND service_type = ? AND status = ?",
			secretID, wsID.String(), "Kubernetes", string(model.SecretStatusActive)).
		First(&row).Error
	if err != nil {
		return nil, err
	}
	plain, err := a.box.Decrypt(row.CredentialBlob)
	if err != nil {
		return nil, err
	}
	var credentials model.SecretCredentials
	if err := json.Unmarshal([]byte(plain), &credentials); err != nil {
		return nil, err
	}
	if strings.TrimSpace(credentials.ExtraConfig) == "" {
		return nil, fmt.Errorf("kubernetes kubeconfig is missing")
	}
	return []byte(credentials.ExtraConfig), nil
}

type clusterCredentialRow struct {
	Server                   string
	CertificateAuthorityData string
	ClientCertificateData    string
	ClientKeyData            string
	BearerToken              string
}

func (a *ProjectClusterAccess) kubeconfigFromCluster(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	clusterID kernel.ClusterInfoID,
) ([]byte, error) {
	var row clusterCredentialRow
	err := a.db.WithContext(ctx).Raw(`
		SELECT c.server, c.certificate_authority_data, c.client_certificate_data,
			c.client_key_data, c.bearer_token
		FROM platform_admin.cluster_info AS c
		INNER JOIN platform_admin.workspace_clusters AS g
			ON g.cluster_id = c.id AND g.workspace_id = ?
		WHERE c.id = ?`, wsID.String(), clusterID.String()).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return writeProjectKubeconfig(row)
}

func writeProjectKubeconfig(row clusterCredentialRow) ([]byte, error) {
	server := strings.TrimSpace(row.Server)
	if server == "" {
		return nil, fmt.Errorf("missing server")
	}
	authInfo := &api.AuthInfo{
		Token:                 strings.TrimSpace(row.BearerToken),
		ClientCertificateData: []byte(row.ClientCertificateData),
		ClientKeyData:         []byte(row.ClientKeyData),
	}
	if authInfo.Token == "" && (len(authInfo.ClientCertificateData) == 0 || len(authInfo.ClientKeyData) == 0) {
		return nil, fmt.Errorf("missing cluster credentials")
	}
	cfg := api.NewConfig()
	cfg.Clusters["sparkops"] = &api.Cluster{
		Server:                   server,
		CertificateAuthorityData: []byte(row.CertificateAuthorityData),
	}
	cfg.AuthInfos["sparkops"] = authInfo
	cfg.Contexts["sparkops"] = &api.Context{Cluster: "sparkops", AuthInfo: "sparkops"}
	cfg.CurrentContext = "sparkops"
	return clientcmd.Write(*cfg)
}

func clusterSecretValue(input *string) string {
	if input == nil {
		return ""
	}
	return *input
}
