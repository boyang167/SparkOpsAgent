package acl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	deploydomain "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	workspacequery "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	workspace "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type WorkspaceAppACL struct {
	db  *gorm.DB
	box *crypto.SecretBox
}

func NewWorkspaceAppACL(db *gorm.DB, box *crypto.SecretBox) *WorkspaceAppACL {
	return &WorkspaceAppACL{db: db, box: box}
}

type deploymentProjectRow struct {
	ID                 string
	WorkspaceID        string
	Name               string
	DisplayName        string
	Namespace          string
	Language           string
	GitURL             string
	GitRefType         string
	GitRefValue        string
	DockerfilePath     string
	TemplateID         string
	SecretID           *string
	ReviewStatus       string
	Status             string
	ArchiveGeneration  *string
	ArchivePhase       string
	Resources          []byte
	DefaultClusterID   *string
	JenkinsSecretID    *string
	KubernetesSecretID *string
	ProductID          *string
}

func (a *WorkspaceAppACL) loadProject(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) (deploymentProjectRow, error) {
	var row deploymentProjectRow
	err := postgres.DB(ctx, a.db).Table("workspace_app.projects").
		Where("id = ? AND workspace_id = ?", projectID.String(), wsID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, deploydomain.ErrDeploymentBindingMissing
	}
	return row, err
}

func (a *WorkspaceAppACL) ProjectLifecycle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) (port.ProjectLifecycle, error) {
	row, err := a.loadProject(ctx, wsID, projectID)
	if err != nil {
		return port.ProjectLifecycle{}, err
	}
	lifecycle := port.ProjectLifecycle{Name: row.Name, Status: row.Status, ArchivePhase: row.ArchivePhase}
	if row.ArchiveGeneration != nil {
		lifecycle.ArchiveGeneration = *row.ArchiveGeneration
	}
	if row.ProductID != nil {
		lifecycle.ProductID = *row.ProductID
	}
	return lifecycle, nil
}

var _ port.ProjectLifecycleReader = (*WorkspaceAppACL)(nil)

func (a *WorkspaceAppACL) TransitionArchivePhase(
	ctx context.Context,
	projectID kernel.ProjectID,
	generation string,
	expected string,
	target string,
) (bool, error) {
	result := postgres.DB(ctx, a.db).Table("workspace_app.projects").
		Where("id = ? AND status = ? AND archive_generation = ? AND archive_phase = ?",
			projectID.String(), string(workspace.ProjectStatusArchiving), generation, expected).
		Update("archive_phase", target)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

var _ port.ProjectArchiveFence = (*WorkspaceAppACL)(nil)

func (a *WorkspaceAppACL) loadSecret(ctx context.Context, wsID kernel.WorkspaceID, secretID string, allowedTypes ...string) (port.DeploymentSecret, error) {
	if strings.TrimSpace(secretID) == "" {
		return port.DeploymentSecret{}, deploydomain.ErrDeploymentBindingMissing
	}
	var row struct {
		ID             string
		ServiceType    string
		ServiceURL     string
		Username       string
		CredentialBlob string
	}
	err := a.db.WithContext(ctx).Table("workspace_app.secrets").
		Where("id = ? AND workspace_id = ? AND status = ?", secretID, wsID.String(), string(workspace.SecretStatusActive)).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return port.DeploymentSecret{}, deploydomain.ErrDeploymentBindingMissing
	}
	if err != nil {
		return port.DeploymentSecret{}, err
	}
	allowed := false
	for _, serviceType := range allowedTypes {
		if row.ServiceType == serviceType {
			allowed = true
			break
		}
	}
	if !allowed {
		return port.DeploymentSecret{}, deploydomain.ErrDeploymentBindingMissing
	}
	id, err := kernel.ParseSecretID(row.ID)
	if err != nil {
		return port.DeploymentSecret{}, err
	}
	plain, err := a.box.Decrypt(row.CredentialBlob)
	if err != nil {
		return port.DeploymentSecret{}, deploydomain.ErrDeploymentBindingInvalid
	}
	var credentials workspace.SecretCredentials
	if plain != "" {
		if err := json.Unmarshal([]byte(plain), &credentials); err != nil {
			return port.DeploymentSecret{}, deploydomain.ErrDeploymentBindingInvalid
		}
	}
	return port.DeploymentSecret{
		ID: id, ServiceType: row.ServiceType, ServiceURL: row.ServiceURL,
		Username: row.Username, Credentials: mapDeploymentCredentials(credentials),
	}, nil
}

