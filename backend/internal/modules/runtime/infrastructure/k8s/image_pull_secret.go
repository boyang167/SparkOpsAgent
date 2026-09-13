package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
)

type dockerConfig struct {
	Auths map[string]dockerAuthEntry `json:"auths"`
}

type dockerAuthEntry struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

func extractImagePullSecretNames(objects []*unstructured.Unstructured) []string {
	seen := map[string]struct{}{}
	names := make([]string, 0)
	for _, object := range objects {
		if object.GetKind() != "Deployment" {
			continue
		}
		pullSecrets, found, err := unstructured.NestedSlice(
			object.Object, "spec", "template", "spec", "imagePullSecrets",
		)
		if err != nil || !found {
			continue
		}
		for _, item := range pullSecrets {
			secretMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name, _ := secretMap["name"].(string)
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	return names
}

func appendUniqueStrings(base []string, extra ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(base)+len(extra))
	for _, name := range append(base, extra...) {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func ensureImagePullSecrets(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	secretNames []string,
	credential *port.RegistryPullCredential,
) error {
	if len(secretNames) == 0 || credential == nil {
		return nil
	}
	server := strings.TrimSpace(credential.Server)
	username := strings.TrimSpace(credential.Username)
	password := credential.Password
	if server == "" || username == "" || password == "" {
		return fmt.Errorf("registry pull credentials are incomplete")
	}
	payload, err := buildDockerConfigJSON(server, username, password)
	if err != nil {
		return err
	}
	for _, name := range secretNames {
		_, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			continue
		}
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("read image pull secret %q: %w", name, err)
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: payload,
			},
		}
		if _, err := client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create image pull secret %q: %w", name, err)
		}
	}
	return nil
}

func buildDockerConfigJSON(server, username, password string) ([]byte, error) {
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	cfg := dockerConfig{
		Auths: map[string]dockerAuthEntry{
			server: {
				Username: username,
				Password: password,
				Auth:     auth,
			},
		},
	}
	return json.Marshal(cfg)
}

func registryHostFromURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return strings.Trim(trimmed, "/")
	}
	return parsed.Host
}
