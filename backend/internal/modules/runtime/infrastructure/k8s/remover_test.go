package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
)

const removerNS = "team-a"

func removerLabels() map[string]string {
	return map[string]string{"app": "orders", "app.kubernetes.io/managed-by": "sparkops"}
}

func removerFixture() *fake.Clientset {
	return fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: removerNS, Labels: removerLabels()},
			Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "orders-data"},
				}}},
			}}},
		},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: removerNS}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "orders-ing", Namespace: removerNS, Labels: removerLabels()}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "orders-config", Namespace: removerNS, Labels: removerLabels()}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "orders-secret", Namespace: removerNS, Labels: removerLabels()}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "harbor-pull", Namespace: removerNS}, Type: corev1.SecretTypeDockerConfigJson},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "orders-data", Namespace: removerNS}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: removerNS}},
	)
}

func removerTarget() port.DeploymentTarget {
	return port.DeploymentTarget{Kubeconfig: "test", Namespace: removerNS}
}

func removerManifests() []port.Manifest {
	return []port.Manifest{
		{YAML: "apiVersion: v1\nkind: Service\nmetadata:\n  name: orders\n"},
		{YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: orders\n"},
	}
}

func TestRemoveDeletesWorkloadsAndKeepsConfigByDefault(t *testing.T) {
	client := removerFixture()
	remover := NewManifestDeployer(nil, client, 0)

	err := remover.Remove(context.Background(), removerTarget(), port.RemoveSpec{
		Manifests: removerManifests(), AppLabel: "orders",
	})

	require.NoError(t, err)
	ctx := context.Background()
	_, err = client.AppsV1().Deployments(removerNS).Get(ctx, "orders", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	_, err = client.CoreV1().Services(removerNS).Get(ctx, "orders", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	_, err = client.NetworkingV1().Ingresses(removerNS).Get(ctx, "orders-ing", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	_, err = client.CoreV1().ConfigMaps(removerNS).Get(ctx, "orders-config", metav1.GetOptions{})
	require.NoError(t, err)
	_, err = client.CoreV1().PersistentVolumeClaims(removerNS).Get(ctx, "orders-data", metav1.GetOptions{})
	require.NoError(t, err)
	_, err = client.CoreV1().Services(removerNS).Get(ctx, "other", metav1.GetOptions{})
	require.NoError(t, err, "unrelated services must be kept")
}

func TestRemoveDeletesConfigAndStorageWhenRequested(t *testing.T) {
	client := removerFixture()
	remover := NewManifestDeployer(nil, client, 0)

	err := remover.Remove(context.Background(), removerTarget(), port.RemoveSpec{
		Manifests: removerManifests(), AppLabel: "orders", DeleteConfig: true, DeleteStorage: true,
	})

	require.NoError(t, err)
	ctx := context.Background()
	_, err = client.CoreV1().ConfigMaps(removerNS).Get(ctx, "orders-config", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	_, err = client.CoreV1().Secrets(removerNS).Get(ctx, "orders-secret", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	_, err = client.CoreV1().Secrets(removerNS).Get(ctx, "harbor-pull", metav1.GetOptions{})
	require.NoError(t, err, "image pull secret must be kept")
	_, err = client.CoreV1().PersistentVolumeClaims(removerNS).Get(ctx, "orders-data", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
}

func TestRemoveIsIdempotentWhenResourcesMissing(t *testing.T) {
	remover := NewManifestDeployer(nil, fake.NewSimpleClientset(), 0)

	err := remover.Remove(context.Background(), removerTarget(), port.RemoveSpec{
		Manifests: removerManifests(), AppLabel: "orders", DeleteConfig: true, DeleteStorage: true,
	})

	require.NoError(t, err)
}

func TestRemoveFallsBackToAppLabelWhenManifestsEmpty(t *testing.T) {
	client := removerFixture()
	remover := NewManifestDeployer(nil, client, 0)

	require.NoError(t, remover.Remove(context.Background(), removerTarget(), port.RemoveSpec{AppLabel: "orders"}))
	_, err := client.AppsV1().Deployments(removerNS).Get(context.Background(), "orders", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
}

func TestRemoveRejectsEmptyNamespace(t *testing.T) {
	remover := NewManifestDeployer(nil, removerFixture(), 0)

	err := remover.Remove(context.Background(), port.DeploymentTarget{Kubeconfig: "test"}, port.RemoveSpec{AppLabel: "orders"})

	require.Error(t, err)
}
