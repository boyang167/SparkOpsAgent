package tty

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/clientcmd"
)

func TestCloudTTYProvisionerCreatesSecretAndCloudShellCR(t *testing.T) {
	fixedNow := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	typed := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app-0", Namespace: "team-a"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	})
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	restCfg := &rest.Config{
		Host:        "https://kubernetes.example",
		BearerToken: "bearer-secret",
		TLSClientConfig: rest.TLSClientConfig{
			CAData: []byte("ca-data"),
		},
	}
	var createdSecret *corev1.Secret
	var createdCR *unstructured.Unstructured
	var patchedSecret *corev1.Secret
	typed.Fake.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		secret := action.(k8stesting.CreateAction).GetObject().(*corev1.Secret).DeepCopy()
		createdSecret = secret.DeepCopy()
		secret.Name = secret.GenerateName + "secret123"
		secret.UID = types.UID("secret-uid")
		return true, secret, nil
	})
	typed.Fake.PrependReactor("patch", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		require.Equal(t, "sparkops-cloudtty-secret123", action.(k8stesting.PatchAction).GetName())
		require.Equal(t, types.MergePatchType, action.(k8stesting.PatchAction).GetPatchType())
		secret := createdSecret.DeepCopy()
		secret.Name = "sparkops-cloudtty-secret123"
		secret.OwnerReferences = []metav1.OwnerReference{{
			APIVersion:         CloudShellAPIVersion,
			Kind:               CloudShellKind,
			Name:               "sparkops-cloudtty-cr123",
			UID:                types.UID("cloudshell-uid"),
			Controller:         boolPtr(false),
			BlockOwnerDeletion: boolPtr(false),
		}}
		patchedSecret = secret
		return true, secret, nil
	})
	dynamicClient.PrependReactor("create", "cloudshells", func(action k8stesting.Action) (bool, runtime.Object, error) {
		cr := action.(k8stesting.CreateAction).GetObject().(*unstructured.Unstructured).DeepCopy()
		createdCR = cr.DeepCopy()
		cr.SetName(cr.GetGenerateName() + "cr123")
		cr.SetUID(types.UID("cloudshell-uid"))
		return true, cr, nil
	})

	session, err := NewProvisioner(ProvisionerOptions{Now: func() time.Time { return fixedNow }}).
		ProvisionCloudTTY(context.Background(), typed, dynamicClient, restCfg, CloudTTYRequest{
			Namespace: "team-a",
			Pod:       "app-0",
			Container: "app",
		})

	require.NoError(t, err)
	require.NotEmpty(t, session.ID)
	require.NotNil(t, session.CloudShell)
	require.Equal(t, "team-a", session.CloudShell.Namespace)
	require.Equal(t, "sparkops-cloudtty-cr123", session.CloudShell.Name)
	require.Equal(t, fixedNow.Add(30*time.Minute), session.ExpiresAt)
	require.Nil(t, session.Pod)
	require.Nil(t, session.Service)
	require.NotContains(t, jsonString(t, session), "secret")
	require.NotContains(t, jsonString(t, session), "bearer")
	require.NotContains(t, jsonString(t, session), "config")

	require.NotNil(t, createdSecret)
	require.Empty(t, createdSecret.Name)
	require.Equal(t, "sparkops-cloudtty-", createdSecret.GenerateName)
	require.Equal(t, "team-a", createdSecret.Namespace)
	require.Equal(t, corev1.SecretTypeOpaque, createdSecret.Type)
	require.Equal(t, sessionLabels(session.ID), createdSecret.Labels)
	require.Contains(t, string(createdSecret.Data["config"]), "https://kubernetes.example")
	require.Contains(t, string(createdSecret.Data["config"]), "bearer-secret")
	kubeconfig, loadErr := clientcmd.Load(createdSecret.Data["config"])
	require.NoError(t, loadErr)
	require.Equal(t, "team-a", kubeconfig.Contexts[kubeconfig.CurrentContext].Namespace)

	require.NotNil(t, createdCR)
	require.Empty(t, createdCR.GetName())
	require.Equal(t, "sparkops-cloudtty-", createdCR.GetGenerateName())
	require.Equal(t, "team-a", createdCR.GetNamespace())
	require.Equal(t, CloudShellAPIVersion, createdCR.GetAPIVersion())
	require.Equal(t, CloudShellKind, createdCR.GetKind())
	require.Equal(t, sessionLabels(session.ID), createdCR.GetLabels())
	require.Equal(t, "sparkops-cloudtty-secret123", nestedString(t, createdCR.Object, "spec", "secretRef", "name"))
	require.Equal(t, "kubectl exec -it -n team-a app-0 -c app -- sh", nestedString(t, createdCR.Object, "spec", "commandAction"))
	require.Equal(t, true, nestedBool(t, createdCR.Object, "spec", "cleanup"))
	require.Equal(t, int64(1800), nestedInt64(t, createdCR.Object, "spec", "ttlSecondsAfterStarted"))

	require.NotNil(t, patchedSecret)
	require.Len(t, patchedSecret.OwnerReferences, 1)
	require.Equal(t, types.UID("cloudshell-uid"), patchedSecret.OwnerReferences[0].UID)
}

