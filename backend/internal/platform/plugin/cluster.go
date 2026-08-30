package plugin

import (
	"context"
	"strings"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ClusterMeta struct {
	ID                 string
	Name               string
	Server             string
	PrometheusURL      string
	PrometheusUser     string
	PrometheusPassword string
	VelauxURL          string
	VelauxUser         string
	VelauxPassword     string
	LLMBaseURL         string
	LLMAPIKey          string
	LLMModel           string
}

type ClusterCredential struct {
	ID                       kernel.ClusterInfoID
	Name                     string
	Server                   string
	CertificateAuthorityData string
	ClientCertificateData    string
	ClientKeyData            string
	BearerToken              string
}

type ClusterCredentialSource interface {
	ListAuthorized(ctx context.Context, wsID kernel.WorkspaceID) ([]ClusterCredential, error)
	GetAuthorized(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (*ClusterCredential, error)
}

type ClusterClientFactory interface {
	ForWorkspace(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (
		kubernetes.Interface, dynamic.Interface, *rest.Config, *ClusterMeta, error,
	)
	ListMeta(ctx context.Context, wsID kernel.WorkspaceID) ([]ClusterMeta, error)
}

func NewFactory(src ClusterCredentialSource) ClusterClientFactory {
	return &clusterClientFactory{src: src}
}

type clusterClientFactory struct {
	src ClusterCredentialSource
}

func (f *clusterClientFactory) ForWorkspace(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *ClusterMeta, error,
) {
	if f.src == nil {
		return nil, nil, nil, nil, kernel.NewDomainError("PLUGIN_CLUSTER_FORBIDDEN", "cluster is not granted to workspace")
	}
	cred, err := f.src.GetAuthorized(ctx, wsID, clusterID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if cred == nil || strings.TrimSpace(cred.Server) == "" {
		return nil, nil, nil, nil, kernel.NewDomainError("PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE", "cluster credentials are incomplete")
	}
	hasCert := cred.ClientCertificateData != "" && cred.ClientKeyData != ""
	hasToken := strings.TrimSpace(cred.BearerToken) != ""
	if !hasCert && !hasToken {
		return nil, nil, nil, nil, kernel.NewDomainError("PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE", "cluster credentials are incomplete")
	}
	cfg := &rest.Config{
		Host:        strings.TrimSpace(cred.Server),
		BearerToken: strings.TrimSpace(cred.BearerToken),
		TLSClientConfig: rest.TLSClientConfig{
			CAData:   []byte(cred.CertificateAuthorityData),
			CertData: []byte(cred.ClientCertificateData),
			KeyData:  []byte(cred.ClientKeyData),
			Insecure: cred.CertificateAuthorityData == "" && hasToken,
		},
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	meta := clusterMetaFromCredential(*cred)
	return clientset, dynamicClient, cfg, &meta, nil
}

func (f *clusterClientFactory) ListMeta(ctx context.Context, wsID kernel.WorkspaceID) ([]ClusterMeta, error) {
	if f.src == nil {
		return nil, nil
	}
	creds, err := f.src.ListAuthorized(ctx, wsID)
	if err != nil {
		return nil, err
	}
	out := make([]ClusterMeta, 0, len(creds))
	for _, cred := range creds {
		out = append(out, clusterMetaFromCredential(cred))
	}
	return out, nil
}

func clusterMetaFromCredential(cred ClusterCredential) ClusterMeta {
	return ClusterMeta{
		ID:     cred.ID.String(),
		Name:   cred.Name,
		Server: strings.TrimSpace(cred.Server),
	}
}
