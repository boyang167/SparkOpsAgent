package k8s_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	runtimek8s "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type stubChecker struct {
	namespace string
}

func (s stubChecker) ProjectInWorkspace(context.Context, kernel.ProjectID, kernel.WorkspaceID) (bool, error) {
	return true, nil
}

func (s stubChecker) ProjectName(context.Context, kernel.ProjectID) (string, error) {
	return "demo-api", nil
}

func (s stubChecker) ProjectNamespace(context.Context, kernel.ProjectID, string) (string, error) {
	return s.namespace, nil
}

func TestPodLister_ListByProject_maps_pods(t *testing.T) {
	projectID := kernel.NewProjectID()
	ns := "sparkops-demo"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-api-0",
			Namespace: ns,
			Labels:    map[string]string{"app": "demo-api"},
		},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
			Containers: []corev1.Container{{
				Name:  "demo-api",
				Image: "sparkops/demo-api:1.0",
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "demo-api",
				RestartCount: 2,
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{
						StartedAt: metav1.Now(),
					},
				},
			}},
		},
	}

	client := fake.NewSimpleClientset(pod)
	lister := runtimek8s.NewPodListerWithClient(client, stubChecker{namespace: ns})

	pods, err := lister.ListByProject(context.Background(), projectID, "demo-api")
	require.NoError(t, err)
	require.Len(t, pods, 1)
	require.Equal(t, "demo-api-0", pods[0].Name)
	require.Equal(t, ns, pods[0].Namespace)
	require.Equal(t, "Running", string(pods[0].Phase))
	require.Equal(t, 2, pods[0].Restarts)

	found, err := lister.FindByID(context.Background(), projectID, "demo-api", pods[0].ID)
	require.NoError(t, err)
	require.Equal(t, pods[0].Name, found.Name)
}

func TestPodLogReader_ReadLogs(t *testing.T) {
	projectID := kernel.NewProjectID()
	ns := "sparkops-demo"
	podObj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-api-0", Namespace: ns, Labels: map[string]string{"app": "demo-api"}},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "demo-api"}}},
	}
	client := fake.NewSimpleClientset(podObj)
	lister := runtimek8s.NewPodListerWithClient(client, stubChecker{namespace: ns})
	logs := runtimek8s.NewPodLogReaderWithClient(client)

	pods, err := lister.ListByProject(context.Background(), projectID, "demo-api")
	require.NoError(t, err)
	require.Len(t, pods, 1)

	text, err := logs.ReadLogs(context.Background(), pods[0])
	require.NoError(t, err)
	require.NotEmpty(t, text)
}

func TestDefaultNamespace(t *testing.T) {
	require.Equal(t, "sparkops-demo-api", runtimek8s.DefaultNamespace("Demo-API"))
	require.Equal(t, "sparkops-demo", runtimek8s.DefaultNamespace(""))
}
