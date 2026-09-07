package k8s

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"

	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
)

func TestExtractImagePullSecretNames(t *testing.T) {
	objects := []*unstructured.Unstructured{{
		Object: map[string]any{
			"kind": "Deployment",
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"imagePullSecrets": []any{
							map[string]any{"name": "pull-a"},
							map[string]any{"name": "pull-a"},
							map[string]any{"name": "pull-b"},
						},
					},
				},
			},
		},
	}}

	names := extractImagePullSecretNames(objects)
	require.Equal(t, []string{"pull-a", "pull-b"}, names)
}

func TestEnsureImagePullSecretsCreatesMissingSecret(t *testing.T) {
	client := kubernetesfake.NewSimpleClientset()
	cred := &runtimeport.RegistryPullCredential{
		Server: "registry.example", Username: "user", Password: "pass",
	}

	err := ensureImagePullSecrets(
		context.Background(),
		client,
		"team-a",
		[]string{"harbor-pull"},
		cred,
	)
	require.NoError(t, err)

	secret, err := client.CoreV1().Secrets("team-a").Get(
		context.Background(), "harbor-pull", metav1.GetOptions{},
	)
	require.NoError(t, err)
	require.Equal(t, corev1.SecretTypeDockerConfigJson, secret.Type)

	var cfg dockerConfig
	require.NoError(t, json.Unmarshal(secret.Data[corev1.DockerConfigJsonKey], &cfg))
	require.Equal(t, "user", cfg.Auths["registry.example"].Username)
	require.Equal(t, "pass", cfg.Auths["registry.example"].Password)
}

func TestEnsureImagePullSecretsSkipsExistingSecret(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "harbor-pull", Namespace: "team-a"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example":{"username":"old"}}}`),
		},
	}
	client := kubernetesfake.NewSimpleClientset(existing)
	cred := &runtimeport.RegistryPullCredential{
		Server: "registry.example", Username: "new", Password: "pass",
	}

	require.NoError(t, ensureImagePullSecrets(
		context.Background(), client, "team-a", []string{"harbor-pull"}, cred,
	))

	secret, err := client.CoreV1().Secrets("team-a").Get(
		context.Background(), "harbor-pull", metav1.GetOptions{},
	)
	require.NoError(t, err)
	require.Contains(t, string(secret.Data[corev1.DockerConfigJsonKey]), `"username":"old"`)
}

func TestBuildDockerConfigJSON(t *testing.T) {
	raw, err := buildDockerConfigJSON("192.168.5.24", "admin", "secret")
	require.NoError(t, err)

	var cfg dockerConfig
	require.NoError(t, json.Unmarshal(raw, &cfg))
	entry := cfg.Auths["192.168.5.24"]
	require.Equal(t, "admin", entry.Username)
	require.Equal(t, "secret", entry.Password)
	require.NotEmpty(t, entry.Auth)
}

func TestRegistryHostFromURL(t *testing.T) {
	require.Equal(t, "192.168.5.24", registryHostFromURL("http://192.168.5.24"))
	require.Equal(t, "harbor.local:443", registryHostFromURL("https://harbor.local:443/v2/"))
}
