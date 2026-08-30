package command

import (
	"fmt"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	KubeTypeConfig = 1
	KubeTypeToken  = 2
)

type ClusterCredentialInput struct {
	KubeType    int
	KubeConfig  string
	APIAddress  string
	UserName    string
}

type ParsedClusterCredentials struct {
	Server                   string
	UserName                 string
	CertificateAuthorityData string
	ClientCertificateData    string
	ClientKeyData            string
	BearerToken              string
}

func ParseClusterCredentials(input ClusterCredentialInput) (*ParsedClusterCredentials, error) {
	kubeType := input.KubeType
	if kubeType == 0 {
		kubeType = KubeTypeConfig
	}
	raw := strings.TrimSpace(input.KubeConfig)
	if raw == "" {
		return nil, fmt.Errorf("cluster credentials are required")
	}
	switch kubeType {
	case KubeTypeToken:
		server := strings.TrimSpace(input.APIAddress)
		if server == "" {
			return nil, fmt.Errorf("api address is required for token credentials")
		}
		return &ParsedClusterCredentials{
			Server:      server,
			UserName:    strings.TrimSpace(input.UserName),
			BearerToken: raw,
		}, nil
	case KubeTypeConfig:
		cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(raw))
		if err != nil {
			return nil, fmt.Errorf("invalid kubeconfig: %w", err)
		}
		return credentialsFromRESTConfig(cfg, input.APIAddress, input.UserName)
	default:
		return nil, fmt.Errorf("unsupported credential type %d", kubeType)
	}
}

func credentialsFromRESTConfig(cfg *rest.Config, apiAddress, userName string) (*ParsedClusterCredentials, error) {
	if cfg == nil {
		return nil, fmt.Errorf("invalid kubeconfig")
	}
	server := strings.TrimSpace(apiAddress)
	if server == "" {
		server = strings.TrimSpace(cfg.Host)
	}
	if server == "" {
		return nil, fmt.Errorf("api address is required")
	}
	out := &ParsedClusterCredentials{
		Server:                   server,
		UserName:                 strings.TrimSpace(userName),
		CertificateAuthorityData: string(cfg.CAData),
		ClientCertificateData:    string(cfg.CertData),
		ClientKeyData:            string(cfg.KeyData),
		BearerToken:              strings.TrimSpace(cfg.BearerToken),
	}
	if out.UserName == "" {
		out.UserName = strings.TrimSpace(cfg.Username)
	}
	if out.BearerToken == "" && cfg.BearerTokenFile != "" {
		return nil, fmt.Errorf("kubeconfig bearer token file is not supported; paste token inline")
	}
	if out.BearerToken == "" && (out.ClientCertificateData == "" || out.ClientKeyData == "") {
		return nil, fmt.Errorf("kubeconfig must include client certificate/key or bearer token")
	}
	return out, nil
}