func TestCloudTTYProvisionerMapsMissingCRDAndDeletesSecretWithBackgroundContext(t *testing.T) {
	reqCtx, cancel := context.WithCancel(context.Background())
	cancel()
	typed := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app-0", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	})
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	var deletedSecret bool
	typed.Fake.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		secret := action.(k8stesting.CreateAction).GetObject().(*corev1.Secret).DeepCopy()
		secret.Name = secret.GenerateName + "secret123"
		return true, secret, nil
	})
	typed.Fake.PrependReactor("delete", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		deletedSecret = true
		opts := action.(k8stesting.DeleteAction).GetDeleteOptions()
		require.Equal(t, int64(0), *opts.GracePeriodSeconds)
		require.Equal(t, metav1.DeletePropagationBackground, *opts.PropagationPolicy)
		return true, nil, nil
	})
	dynamicClient.PrependReactor("create", "cloudshells", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: CloudShellGroup, Resource: CloudShellResource}, "")
	})

	_, err := NewProvisioner(ProvisionerOptions{}).ProvisionCloudTTY(reqCtx, typed, dynamicClient, bearerRestConfig(), CloudTTYRequest{
		Namespace: "default",
		Pod:       "app-0",
		Container: "app",
	})

	require.ErrorIs(t, err, ErrCloudTTYCRDUnavailable)
	require.True(t, deletedSecret)
}

func TestCloudTTYProvisionerDeletesCloudShellAndSecretWhenOwnerPatchFails(t *testing.T) {
	typed := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app-0", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	})
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	var deletedSecret bool
	var deletedCR bool
	typed.Fake.PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		secret := action.(k8stesting.CreateAction).GetObject().(*corev1.Secret).DeepCopy()
		secret.Name = secret.GenerateName + "secret123"
		return true, secret, nil
	})
	dynamicClient.PrependReactor("create", "cloudshells", func(action k8stesting.Action) (bool, runtime.Object, error) {
		cr := action.(k8stesting.CreateAction).GetObject().(*unstructured.Unstructured).DeepCopy()
		cr.SetName(cr.GetGenerateName() + "cr123")
		cr.SetUID(types.UID("cloudshell-uid"))
		return true, cr, nil
	})
	typed.Fake.PrependReactor("patch", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewInternalError(errors.New("patch boom"))
	})
	typed.Fake.PrependReactor("delete", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		deletedSecret = true
		return true, nil, nil
	})
	dynamicClient.PrependReactor("delete", "cloudshells", func(k8stesting.Action) (bool, runtime.Object, error) {
		deletedCR = true
		return true, nil, nil
	})

	_, err := NewProvisioner(ProvisionerOptions{}).ProvisionCloudTTY(context.Background(), typed, dynamicClient, bearerRestConfig(), CloudTTYRequest{
		Namespace: "default",
		Pod:       "app-0",
		Container: "app",
	})

	require.ErrorIs(t, err, ErrCloudTTYOwnerPatchFailed)
	require.True(t, deletedCR)
	require.True(t, deletedSecret)
}

func TestCloudTTYProvisionerRejectsUnknownContainerBeforeCreatingResources(t *testing.T) {
	typed := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app-0", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	})
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

	_, err := NewProvisioner(ProvisionerOptions{}).ProvisionCloudTTY(context.Background(), typed, dynamicClient, bearerRestConfig(), CloudTTYRequest{
		Namespace: "default",
		Pod:       "app-0",
		Container: "missing",
	})

	require.ErrorIs(t, err, ErrCloudTTYTargetInvalid)
	for _, action := range append(typed.Actions(), dynamicClient.Actions()...) {
		require.NotEqual(t, "create", action.GetVerb())
	}
}

