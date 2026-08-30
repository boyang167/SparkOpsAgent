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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

func TestCloudTTYProvisionerCreatesTemporaryPodAndService(t *testing.T) {
	fixedNow := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app-0", Namespace: "default"},
		Spec: corev1.PodSpec{Containers: []corev1.Container{
			{Name: "app"},
			{Name: "sidecar"},
		}},
	})
	var createdPod *corev1.Pod
	var createdService *corev1.Service
	client.Fake.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		pod := action.(k8stesting.CreateAction).GetObject().(*corev1.Pod).DeepCopy()
		createdPod = pod.DeepCopy()
		pod.Name = pod.GenerateName + "abc123"
		pod.UID = types.UID("pod-uid")
		return true, pod, nil
	})
	client.Fake.PrependReactor("create", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
		svc := action.(k8stesting.CreateAction).GetObject().(*corev1.Service).DeepCopy()
		createdService = svc.DeepCopy()
		svc.Name = svc.GenerateName + "svc123"
		return true, svc, nil
	})

	session, err := NewProvisioner(ProvisionerOptions{Now: func() time.Time { return fixedNow }}).
		ProvisionCloudTTY(context.Background(), client, CloudTTYRequest{
			Namespace: "default",
			Pod:       "app-0",
			Container: "app",
		})

	require.NoError(t, err)
	require.NotEmpty(t, session.ID)
	require.Equal(t, "default", session.Pod.Namespace)
	require.Equal(t, "sparkops-cloudtty-abc123", session.Pod.Name)
	require.Equal(t, "default", session.Service.Namespace)
	require.Equal(t, "sparkops-cloudtty-svc123", session.Service.Name)
	require.Equal(t, fixedNow.Add(time.Hour), session.ExpiresAt)

	require.NotNil(t, createdPod)
	require.Empty(t, createdPod.Name)
	require.Equal(t, "sparkops-cloudtty-", createdPod.GenerateName)
	require.Equal(t, map[string]string{
		"app.kubernetes.io/managed-by": "sparkops-kubernetes-plugin",
		"session-id":                   session.ID,
	}, createdPod.Labels)
	require.Equal(t, int64(3600), *createdPod.Spec.ActiveDeadlineSeconds)
	require.Equal(t, corev1.RestartPolicyNever, createdPod.Spec.RestartPolicy)
	require.False(t, *createdPod.Spec.Containers[0].SecurityContext.Privileged)
	require.Contains(t, createdPod.Spec.Containers[0].Args, "app-0")
	require.Contains(t, createdPod.Spec.Containers[0].Args, "app")

	require.NotNil(t, createdService)
	require.Empty(t, createdService.Name)
	require.Equal(t, "sparkops-cloudtty-", createdService.GenerateName)
	require.Equal(t, createdPod.Labels, createdService.Labels)
	require.Len(t, createdService.OwnerReferences, 1)
	require.Equal(t, types.UID("pod-uid"), createdService.OwnerReferences[0].UID)
	require.False(t, *createdService.OwnerReferences[0].Controller)
	require.False(t, *createdService.OwnerReferences[0].BlockOwnerDeletion)
}

func TestCloudTTYProvisionerRejectsUnknownContainerBeforeCreatingResources(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app-0", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	})

	_, err := NewProvisioner(ProvisionerOptions{}).ProvisionCloudTTY(context.Background(), client, CloudTTYRequest{
		Namespace: "default",
		Pod:       "app-0",
		Container: "missing",
	})

	require.ErrorIs(t, err, ErrCloudTTYTargetInvalid)
	for _, action := range client.Actions() {
		require.NotEqual(t, "create", action.GetVerb())
	}
}

