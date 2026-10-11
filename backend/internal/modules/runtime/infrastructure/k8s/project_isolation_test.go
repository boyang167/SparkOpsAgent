package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type sharedNamespaceChecker struct{}

func (sharedNamespaceChecker) ProjectInWorkspace(context.Context, kernel.ProjectID, kernel.WorkspaceID) (bool, error) {
	return true, nil
}

func (sharedNamespaceChecker) ProjectName(context.Context, kernel.ProjectID) (string, error) {
	return "orders", nil
}

func (sharedNamespaceChecker) ProjectNamespace(context.Context, kernel.ProjectID, string) (string, error) {
	return "shared", nil
}

func deploymentWithApp(name, app string) *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "shared", Labels: map[string]string{"app": app},
	}}
}

func TestPodListerOnlyReturnsProjectPods(t *testing.T) {
	client := fake.NewSimpleClientset(podWithApp("orders-abc", "orders"), podWithApp("billing-xyz", "billing"))
	lister := NewPodListerWithClient(client, sharedNamespaceChecker{})

	pods, err := lister.ListByProject(context.Background(), kernel.NewProjectID(), "orders")

	require.NoError(t, err)
	require.Len(t, pods, 1)
	require.Equal(t, "orders-abc", pods[0].Name)
}

func TestResourceServiceIsolatesDeploymentsByProject(t *testing.T) {
	client := fake.NewSimpleClientset(
		deploymentWithApp("orders-deployment", "orders"),
		deploymentWithApp("billing-deployment", "billing"),
	)
	svc := &ResourceService{client: client, checker: sharedNamespaceChecker{}}
	ctx := context.Background()
	projectID := kernel.NewProjectID()

	items, err := svc.ListDeployments(ctx, projectID, "orders")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "orders-deployment", items[0]["name"])

	_, err = svc.GetDeployment(ctx, projectID, "orders", "billing-deployment")
	require.Error(t, err)

	require.Error(t, svc.ScaleDeployment(ctx, projectID, "orders", "billing-deployment", 0))
	billing, err := client.AppsV1().Deployments("shared").Get(ctx, "billing-deployment", metav1.GetOptions{})
	require.NoError(t, err)
	require.Nil(t, billing.Spec.Replicas)

	require.NoError(t, svc.ScaleDeployment(ctx, projectID, "orders", "orders-deployment", 2))
}
