package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	runtimedomain "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

var (
	deploymentGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	serviceGVR    = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
)

func TestBuildClientsRejectsEmptyKubeconfig(t *testing.T) {
	_, _, err := buildClients(nil, "https://cluster-a.example")
	require.ErrorIs(t, err, runtimedomain.ErrKubeconfigMissing)
}

func TestBuildClientsRejectsServerMismatchWithoutLeakingCredentials(t *testing.T) {
	const secret = "super-secret-token"
	raw := kubeconfigForServer("https://cluster-a.example", secret)

	_, _, err := buildClients([]byte(raw), "https://cluster-b.example")

	require.ErrorIs(t, err, runtimedomain.ErrClusterServerMismatch)
	require.NotContains(t, err.Error(), secret)
}

func TestBuildClientsRejectsHTTPSAuthorizationDowngrade(t *testing.T) {
	raw := kubeconfigForServer("http://cluster-a.example:6443", "token")

	_, _, err := buildClients([]byte(raw), "https://cluster-a.example:6443")

	require.ErrorIs(t, err, runtimedomain.ErrClusterServerMismatch)
}

func TestBuildClientsAllowsAuthorizedHTTPOnlyWithMatchingHTTPServer(t *testing.T) {
	raw := kubeconfigForServer("http://CLUSTER-a.example:8080/", "token")

	typedClient, dynamicClient, err := buildClients([]byte(raw), "http://cluster-A.example:8080")

	require.NoError(t, err)
	require.NotNil(t, typedClient)
	require.NotNil(t, dynamicClient)
}

func TestBuildClientsRejectsInvalidKubeconfigWithoutLeakingCredentials(t *testing.T) {
	const secret = "super-secret-token"

	_, _, err := buildClients([]byte("token: "+secret+"\nnot: [valid"), "https://cluster-a.example")

	require.ErrorIs(t, err, runtimedomain.ErrKubeconfigInvalid)
	require.NotContains(t, err.Error(), secret)
}

func TestBuildClientsCreatesTypedAndDynamicClientsFromCompleteKubeconfig(t *testing.T) {
	raw := kubeconfigForServer("https://cluster-a.example", "token")

	typedClient, dynamicClient, err := buildClients([]byte(raw), "https://cluster-a.example")

	require.NoError(t, err)
	require.NotNil(t, typedClient)
	require.NotNil(t, dynamicClient)
}

func TestDeployCreatesMissingNamespaceBeforeApply(t *testing.T) {
	dyn := applyDynamicClient(t)
	typed := kubernetesfake.NewSimpleClientset(readyDeployment(2, 2, 1, nil), expectedService())
	deployer := NewManifestDeployer(dyn, typed, time.Millisecond)

	tgt := target(100 * time.Millisecond)
	err := deployer.Deploy(context.Background(), tgt, manifests())
	require.NoError(t, err)

	ns, err := typed.CoreV1().Namespaces().Get(context.Background(), tgt.Namespace, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, tgt.Namespace, ns.Name)
}

func TestDeployCreatesMissingImagePullSecretBeforeApply(t *testing.T) {
	dyn := applyDynamicClient(t)
	typed := kubernetesfake.NewSimpleClientset(readyDeployment(2, 2, 1, nil), expectedService())
	deployer := NewManifestDeployer(dyn, typed, time.Millisecond)

	tgt := target(100 * time.Millisecond)
	tgt.RegistryPull = &runtimeport.RegistryPullCredential{
		Server: "registry.example", Username: "user", Password: "pass",
	}
	manifest := deploymentYAMLWithPullSecret("registry.example/orders:deploy-123", "harbor-pull") +
		"\n---\n" + serviceYAML("")

	err := deployer.Deploy(context.Background(), tgt, []runtimeport.Manifest{{YAML: manifest}})
	require.NoError(t, err)

	secret, err := typed.CoreV1().Secrets(tgt.Namespace).Get(
		context.Background(), "harbor-pull", metav1.GetOptions{},
	)
	require.NoError(t, err)
	require.Equal(t, corev1.SecretTypeDockerConfigJson, secret.Type)
}