func TestNodeTTYProvisionerCreatesPrivilegedNsenterPod(t *testing.T) {
	fixedNow := time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC)
	typed := fake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}})
	var createdPod *corev1.Pod
	typed.Fake.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		pod := action.(k8stesting.CreateAction).GetObject().(*corev1.Pod).DeepCopy()
		createdPod = pod.DeepCopy()
		pod.Name = pod.GenerateName + "abc123"
		return true, pod, nil
	})

	session, err := NewProvisioner(ProvisionerOptions{Now: func() time.Time { return fixedNow }}).
		ProvisionNodeTTY(context.Background(), typed, nil, nil, NodeTTYRequest{Node: "node-a"})

	require.NoError(t, err)
	require.NotEmpty(t, session.ID)
	require.NotNil(t, session.Pod)
	require.Equal(t, NodeTTYNamespace, session.Pod.Namespace)
	require.Equal(t, "sparkops-nodetty-abc123", session.Pod.Name)
	require.Nil(t, session.CloudShell)
	require.Nil(t, session.Service)
	require.Equal(t, fixedNow.Add(30*time.Minute), session.ExpiresAt)

	require.NotNil(t, createdPod)
	require.Empty(t, createdPod.Name)
	require.Equal(t, "sparkops-nodetty-", createdPod.GenerateName)
	require.Equal(t, NodeTTYNamespace, createdPod.Namespace)
	require.Equal(t, "node-a", createdPod.Spec.NodeName)
	require.True(t, createdPod.Spec.HostPID)
	require.False(t, createdPod.Spec.HostNetwork)
	require.Equal(t, int64(1800), *createdPod.Spec.ActiveDeadlineSeconds)
	require.Equal(t, corev1.RestartPolicyNever, createdPod.Spec.RestartPolicy)
	require.Equal(t, []string{"nsenter"}, createdPod.Spec.Containers[0].Command)
	require.Equal(t, []string{"-t", "1", "-m", "-u", "-i", "-n", "sleep", "1800"}, createdPod.Spec.Containers[0].Args)
	require.True(t, *createdPod.Spec.Containers[0].SecurityContext.Privileged)
}

func TestNodeTTYProvisionerCreateFailureLeavesNoPod(t *testing.T) {
	typed := fake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}})
	typed.Fake.PrependReactor("create", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewInternalError(errors.New("pod boom"))
	})

	_, err := NewProvisioner(ProvisionerOptions{}).ProvisionNodeTTY(context.Background(), typed, nil, nil, NodeTTYRequest{Node: "node-a"})

	require.ErrorIs(t, err, ErrNodeTTYPodCreateFailed)
	pods, listErr := typed.CoreV1().Pods(NodeTTYNamespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, listErr)
	require.Empty(t, pods.Items)
}

func TestCloudTTYHTTPRequiresWriterBeforeClusterConnection(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	factory := &recordingClusterFactory{client: fake.NewSimpleClientset(), dynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())}
	provisioner := &recordingProvisioner{}
	engine := ttyTestEngine(memberID, sparkplugin.PluginContext{
		Clusters: factory,
		Writer:   allowWriter{allow: false},
	}, provisioner)

	rec := performJSON(engine, http.MethodPost, "/clusters/"+clusterID.String()+"/cloudtty", wsID, map[string]string{
		"namespace": "default",
		"pod":       "app-0",
		"container": "app",
	})

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.False(t, factory.called)
	require.False(t, provisioner.cloudTTYCalled)
}

func TestCloudTTYHTTPMapsMissingCRDToFixedGatewayError(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	factory := &recordingClusterFactory{client: fake.NewSimpleClientset(), dynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())}
	provisioner := &recordingProvisioner{err: ErrCloudTTYCRDUnavailable}
	engine := ttyTestEngine(memberID, sparkplugin.PluginContext{
		Clusters: factory,
		Writer:   allowWriter{allow: true},
	}, provisioner)

	rec := performJSON(engine, http.MethodPost, "/clusters/"+clusterID.String()+"/cloudtty", wsID, map[string]string{
		"namespace": "default",
		"pod":       "app-0",
		"container": "app",
	})

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), "PLUGIN_CLOUDTTY_CRD_UNAVAILABLE")
}