func TestCloudTTYProvisionerDeletesPodWhenServiceCreateFails(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app-0", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	})
	var deleteAction k8stesting.DeleteAction
	client.Fake.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		pod := action.(k8stesting.CreateAction).GetObject().(*corev1.Pod).DeepCopy()
		pod.Name = pod.GenerateName + "abc123"
		pod.UID = types.UID("pod-uid")
		return true, pod, nil
	})
	client.Fake.PrependReactor("create", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewInternalError(errors.New("service boom"))
	})
	client.Fake.PrependReactor("delete", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		deleteAction = action.(k8stesting.DeleteAction)
		return true, nil, nil
	})

	_, err := NewProvisioner(ProvisionerOptions{}).ProvisionCloudTTY(context.Background(), client, CloudTTYRequest{
		Namespace: "default",
		Pod:       "app-0",
		Container: "app",
	})

	require.ErrorIs(t, err, ErrCloudTTYServiceCreateFailed)
	require.NotNil(t, deleteAction)
	require.Equal(t, "sparkops-cloudtty-abc123", deleteAction.GetName())
	opts := deleteAction.GetDeleteOptions()
	require.Equal(t, int64(0), *opts.GracePeriodSeconds)
	require.Equal(t, metav1.DeletePropagationBackground, *opts.PropagationPolicy)
}

func TestNodeTTYProvisionerCreatesPrivilegedNsenterPod(t *testing.T) {
	fixedNow := time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC)
	client := fake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}})
	var createdPod *corev1.Pod
	client.Fake.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		pod := action.(k8stesting.CreateAction).GetObject().(*corev1.Pod).DeepCopy()
		createdPod = pod.DeepCopy()
		pod.Name = pod.GenerateName + "abc123"
		return true, pod, nil
	})

	session, err := NewProvisioner(ProvisionerOptions{Now: func() time.Time { return fixedNow }}).
		ProvisionNodeTTY(context.Background(), client, NodeTTYRequest{Node: "node-a"})

	require.NoError(t, err)
	require.NotEmpty(t, session.ID)
	require.Equal(t, NodeTTYNamespace, session.Pod.Namespace)
	require.Equal(t, "sparkops-nodetty-abc123", session.Pod.Name)
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
	client := fake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}})
	client.Fake.PrependReactor("create", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewInternalError(errors.New("pod boom"))
	})

	_, err := NewProvisioner(ProvisionerOptions{}).ProvisionNodeTTY(context.Background(), client, NodeTTYRequest{Node: "node-a"})

	require.ErrorIs(t, err, ErrNodeTTYPodCreateFailed)
	pods, listErr := client.CoreV1().Pods(NodeTTYNamespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, listErr)
	require.Empty(t, pods.Items)
}

func TestCloudTTYHTTPRequiresWriterBeforeClusterConnection(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	factory := &recordingClusterFactory{client: fake.NewSimpleClientset()}
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

func TestNodeTTYHTTPUsesAuthorizedWorkspaceCluster(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	client := fake.NewSimpleClientset()
	factory := &recordingClusterFactory{client: client}
	provisioner := &recordingProvisioner{session: Session{
		ID:        "session-1",
		Pod:       ResourceRef{Name: "sparkops-nodetty-abc123", Namespace: NodeTTYNamespace},
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
	require.Same(t, client, provisioner.client)
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
	called      bool
	workspaceID kernel.WorkspaceID
	clusterID   kernel.ClusterInfoID
	client      kubernetes.Interface
}

func (f *recordingClusterFactory) ForWorkspace(_ context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	f.called = true
	f.workspaceID = wsID
	f.clusterID = clusterID
	return f.client, nil, &rest.Config{Host: "https://kubernetes.example"}, nil, nil
}

func (f *recordingClusterFactory) ListMeta(context.Context, kernel.WorkspaceID) ([]sparkplugin.ClusterMeta, error) {
	return nil, errors.New("not used")
}

type recordingProvisioner struct {
	cloudTTYCalled bool
	nodeTTYCalled  bool
	client         kubernetes.Interface
	session        Session
}

func (p *recordingProvisioner) ProvisionCloudTTY(_ context.Context, client kubernetes.Interface, _ CloudTTYRequest) (Session, error) {
	p.cloudTTYCalled = true
	p.client = client
	return p.session, nil
}

func (p *recordingProvisioner) ProvisionNodeTTY(_ context.Context, client kubernetes.Interface, _ NodeTTYRequest) (Session, error) {
	p.nodeTTYCalled = true
	p.client = client
	return p.session, nil
}

type allowWriter struct {
	allow bool
}

func (w allowWriter) CanWrite(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return w.allow, nil
}
