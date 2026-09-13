package k8s

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMarshalDeploymentYAMLIncludesContainerSpec(t *testing.T) {
	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repo",
			Namespace: "test-workspace",
			Labels:    map[string]string{"app": "test-repo", "language": "go"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test-repo"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test-repo"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "test-repo",
						Image: "192.168.5.24/test-workspace/test-repo:master",
						EnvFrom: []corev1.EnvFromSource{{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "bionet-env-config"},
							},
						}},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1Gi"),
								corev1.ResourceCPU:    resource.MustParse("500m"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("256Mi"),
								corev1.ResourceCPU:    resource.MustParse("100m"),
							},
						},
					}},
				},
			},
		},
	}

	yamlText, err := marshalDeploymentYAML(dep)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		"kind: Deployment",
		"apiVersion: apps/v1",
		"name: test-repo",
		"bionet-env-config",
		"1Gi",
		"500m",
		"256Mi",
		"100m",
	} {
		if !strings.Contains(yamlText, want) {
			t.Fatalf("expected YAML to contain %q, got:\n%s", want, yamlText)
		}
	}
}
