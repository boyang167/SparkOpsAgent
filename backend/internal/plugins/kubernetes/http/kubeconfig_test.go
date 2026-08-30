package http

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func TestKubeconfigYAMLFromRESTIncludesTokenAndServer(t *testing.T) {
	yaml, err := kubeconfigYAMLFromREST(&rest.Config{
		Host:        "https://k8s.example:6443",
		BearerToken: "test-token",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}, "dev")
	require.NoError(t, err)
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(yaml))
	require.NoError(t, err)
	require.Equal(t, "https://k8s.example:6443", cfg.Host)
	require.Equal(t, "test-token", cfg.BearerToken)
	require.True(t, cfg.Insecure)
}

func TestKubeconfigYAMLFromRESTRejectsNilConfig(t *testing.T) {
	_, err := kubeconfigYAMLFromREST(nil, "dev")
	require.Error(t, err)
}
