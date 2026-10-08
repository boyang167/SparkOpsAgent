package k8s

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestGetStorageOverviewReturnsPVCsAndStorageClasses(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
			Name: "orders-data", Namespace: "team-a", Labels: map[string]string{"app": "orders"},
		}},
		&storagev1.StorageClass{
			ObjectMeta:  metav1.ObjectMeta{Name: "fast"},
			Provisioner: "example.com/fast",
		},
	)
	session := &ProjectClusterSession{
		Client: client, Namespace: "team-a", ProjectName: "orders",
	}

	overview, err := getStorageOverviewWithSession(context.Background(), session)

	require.NoError(t, err)
	require.Len(t, overview.Items, 1)
	require.Len(t, overview.Classes, 1)
	require.Empty(t, overview.Warnings)
}

func TestGetStorageOverviewKeepsPVCsWhenStorageClassesFail(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: "orders-data", Namespace: "team-a", Labels: map[string]string{"app": "orders"},
	}})
	client.PrependReactor(
		"list",
		"storageclasses",
		func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("storage API unavailable")
		},
	)
	session := &ProjectClusterSession{
		Client: client, Namespace: "team-a", ProjectName: "orders",
	}

	overview, err := getStorageOverviewWithSession(context.Background(), session)

	require.NoError(t, err)
	require.Len(t, overview.Items, 1)
	require.Empty(t, overview.Classes)
	require.Equal(t, []string{"StorageClass 加载失败：storage API unavailable"}, overview.Warnings)
}

func TestPVCBelongsToProject(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "test-repo", Labels: map[string]string{"app": "test-repo"}}}
	if !pvcBelongsToProject(pvc, "test-repo", nil) {
		t.Fatal("expected name+label match")
	}
	other := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "other"}}
	if pvcBelongsToProject(other, "test-repo", nil) {
		t.Fatal("expected unrelated pvc to be excluded")
	}
	mounted := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data"}}
	if !pvcBelongsToProject(mounted, "test-repo", map[string]pvcMountInfo{"data": {VolumeName: "data", MountPath: "/data"}}) {
		t.Fatal("expected mounted pvc")
	}
}

func TestCollectDeploymentPVCs(t *testing.T) {
	dep := &appsv1.Deployment{}
	dep.Spec.Template.Spec.Volumes = []corev1.Volume{{
		Name: "data",
		VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: "test-repo-data",
		}},
	}}
	dep.Spec.Template.Spec.Containers = []corev1.Container{{
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/var/lib/app"}},
	}}
	out := map[string]pvcMountInfo{}
	collectDeploymentPVCs(dep, out)
	info, ok := out["test-repo-data"]
	if !ok {
		t.Fatal("missing pvc claim")
	}
	if info.VolumeName != "data" || info.MountPath != "/var/lib/app" {
		t.Fatalf("unexpected mount %+v", info)
	}
}

func TestParseAccessMode(t *testing.T) {
	mode, err := parseAccessMode("")
	if err != nil || mode != corev1.ReadWriteOnce {
		t.Fatalf("default: %v %v", mode, err)
	}
	if _, err := parseAccessMode("Nope"); err == nil {
		t.Fatal("expected invalid access mode")
	}
}

func TestVolumeNameFromPVC(t *testing.T) {
	if got := volumeNameFromPVC("test-repo"); got != "test-repo" {
		t.Fatalf("got %s", got)
	}
}