func deploymentYAMLWithPullSecret(image, secretName string) string {
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders-deployment
spec:
  selector:
    matchLabels:
      app: orders
  template:
    metadata:
      labels:
        app: orders
    spec:
      imagePullSecrets:
        - name: %s
      containers:
        - name: orders
          image: %s
`, secretName, image)
}

func TestDeployAppliesMultiDocumentManifestAndWaitsForReadyResources(t *testing.T) {
	dyn := applyDynamicClient(t)
	typed := kubernetesfake.NewSimpleClientset(readyDeployment(2, 2, 1, nil), expectedService())
	deployer := NewManifestDeployer(dyn, typed, time.Millisecond)

	err := deployer.Deploy(context.Background(), target(100*time.Millisecond), []runtimeport.Manifest{{
		YAML: deploymentYAML("registry.example/orders:deploy-123", "") + "\n---\n\n---\n" + serviceYAML(""),
	}})

	require.NoError(t, err)
	deployment, err := dyn.Resource(deploymentGVR).Namespace("sparkops-orders").Get(
		context.Background(), "orders-deployment", metav1.GetOptions{},
	)
	require.NoError(t, err)
	containers, found, err := unstructured.NestedSlice(
		deployment.Object, "spec", "template", "spec", "containers",
	)
	require.NoError(t, err)
	require.True(t, found)
	require.NotEmpty(t, containers)
	image := containers[0].(map[string]any)["image"].(string)
	require.Equal(t, "registry.example/orders:deploy-123", image)

	service, err := dyn.Resource(serviceGVR).Namespace("sparkops-orders").Get(
		context.Background(), "orders-service", metav1.GetOptions{},
	)
	require.NoError(t, err)
	require.Equal(t, "sparkops-orders", service.GetNamespace())

	var applyCount int
	for _, action := range dyn.Actions() {
		if patch, ok := action.(k8stesting.PatchAction); ok {
			require.Equal(t, types.ApplyPatchType, patch.GetPatchType())
			applyCount++
		}
	}
	require.Equal(t, 2, applyCount)
}

func TestDeployTreatsNilReplicasAsKubernetesDefaultOne(t *testing.T) {
	dyn := applyDynamicClient(t)
	typed := kubernetesfake.NewSimpleClientset(readyDeployment(1, 1, 1, nil), expectedService())

	err := NewManifestDeployer(dyn, typed, time.Millisecond).Deploy(
		context.Background(), target(50*time.Millisecond), manifests(),
	)

	require.NoError(t, err)
}

func TestDeployReturnsDeadlineWhenRolloutNeverReady(t *testing.T) {
	dyn := applyDynamicClient(t)
	replicas := int32(2)
	typed := kubernetesfake.NewSimpleClientset(readyDeployment(2, 1, 1, &replicas), expectedService())

	err := NewManifestDeployer(dyn, typed, time.Millisecond).Deploy(
		context.Background(), target(10*time.Millisecond), manifests(),
	)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestDeployDoesNotTreatDeletingDeploymentAsReady(t *testing.T) {
	dyn := applyDynamicClient(t)
	deployment := readyDeployment(2, 2, 1, nil)
	now := metav1.Now()
	deployment.DeletionTimestamp = &now
	typed := kubernetesfake.NewSimpleClientset(deployment, expectedService())

	err := NewManifestDeployer(dyn, typed, time.Millisecond).Deploy(
		context.Background(), target(10*time.Millisecond), manifests(),
	)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestDeployReturnsImmediatelyWhenDeploymentExceededProgressDeadline(t *testing.T) {
	dyn := applyDynamicClient(t)
	deployment := readyDeployment(2, 2, 1, nil)
	deployment.Status.Conditions = []appsv1.DeploymentCondition{{
		Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse,
		Reason: "ProgressDeadlineExceeded",
	}}
	typed := kubernetesfake.NewSimpleClientset(deployment, expectedService())

	err := NewManifestDeployer(dyn, typed, time.Hour).Deploy(
		context.Background(), target(20*time.Millisecond), manifests(),
	)

	require.ErrorIs(t, err, runtimedomain.ErrDeploymentProgressDeadlineExceeded)
	require.NotErrorIs(t, err, context.DeadlineExceeded)
}

func TestDeployStopsWhenContextIsCancelled(t *testing.T) {
	dyn := applyDynamicClient(t)
	typed := kubernetesfake.NewSimpleClientset(readyDeployment(2, 1, 0, nil), expectedService())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := NewManifestDeployer(dyn, typed, time.Hour).Deploy(ctx, target(time.Minute), manifests())

	require.ErrorIs(t, err, context.Canceled)
}

func TestDeployReturnsErrorWhenServiceIsMissing(t *testing.T) {
	dyn := applyDynamicClient(t)
	typed := kubernetesfake.NewSimpleClientset(readyDeployment(2, 2, 1, nil))

	err := NewManifestDeployer(dyn, typed, time.Millisecond).Deploy(
		context.Background(), target(50*time.Millisecond), manifests(),
	)

	require.Error(t, err)
	require.NotErrorIs(t, err, context.DeadlineExceeded)
}

func TestDeployRejectsDeletingService(t *testing.T) {
	dyn := applyDynamicClient(t)
	service := expectedService()
	now := metav1.Now()
	service.DeletionTimestamp = &now
	typed := kubernetesfake.NewSimpleClientset(readyDeployment(2, 2, 1, nil), service)

	err := NewManifestDeployer(dyn, typed, time.Millisecond).Deploy(
		context.Background(), target(50*time.Millisecond), manifests(),
	)

	require.ErrorIs(t, err, runtimedomain.ErrServiceDeleting)
}

func TestDeployRejectsMalformedUnknownAndCrossNamespaceDocuments(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected error
	}{
		{name: "malformed", yaml: "apiVersion: apps/v1\nkind: Deployment\nmetadata: [", expected: runtimedomain.ErrManifestInvalid},
		{name: "unknown", yaml: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: forbidden", expected: runtimedomain.ErrManifestUnsupported},
		{name: "cross namespace", yaml: deploymentYAML("repo/app:tag", "another-namespace"), expected: runtimedomain.ErrManifestNamespaceMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dyn := applyDynamicClient(t)
			deployer := NewManifestDeployer(dyn, kubernetesfake.NewSimpleClientset(), time.Millisecond)

			err := deployer.Deploy(context.Background(), target(20*time.Millisecond), []runtimeport.Manifest{{YAML: tt.yaml}})

			require.ErrorIs(t, err, tt.expected)
			require.Empty(t, dyn.Actions())
		})
	}
}

func TestApplyObjectsDoesNotPatchWhenLaterObjectCannotBeMarshaled(t *testing.T) {
	dyn := fake.NewSimpleDynamicClient(runtime.NewScheme())
	objects := []*unstructured.Unstructured{
		{Object: map[string]any{
			"apiVersion": "v1", "kind": "Service",
			"metadata": map[string]any{"name": "orders-service", "namespace": "sparkops-orders"},
		}},
		{Object: map[string]any{
			"apiVersion": "apps/v1", "kind": "Deployment",
			"metadata": map[string]any{"name": "orders-deployment", "namespace": "sparkops-orders"},
			"invalid":  func() {},
		}},
	}

	operations, prepareErr := prepareApplyOperations(objects)
	require.ErrorIs(t, prepareErr, runtimedomain.ErrManifestInvalid)
	require.Nil(t, operations)

	err := applyObjects(context.Background(), dyn, objects)
	require.ErrorIs(t, err, runtimedomain.ErrManifestInvalid)
	require.Empty(t, dyn.Actions())
}

func TestDeployRejectsDeploymentWithoutTargetImageTagBeforeApply(t *testing.T) {
	dyn := applyDynamicClient(t)
	deploymentTarget := target(time.Second)
	deploymentTarget.ImageTag = "another-tag"

	err := NewManifestDeployer(
		dyn, kubernetesfake.NewSimpleClientset(), time.Millisecond,
	).Deploy(context.Background(), deploymentTarget, manifests())

	require.ErrorIs(t, err, runtimedomain.ErrManifestImageTagMismatch)
	require.Empty(t, dyn.Actions())
}

func TestMockManifestDeployerValidatesExactTargetImageTag(t *testing.T) {
	deployer := NewMockManifestDeployer()
	validTarget := target(time.Second)

	empty := validTarget
	empty.Kubeconfig = ""
	require.ErrorIs(t, deployer.Deploy(context.Background(), empty, manifests()), runtimedomain.ErrKubeconfigMissing)

	require.ErrorIs(t,
		deployer.Deploy(context.Background(), validTarget, []runtimeport.Manifest{
			{YAML: deploymentYAML("registry/orders", "")},
			{YAML: serviceYAML("")},
		}),
		runtimedomain.ErrManifestImageTagMismatch,
	)

	emptyTag := validTarget
	emptyTag.ImageTag = ""
	require.ErrorIs(t,
		deployer.Deploy(context.Background(), emptyTag, manifests()),
		runtimedomain.ErrManifestImageTagMissing,
	)

	mismatch := validTarget
	mismatch.ImageTag = "release-456"
	require.ErrorIs(t,
		deployer.Deploy(context.Background(), mismatch, manifests()),
		runtimedomain.ErrManifestImageTagMismatch,
	)
	require.ErrorIs(t,
		deployer.Deploy(context.Background(), validTarget, []runtimeport.Manifest{
			{YAML: deploymentYAML("registry/orders@sha256:abcdef", "")},
			{YAML: serviceYAML("")},
		}),
		runtimedomain.ErrManifestImageTagMismatch,
	)

	require.NoError(t, deployer.Deploy(context.Background(), validTarget, []runtimeport.Manifest{
		{YAML: deploymentYAMLWithImages("registry/sidecar:other", "registry/orders:deploy-123")},
		{YAML: serviceYAML("")},
	}))
	require.NoError(t, deployer.Deploy(context.Background(), validTarget, manifests()))
}

func TestNewAdaptersInjectsMockManifestDeployer(t *testing.T) {
	adapters, err := NewAdapters(&config.Config{Runtime: config.RuntimeConfig{Driver: "mock"}}, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, adapters.Deployer)
	require.NoError(t, adapters.Deployer.Deploy(context.Background(), target(time.Second), manifests()))
}

func TestNewAdaptersUsesKubeconfigDeployerForK8sDriver(t *testing.T) {
	adapters, err := NewAdapters(&config.Config{Runtime: config.RuntimeConfig{Driver: "k8s"}}, nil, nil, nil, nil)
	require.NoError(t, err)
	_, ok := adapters.Deployer.(*ManifestDeployer)
	require.True(t, ok, "k8s driver should use kubeconfig manifest deployer")
}

func applyDynamicClient(t *testing.T) *fake.FakeDynamicClient {
	t.Helper()
	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	client.PrependReactor("patch", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		patch, ok := action.(k8stesting.PatchAction)
		if !ok {
			return true, nil, errors.New("expected patch action")
		}
		obj := &unstructured.Unstructured{}
		if err := json.Unmarshal(patch.GetPatch(), &obj.Object); err != nil {
			return true, nil, err
		}
		obj.SetNamespace(action.GetNamespace())
		gvr := action.GetResource()
		if err := client.Tracker().Create(gvr, obj.DeepCopy(), action.GetNamespace()); err != nil {
			return true, nil, err
		}
		return true, obj, nil
	})
	return client
}

func target(timeout time.Duration) runtimeport.DeploymentTarget {
	return runtimeport.DeploymentTarget{
		ClusterID: "cluster-a", Server: "https://cluster-a.example",
		Kubeconfig: "complete-kubeconfig", Namespace: "sparkops-orders",
		DeploymentName: "orders-deployment", ServiceName: "orders-service",
		ImageTag: "deploy-123", RolloutTimeout: timeout,
	}
}

func manifests() []runtimeport.Manifest {
	return []runtimeport.Manifest{
		{YAML: deploymentYAML("registry.example/orders:deploy-123", "")},
		{YAML: serviceYAML("")},
	}
}

func deploymentYAML(image, namespace string) string {
	namespaceLine := ""
	if namespace != "" {
		namespaceLine = "\n  namespace: " + namespace
	}
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders-deployment%s
spec:
  selector:
    matchLabels:
      app: orders
  template:
    metadata:
      labels:
        app: orders
    spec:
      containers:
        - name: orders
          image: %s
`, namespaceLine, image)
}

