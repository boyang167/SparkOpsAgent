package k8s

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfigMapBelongsToProject(t *testing.T) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-repo", Labels: map[string]string{"app": "test-repo"}}}
	if !configMapBelongsToProject(cm, "test-repo", nil) {
		t.Fatal("expected name+label match")
	}
	other := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other"}}
	if configMapBelongsToProject(other, "test-repo", nil) {
		t.Fatal("expected unrelated configmap to be excluded")
	}
	ref := map[string]struct{}{"mounted": {}}
	mounted := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "mounted"}}
	if !configMapBelongsToProject(mounted, "test-repo", ref) {
		t.Fatal("expected referenced configmap")
	}
}

func TestCollectDeploymentConfigMaps(t *testing.T) {
	dep := &appsv1.Deployment{}
	dep.Spec.Template.Spec.Volumes = []corev1.Volume{{
		Name: "cfg",
		VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: "app-cfg"},
		}},
	}}
	dep.Spec.Template.Spec.Containers = []corev1.Container{{
		EnvFrom: []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: "env-cfg"},
		}}},
	}}
	out := map[string]struct{}{}
	collectDeploymentConfigMaps(dep, out)
	if _, ok := out["app-cfg"]; !ok {
		t.Fatal("missing volume configmap")
	}
	if _, ok := out["env-cfg"]; !ok {
		t.Fatal("missing envFrom configmap")
	}
}
