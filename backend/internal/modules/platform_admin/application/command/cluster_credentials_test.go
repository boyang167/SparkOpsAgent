package command_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/application/command"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestParseClusterCredentialsFromKubeConfig(t *testing.T) {
	raw := mustKubeconfig(t, "https://127.0.0.1:6443", "token-value", "", "", "")
	got, err := command.ParseClusterCredentials(command.ClusterCredentialInput{
		KubeType:   command.KubeTypeConfig,
		KubeConfig: raw,
	})
	require.NoError(t, err)
	require.Equal(t, "https://127.0.0.1:6443", got.Server)
	require.Equal(t, "token-value", got.BearerToken)
}

func TestParseClusterCredentialsFromToken(t *testing.T) {
	got, err := command.ParseClusterCredentials(command.ClusterCredentialInput{
		KubeType:   command.KubeTypeToken,
		KubeConfig: "plain-token",
		APIAddress: "https://k8s.example:6443",
	})
	require.NoError(t, err)
	require.Equal(t, "https://k8s.example:6443", got.Server)
	require.Equal(t, "plain-token", got.BearerToken)
}

func TestParseClusterCredentialsRejectsEmpty(t *testing.T) {
	_, err := command.ParseClusterCredentials(command.ClusterCredentialInput{KubeType: command.KubeTypeConfig})
	require.Error(t, err)
}

func mustKubeconfig(t *testing.T, server, token, ca, cert, key string) string {
	t.Helper()
	cfg := api.NewConfig()
	cfg.Clusters["c"] = &api.Cluster{Server: server, CertificateAuthorityData: []byte(ca)}
	cfg.AuthInfos["u"] = &api.AuthInfo{
		Token: token, ClientCertificateData: []byte(cert), ClientKeyData: []byte(key),
	}
	cfg.Contexts["ctx"] = &api.Context{Cluster: "c", AuthInfo: "u"}
	cfg.CurrentContext = "ctx"
	raw, err := clientcmd.Write(*cfg)
	require.NoError(t, err)
	return string(raw)
}
