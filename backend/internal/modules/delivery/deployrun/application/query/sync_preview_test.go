package query

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/stretchr/testify/require"
)

func TestRelatedResourceReason(t *testing.T) {
	require.Equal(t, "referenced by Deployment volume", relatedResourceReason(
		"app-cfg", nil, "demo", map[string]string{"app-cfg": "referenced by Deployment volume"},
	))
	require.Equal(t, "name prefix", relatedResourceReason("demo-extra", nil, "demo", nil))
	require.Equal(t, "label match", relatedResourceReason("other", map[string]string{"app": "demo"}, "demo", nil))
	require.Empty(t, relatedResourceReason("unrelated", nil, "demo", nil))
}

func TestRelatedIngressReasonBackend(t *testing.T) {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ext"},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{Name: "demo"},
							},
						}},
					},
				},
			}},
		},
	}
	require.Equal(t, "ingress backend", relatedIngressReason(ing, "demo", "demo"))
}

func TestExcludeSystemSecret(t *testing.T) {
	require.True(t, excludeSystemSecret(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "default-token-abc"},
		Type:       corev1.SecretTypeServiceAccountToken,
	}))
	require.True(t, excludeSystemSecret(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "sh.helm.release.v1.demo.v1"},
		Type:       "helm.sh/release.v1",
	}))
	require.False(t, excludeSystemSecret(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-tls"},
		Type:       corev1.SecretTypeOpaque,
	}))
}

func TestReferencedFromDeployment(t *testing.T) {
	dep := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{{Name: "registry-pull"}},
					Volumes: []corev1.Volume{{
						Name: "cfg",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "app-cfg"},
							},
						},
					}},
					Containers: []corev1.Container{{
						Name: "app",
						EnvFrom: []corev1.EnvFromSource{{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "app-secret"},
							},
						}},
					}},
				},
			},
		},
	}
	cms, secrets := referencedFromDeployment(dep)
	require.Equal(t, "referenced by Deployment volume", cms["app-cfg"])
	require.Equal(t, "referenced by Deployment envFrom", secrets["app-secret"])
	require.Equal(t, []string{"registry-pull"}, imagePullSecretNames(dep))
}

func TestRewriteContainerImageTags(t *testing.T) {
	raw := "apiVersion: apps/v1\nkind: " + model.SyncManifestKindDeployment + "\n" +
		"metadata:\n  name: demo\n" +
		"spec:\n  template:\n    spec:\n      containers:\n" +
		"        - name: app\n          image: harbor.example/demo:old\n"
	out := rewriteContainerImageTags(raw, "harbor.example/demo", "v2")
	require.Contains(t, out, "harbor.example/demo:v2")
	require.NotContains(t, out, ":old")
}