func mapDeploymentCredentials(credentials workspace.SecretCredentials) port.DeploymentCredentials {
	return port.DeploymentCredentials{
		Password: credentials.Password, Token: credentials.Token, APIKey: credentials.APIKey,
		PrivateKey: credentials.PrivateKey, PublicKey: credentials.PublicKey,
		Certificate: credentials.Certificate, ExtraConfig: credentials.ExtraConfig,
	}
}

func (a *WorkspaceAppACL) loadAuthorizedCluster(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (port.DeploymentCluster, error) {
	var row struct {
		ID          string
		ClusterName string
		Server      string
	}
	err := a.db.WithContext(ctx).Raw(`
		SELECT c.id, c.cluster_name, c.server
		FROM platform_admin.cluster_info AS c
		INNER JOIN platform_admin.workspace_clusters AS g
			ON g.cluster_id = c.id AND g.workspace_id = ?
		WHERE c.id = ?`, wsID.String(), clusterID.String()).Scan(&row).Error
	if err != nil {
		return port.DeploymentCluster{}, err
	}
	if row.ID == "" {
		return port.DeploymentCluster{}, deploydomain.ErrDeploymentClusterNotAuthorized
	}
	id, err := kernel.ParseClusterInfoID(row.ID)
	if err != nil {
		return port.DeploymentCluster{}, err
	}
	return port.DeploymentCluster{ID: id, Name: row.ClusterName, Server: row.Server}, nil
}

func (a *WorkspaceAppACL) loadKubernetesCredentials(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	clusterID kernel.ClusterInfoID,
	secretID string,
) (port.DeploymentSecret, error) {
	if strings.TrimSpace(secretID) != "" {
		return a.loadKubernetesSecret(ctx, wsID, secretID)
	}
	return a.loadKubernetesFromCluster(ctx, wsID, clusterID)
}

func (a *WorkspaceAppACL) loadKubernetesSecret(ctx context.Context, wsID kernel.WorkspaceID, secretID string) (port.DeploymentSecret, error) {
	secret, err := a.loadSecret(ctx, wsID, secretID, "Kubernetes")
	if err != nil {
		return port.DeploymentSecret{}, err
	}
	if strings.TrimSpace(secret.Credentials.ExtraConfig) == "" {
		return port.DeploymentSecret{}, deploydomain.ErrKubernetesKubeconfigMissing
	}
	return secret, nil
}

type clusterCredentialRow struct {
	ID                       string
	ClusterName              string
	Server                   string
	UserName                 string
	CertificateAuthorityData string
	ClientCertificateData    string
	ClientKeyData            string
	BearerToken              string
}

func (a *WorkspaceAppACL) loadKubernetesFromCluster(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	clusterID kernel.ClusterInfoID,
) (port.DeploymentSecret, error) {
	var row clusterCredentialRow
	err := a.db.WithContext(ctx).Raw(`
		SELECT c.id, c.cluster_name, c.server, c.user_name,
			c.certificate_authority_data, c.client_certificate_data,
			c.client_key_data, c.bearer_token
		FROM platform_admin.cluster_info AS c
		INNER JOIN platform_admin.workspace_clusters AS g
			ON g.cluster_id = c.id AND g.workspace_id = ?
		WHERE c.id = ?`, wsID.String(), clusterID.String()).Scan(&row).Error
	if err != nil {
		return port.DeploymentSecret{}, err
	}
	if row.ID == "" {
		return port.DeploymentSecret{}, deploydomain.ErrDeploymentClusterNotAuthorized
	}
	kubeconfig, err := kubeconfigFromClusterCredentials(row)
	if err != nil {
		return port.DeploymentSecret{}, deploydomain.ErrKubernetesKubeconfigMissing
	}
	return port.DeploymentSecret{
		ServiceType: "Kubernetes",
		Credentials: port.DeploymentCredentials{ExtraConfig: string(kubeconfig)},
	}, nil
}

func kubeconfigFromClusterCredentials(row clusterCredentialRow) ([]byte, error) {
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
		return nil, fmt.Errorf("missing credentials")
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

func (a *WorkspaceAppACL) Load(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, selected kernel.ClusterInfoID) (port.ProjectDeploymentContext, error) {
	project, err := a.loadProject(ctx, wsID, projectID)
	if err != nil {
		return port.ProjectDeploymentContext{}, err
	}
	if project.ReviewStatus != string(workspace.ProjectReviewApproved) {
		return port.ProjectDeploymentContext{}, deploydomain.ErrProjectNotApproved
	}
	var resources workspace.ProjectResources
	if len(project.Resources) > 0 {
		if err := json.Unmarshal(project.Resources, &resources); err != nil {
			return port.ProjectDeploymentContext{}, deploydomain.ErrDeploymentBindingInvalid
		}
	}
	clusterID := selected
	if clusterID.UUID == uuid.Nil {
		buildCluster, loadErr := a.loadBuildCluster(ctx, wsID)
		if loadErr != nil {
			if project.DefaultClusterID == nil {
				return port.ProjectDeploymentContext{}, deploydomain.ErrDeploymentBindingMissing
			}
			clusterID, err = kernel.ParseClusterInfoID(*project.DefaultClusterID)
			if err != nil {
				return port.ProjectDeploymentContext{}, deploydomain.ErrDeploymentBindingMissing
			}
		} else {
			clusterID = buildCluster.ID
		}
	}
	cluster, err := a.loadAuthorizedCluster(ctx, wsID, clusterID)
	if err != nil {
		return port.ProjectDeploymentContext{}, err
	}
	git, err := a.loadSecret(ctx, wsID, value(project.SecretID), "GitLab", "GitHub")
	if err != nil {
		return port.ProjectDeploymentContext{}, err
	}
	harbor, err := a.loadSecret(ctx, wsID, resources.RegistrySecretID, "Harbor")
	if err != nil {
		return port.ProjectDeploymentContext{}, err
	}
	jenkins, err := a.loadSecret(ctx, wsID, value(project.JenkinsSecretID), "Jenkins")
	if err != nil {
		return port.ProjectDeploymentContext{}, err
	}
	kubernetes, err := a.loadKubernetesCredentials(ctx, wsID, clusterID, value(project.KubernetesSecretID))
	if err != nil {
		return port.ProjectDeploymentContext{}, err
	}
	var sonar *port.DeploymentSecret
	if resources.EnableStaticScan {
		loaded, err := a.loadSecret(ctx, wsID, resources.SonarqubeSecretID, "SonarQube")
		if err != nil {
			return port.ProjectDeploymentContext{}, err
		}
		sonar = &loaded
	}
	imageRepository := deploymentImageRepository(harbor.ServiceURL, project.Namespace, project.Name)
	serviceYAML, deploymentYAML, jenkinsfile, err := a.renderProjectArtifacts(
		ctx, wsID, project, resources, imageRepository,
	)
	if err != nil {
		return port.ProjectDeploymentContext{}, err
	}
	return port.ProjectDeploymentContext{
		WorkspaceID: wsID, ProjectID: projectID, Name: project.Name, Namespace: project.Namespace,
		GitURL:      project.GitURL,
		Jenkinsfile: jenkinsfile, ServiceYAML: serviceYAML, DeploymentYAML: deploymentYAML,
		ImageRepository:  imageRepository,
		EnableStaticScan: resources.EnableStaticScan, Cluster: cluster,
		Git: git, Harbor: harbor, Jenkins: jenkins, Kubernetes: kubernetes, Sonar: sonar,
	}, nil
}

func (a *WorkspaceAppACL) renderProjectArtifacts(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	row deploymentProjectRow,
	resources workspace.ProjectResources,
	imageRepository string,
) (string, string, string, error) {
	name, err := workspace.ProjectNameFromDB(row.Name)
	if err != nil {
		return "", "", "", err
	}
	project := &workspace.Project{
		WorkspaceID: wsID, Name: name, DisplayName: row.DisplayName, Namespace: row.Namespace,
		Language: row.Language, GitURL: row.GitURL, GitRefType: row.GitRefType,
		GitRefValue: row.GitRefValue, DockerfilePath: row.DockerfilePath,
		TemplateID: row.TemplateID, Resources: resources,
	}
	ids := selectedTemplateIDs(row.TemplateID, resources)
	templates := make(map[string]*workspace.Template, len(ids))
	if len(ids) > 0 {
		var rows []struct {
			ID, Name, Kind, Language, Detail, Status string
			IsPublic                                 bool
		}
		if err := a.db.WithContext(ctx).Table("workspace_app.templates").
			Where("id IN ? AND status = ? AND (workspace_id = ? OR is_public = ?)",
				ids, string(workspace.TemplateStatusActive), wsID.String(), true).
			Find(&rows).Error; err != nil {
			return "", "", "", err
		}
		for _, item := range rows {
			id, err := kernel.ParseTemplateID(item.ID)
			if err != nil {
				return "", "", "", err
			}
			templateName, err := workspace.NewTemplateName(item.Name)
			if err != nil {
				return "", "", "", err
			}
			templates[item.ID] = &workspace.Template{
				ID: id, Name: templateName, Kind: item.Kind, Language: item.Language,
				Detail: item.Detail, Status: workspace.TemplateStatus(item.Status), IsPublic: item.IsPublic,
			}
		}
	}
	if len(ids) == 0 {
		project.TemplateID = "deployment-default"
	}
	var serviceYAML, deploymentYAML, jenkinsfile string
	for _, artifact := range workspacequery.GenerateArtifacts(project, templates, workspacequery.TemplateRenderContext{
		ContainerImage: imageRepository + ":latest", ImagePullSecret: "registry-" + row.Name,
	}) {
		switch artifact.Kind {
		case "Service":
			serviceYAML = artifact.Content
		case "Deployment":
			deploymentYAML = artifact.Content
		case "jenkinsFile":
			jenkinsfile = artifact.Content
		}
	}
	if serviceYAML == "" || deploymentYAML == "" {
		return "", "", "", deploydomain.ErrDeploymentBindingMissing
	}
	return serviceYAML, deploymentYAML, jenkinsfile, nil
}

func selectedTemplateIDs(templateID string, resources workspace.ProjectResources) []string {
	seen := map[string]bool{}
	var ids []string
	for _, id := range []string{
		templateID, resources.ServiceTemplateID, resources.DeploymentTemplateID, resources.JenkinsfileTemplateID,
	} {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func OverrideDeployNamespace(project *port.ProjectDeploymentContext, namespace string) {
	ns := strings.TrimSpace(namespace)
	if ns == "" || ns == project.Namespace {
		return
	}
	project.Namespace = ns
	if strings.TrimSpace(project.Harbor.ServiceURL) != "" {
		project.ImageRepository = deploymentImageRepository(project.Harbor.ServiceURL, ns, project.Name)
	}
}

func deploymentImageRepository(registryURL, namespace, projectName string) string {
	raw := strings.TrimSpace(registryURL)
	if raw == "" {
		return ""
	}
	parseable := raw
	if !strings.Contains(parseable, "://") {
		parseable = "https://" + parseable
	}
	parsed, err := url.Parse(parseable)
	host := ""
	if err == nil {
		host = parsed.Host
	}
	if host == "" {
		host = strings.Trim(raw, "/")
	}
	return strings.ToLower(strings.Trim(host, "/")) + "/" +
		strings.Trim(strings.ToLower(namespace), "/") + "/" +
		strings.Trim(strings.ToLower(projectName), "/")
}

func (a *WorkspaceAppACL) ListAuthorizedClusters(ctx context.Context, wsID kernel.WorkspaceID) ([]port.DeploymentCluster, error) {
	var rows []struct {
		ID          string
		ClusterName string
		Server      string
		PriorityID  int
		IsBuildEnv  bool
	}
	if err := a.db.WithContext(ctx).Raw(`
		SELECT c.id, c.cluster_name, c.server, g.priority_id, c.is_build_env
		FROM platform_admin.cluster_info AS c
		INNER JOIN platform_admin.workspace_clusters AS g ON g.cluster_id = c.id
		WHERE g.workspace_id = ?
		ORDER BY g.priority_id ASC`, wsID.String()).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]port.DeploymentCluster, 0, len(rows))
	for _, row := range rows {
		id, err := kernel.ParseClusterInfoID(row.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, port.DeploymentCluster{
			ID: id, Name: row.ClusterName, Server: row.Server,
			IsBuildEnv: row.IsBuildEnv, PriorityID: row.PriorityID,
		})
	}
	return out, nil
}

func (a *WorkspaceAppACL) loadBuildCluster(ctx context.Context, wsID kernel.WorkspaceID) (port.DeploymentCluster, error) {
	clusters, err := a.ListAuthorizedClusters(ctx, wsID)
	if err != nil {
		return port.DeploymentCluster{}, err
	}
	if len(clusters) == 0 {
		return port.DeploymentCluster{}, deploydomain.ErrDeploymentClusterNotAuthorized
	}
	for _, cluster := range clusters {
		if cluster.IsBuildEnv {
			return cluster, nil
		}
	}
	return clusters[0], nil
}

func value(input *string) string {
	if input == nil {
		return ""
	}
	return *input
}

var _ port.ProjectDeploymentReader = (*WorkspaceAppACL)(nil)
