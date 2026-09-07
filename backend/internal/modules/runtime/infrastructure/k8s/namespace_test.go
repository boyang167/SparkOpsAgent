package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

func TestEnsureNamespaceCreatesMissingNamespace(t *testing.T) {
	client := kubernetesfake.NewSimpleClientset()

	require.NoError(t, ensureNamespace(context.Background(), client, "team-a"))

	ns, err := client.CoreV1().Namespaces().Get(context.Background(), "team-a", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "team-a", ns.Name)
}

func TestEnsureNamespaceSkipsExistingNamespace(t *testing.T) {
	client := kubernetesfake.NewSimpleClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a"},
	})

	require.NoError(t, ensureNamespace(context.Background(), client, "team-a"))

	list, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
}

func TestEnsureNamespaceRejectsEmptyName(t *testing.T) {
	client := kubernetesfake.NewSimpleClientset()
	require.Error(t, ensureNamespace(context.Background(), client, "  "))
}