func TestNodeTTYHTTPUsesAuthorizedWorkspaceCluster(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	typed := fake.NewSimpleClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	restCfg := bearerRestConfig()
	factory := &recordingClusterFactory{client: typed, dynamicClient: dynamicClient, restConfig: restCfg}
	provisioner := &recordingProvisioner{session: Session{
		ID:        "session-1",
		Pod:       &ResourceRef{Name: "sparkops-nodetty-abc123", Namespace: NodeTTYNamespace},
		ExpiresAt: time.Date(2026, 8, 30, 11, 30, 0, 0, time.UTC),
	}}
	engine := ttyTestEngine(memberID, sparkplugin.PluginContext{
		Clusters: factory,
		Writer:   allowWriter{allow: true},
	}, provisioner)

	rec := performJSON(engine, http.MethodPost, "/clusters/"+clusterID.String()+"/nodetty", wsID, map[string]string{
		"node": "node-a",
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, factory.called)
	require.Equal(t, wsID, factory.workspaceID)
	require.Equal(t, clusterID, factory.clusterID)
	require.True(t, provisioner.nodeTTYCalled)
	require.Same(t, typed, provisioner.client)
	require.Same(t, dynamicClient, provisioner.dynamicClient)
	require.Same(t, restCfg, provisioner.restConfig)
	require.NotContains(t, rec.Body.String(), "token")
	require.NotContains(t, rec.Body.String(), "config")
	require.Contains(t, rec.Body.String(), "sparkops-nodetty-abc123")
}

func ttyTestEngine(memberID kernel.MemberID, ctx sparkplugin.PluginContext, provisioner Provisioner) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	engine.Use(httpx.RequireWorkspace())
	handler := NewHandlerWithProvisioner(ctx, provisioner)
	engine.POST("/clusters/:clusterId/cloudtty", handler.CloudTTY)
	engine.POST("/clusters/:clusterId/nodetty", handler.NodeTTY)
	return engine
}

func performJSON(engine *gin.Engine, method, path string, wsID kernel.WorkspaceID, payload any) *httptest.ResponseRecorder {
	var body bytes.Buffer
	if payload != nil {
		_ = json.NewEncoder(&body).Encode(payload)
	}
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("X-Workspace-Id", wsID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

type recordingClusterFactory struct {
	called        bool
	workspaceID   kernel.WorkspaceID
	clusterID     kernel.ClusterInfoID
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
	restConfig    *rest.Config
}

func (f *recordingClusterFactory) ForWorkspace(_ context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	f.called = true
	f.workspaceID = wsID
	f.clusterID = clusterID
	restCfg := f.restConfig
	if restCfg == nil {
		restCfg = bearerRestConfig()
	}
	return f.client, f.dynamicClient, restCfg, nil, nil
}

func (f *recordingClusterFactory) ListMeta(context.Context, kernel.WorkspaceID) ([]sparkplugin.ClusterMeta, error) {
	return nil, errors.New("not used")
}

type recordingProvisioner struct {
	cloudTTYCalled bool
	nodeTTYCalled  bool
	client         kubernetes.Interface
	dynamicClient  dynamic.Interface
	restConfig     *rest.Config
	session        Session
	err            error
}

func (p *recordingProvisioner) ProvisionCloudTTY(_ context.Context, client kubernetes.Interface, dynamicClient dynamic.Interface, restConfig *rest.Config, _ CloudTTYRequest) (Session, error) {
	p.cloudTTYCalled = true
	p.client = client
	p.dynamicClient = dynamicClient
	p.restConfig = restConfig
	return p.session, p.err
}

func (p *recordingProvisioner) ProvisionNodeTTY(_ context.Context, client kubernetes.Interface, dynamicClient dynamic.Interface, restConfig *rest.Config, _ NodeTTYRequest) (Session, error) {
	p.nodeTTYCalled = true
	p.client = client
	p.dynamicClient = dynamicClient
	p.restConfig = restConfig
	return p.session, p.err
}

type allowWriter struct {
	allow bool
}

func (w allowWriter) CanWrite(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return w.allow, nil
}

func bearerRestConfig() *rest.Config {
	return &rest.Config{
		Host:        "https://kubernetes.example",
		BearerToken: "token",
		TLSClientConfig: rest.TLSClientConfig{
			CAData: []byte("ca"),
		},
	}
}

func jsonString(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return string(raw)
}

func nestedString(t *testing.T, obj map[string]any, fields ...string) string {
	t.Helper()
	value, found, err := unstructured.NestedString(obj, fields...)
	require.NoError(t, err)
	require.True(t, found)
	return value
}

func nestedBool(t *testing.T, obj map[string]any, fields ...string) bool {
	t.Helper()
	value, found, err := unstructured.NestedBool(obj, fields...)
	require.NoError(t, err)
	require.True(t, found)
	return value
}

func nestedInt64(t *testing.T, obj map[string]any, fields ...string) int64 {
	t.Helper()
	value, found, err := unstructured.NestedInt64(obj, fields...)
	require.NoError(t, err)
	require.True(t, found)
	return value
}

func boolPtr(value bool) *bool {
	return &value
}
