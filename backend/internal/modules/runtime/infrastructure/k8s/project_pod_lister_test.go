package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func podWithApp(name, app string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "shared", Labels: map[string]string{"app": app},
	}}
}

func TestListProjectPodsOnlyReturnsProjectPods(t *testing.T) {
	client := fake.NewSimpleClientset(
		podWithApp("orders-abc", "orders"),
		podWithApp("billing-xyz", "billing"),
	)

	pods, err := listProjectPods(context.Background(), client, kernel.NewProjectID(), "shared", "orders")

	require.NoError(t, err)
	require.Len(t, pods, 1)
	require.Equal(t, "orders-abc", pods[0].Name)
}

func TestListProjectPodsDoesNotLeakOtherProjectsWhenEmpty(t *testing.T) {
	client := fake.NewSimpleClientset(podWithApp("billing-xyz", "billing"))

	pods, err := listProjectPods(context.Background(), client, kernel.NewProjectID(), "shared", "orders")

	require.NoError(t, err)
	require.Empty(t, pods)
}
