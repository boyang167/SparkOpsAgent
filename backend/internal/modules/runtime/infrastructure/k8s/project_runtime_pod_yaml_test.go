package k8s

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMarshalPodYAMLIncludesContainerSpec(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repo-abc",
			Namespace: "test-workspace",
			Labels:    map[string]string{"app": "test-repo"},
		},
		Spec: corev1.PodSpec{
			NodeName: "k8smaster",
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
					},
				},
			}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	yamlText, err := marshalPodYAML(pod)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		"kind: Pod",
		"apiVersion: v1",
		"name: test-repo-abc",
		"bionet-env-config",
		"nodeName: k8smaster",
		"1Gi",
		"phase: Running",
	} {
		if !strings.Contains(yamlText, want) {
			t.Fatalf("expected YAML to contain %q, got:\n%s", want, yamlText)
		}
	}
}