func deploymentYAMLWithImages(images ...string) string {
	var containers strings.Builder
	for i, image := range images {
		fmt.Fprintf(&containers, "        - name: container-%d\n          image: %s\n", i, image)
	}
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders-deployment
spec:
  selector:
    matchLabels:
      app: orders
  template:
    metadata:
      labels:
        app: orders
    spec:
      containers:
%s`, containers.String())
}

func serviceYAML(namespace string) string {
	namespaceLine := ""
	if namespace != "" {
		namespaceLine = "\n  namespace: " + namespace
	}
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: orders-service%s
spec:
  selector:
    app: orders
  ports:
    - port: 80
`, namespaceLine)
}

func readyDeployment(generation, observed int64, available int32, replicas *int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "orders-deployment", Namespace: "sparkops-orders", Generation: generation,
		},
		Spec: appsv1.DeploymentSpec{Replicas: replicas},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: observed, AvailableReplicas: available,
		},
	}
}

func expectedService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: "orders-service", Namespace: "sparkops-orders",
	}}
}

func kubeconfigForServer(server, token string) string {
	return strings.TrimSpace(fmt.Sprintf(`
apiVersion: v1
kind: Config
clusters:
  - name: target
    cluster:
      server: %s
contexts:
  - name: target
    context:
      cluster: target
      user: deployer
current-context: target
users:
  - name: deployer
    user:
      token: %s
`, server, token))
}
