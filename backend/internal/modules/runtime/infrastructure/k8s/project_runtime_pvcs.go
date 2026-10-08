package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectPVCVM struct {
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	Status       string   `json:"status,omitempty"`
	Storage      string   `json:"storage,omitempty"`
	AccessModes  []string `json:"access_modes,omitempty"`
	StorageClass string   `json:"storage_class,omitempty"`
	VolumeName   string   `json:"volume_name,omitempty"`
	MountName    string   `json:"mount_name,omitempty"`
	MountPath    string   `json:"mount_path,omitempty"`
	Mounted      bool     `json:"mounted"`
	CreatedAt    string   `json:"created_at,omitempty"`
}

type StorageClassVM struct {
	Name          string `json:"name"`
	Provisioner   string `json:"provisioner,omitempty"`
	ReclaimPolicy string `json:"reclaim_policy,omitempty"`
}

type StorageOverviewVM struct {
	Items    []ProjectPVCVM   `json:"items"`
	Classes  []StorageClassVM `json:"classes"`
	Warnings []string         `json:"warnings"`
}

type CreatePVCInput struct {
	Name         string `json:"name"`
	Storage      string `json:"storage"`
	AccessMode   string `json:"access_mode"`
	StorageClass string `json:"storage_class"`
	MountPath    string `json:"mount_path"`
	VolumeName   string `json:"volume_name"`
}

type BindPVCInput struct {
	MountPath  string `json:"mount_path"`
	VolumeName string `json:"volume_name"`
}

type pvcMountInfo struct {
	VolumeName string
	MountPath  string
}

func (c *ProjectRuntimeConfigurator) ListPVCs(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) ([]ProjectPVCVM, error) {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return listPVCsWithSession(ctx, session)
}

func listPVCsWithSession(
	ctx context.Context,
	session *ProjectClusterSession,
) ([]ProjectPVCVM, error) {
	list, err := session.Client.CoreV1().PersistentVolumeClaims(session.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	mounts := pvcMountsFromDeployment(ctx, session)
	out := make([]ProjectPVCVM, 0)
	for i := range list.Items {
		pvc := list.Items[i]
		if !pvcBelongsToProject(&pvc, session.ProjectName, mounts) {
			continue
		}
		out = append(out, toPVCVM(&pvc, mounts[pvc.Name]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *ProjectRuntimeConfigurator) ListStorageClasses(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) ([]StorageClassVM, error) {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return listStorageClassesWithSession(ctx, session)
}

func listStorageClassesWithSession(
	ctx context.Context,
	session *ProjectClusterSession,
) ([]StorageClassVM, error) {
	list, err := session.Client.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]StorageClassVM, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, toStorageClassVM(&list.Items[i]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *ProjectRuntimeConfigurator) GetStorageOverview(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) (StorageOverviewVM, error) {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return StorageOverviewVM{}, err
	}
	return getStorageOverviewWithSession(ctx, session)
}

func getStorageOverviewWithSession(
	ctx context.Context,
	session *ProjectClusterSession,
) (StorageOverviewVM, error) {
	type pvcResult struct {
		items []ProjectPVCVM
		err   error
	}
	type classResult struct {
		items []StorageClassVM
		err   error
	}

	pvcCh := make(chan pvcResult, 1)
	classCh := make(chan classResult, 1)
	go func() {
		items, err := listPVCsWithSession(ctx, session)
		pvcCh <- pvcResult{items: items, err: err}
	}()
	go func() {
		items, err := listStorageClassesWithSession(ctx, session)
		classCh <- classResult{items: items, err: err}
	}()

	pvcs := <-pvcCh
	classes := <-classCh
	if pvcs.err != nil {
		return StorageOverviewVM{}, pvcs.err
	}
	overview := StorageOverviewVM{
		Items:    pvcs.items,
		Classes:  classes.items,
		Warnings: []string{},
	}
	if classes.err != nil {
		overview.Classes = []StorageClassVM{}
		overview.Warnings = append(
			overview.Warnings,
			"StorageClass 加载失败："+classes.err.Error(),
		)
	}
	return overview, nil
}

func (c *ProjectRuntimeConfigurator) CreatePVC(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input CreatePVCInput,
) error {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	name := strings.ToLower(strings.TrimSpace(input.Name))
	if name == "" {
		name = session.ProjectName
	}
	if !k8sNamePattern.MatchString(name) || len(name) > 253 {
		return fmt.Errorf("invalid pvc name")
	}
	storage := strings.TrimSpace(input.Storage)
	if storage == "" {
		storage = "1Gi"
	}
	qty, err := resource.ParseQuantity(storage)
	if err != nil {
		return fmt.Errorf("invalid storage quantity")
	}
	mode, err := parseAccessMode(input.AccessMode)
	if err != nil {
		return err
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: session.Namespace,
			Labels: map[string]string{
				sparkopsAppLabel: session.ProjectName,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{mode},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: qty,
				},
			},
		},
	}
	if class := strings.TrimSpace(input.StorageClass); class != "" {
		pvc.Spec.StorageClassName = &class
	}
	_, err = session.Client.CoreV1().PersistentVolumeClaims(session.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("pvc %s already exists", name)
	}
	if err != nil {
		return err
	}
	mountPath := strings.TrimSpace(input.MountPath)
	if mountPath == "" {
		return nil
	}
	return c.bindPVCSession(ctx, session, name, BindPVCInput{
		MountPath:  mountPath,
		VolumeName: input.VolumeName,
	})
}

func (c *ProjectRuntimeConfigurator) BindPVC(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	name string,
	input BindPVCInput,
) error {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	return c.bindPVCSession(ctx, session, name, input)
}

func (c *ProjectRuntimeConfigurator) UnbindPVC(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	name string,
) error {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	return c.unbindPVCSession(ctx, session, name)
}

func (c *ProjectRuntimeConfigurator) DeletePVC(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	name string,
) error {
	session, err := c.access.Open(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if !k8sNamePattern.MatchString(name) {
		return fmt.Errorf("invalid pvc name")
	}
	pvc, err := session.Client.CoreV1().PersistentVolumeClaims(session.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	mounts := pvcMountsFromDeployment(ctx, session)
	if !pvcBelongsToProject(pvc, session.ProjectName, mounts) {
		return fmt.Errorf("pvc does not belong to this project")
	}
	if _, mounted := mounts[name]; mounted {
		if err := c.unbindPVCSession(ctx, session, name); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return session.Client.CoreV1().PersistentVolumeClaims(session.Namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (c *ProjectRuntimeConfigurator) bindPVCSession(
	ctx context.Context,
	session *ProjectClusterSession,
	name string,
	input BindPVCInput,
) error {
	name = strings.TrimSpace(name)
	mountPath := strings.TrimSpace(input.MountPath)
	if !k8sNamePattern.MatchString(name) {
		return fmt.Errorf("invalid pvc name")
	}
	if !strings.HasPrefix(mountPath, "/") || mountPath == "/" {
		return fmt.Errorf("mount path must be an absolute path")
	}
	pvc, err := session.Client.CoreV1().PersistentVolumeClaims(session.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	mounts := pvcMountsFromDeployment(ctx, session)
	if !pvcBelongsToProject(pvc, session.ProjectName, mounts) {
		return fmt.Errorf("pvc does not belong to this project")
	}
	dep, err := getDeployment(ctx, session.Client, session.Namespace, session.ProjectName)
	if err != nil {
		return err
	}
	idx := primaryContainerIndex(dep, session.ProjectName)
	if idx < 0 {
		return fmt.Errorf("deployment container not found")
	}
	volumeName := strings.TrimSpace(input.VolumeName)
	if existing, ok := mounts[name]; ok && existing.VolumeName != "" {
		volumeName = existing.VolumeName
	}
	if volumeName == "" {
		volumeName = volumeNameFromPVC(name)
	}
	if !k8sNamePattern.MatchString(volumeName) || len(volumeName) > 63 {
		return fmt.Errorf("invalid volume name")
	}
	container := &dep.Spec.Template.Spec.Containers[idx]
	if hasVolumeMount(container, mountPath) && (mounts[name].MountPath != mountPath) {
		return fmt.Errorf("mount path already in use")
	}
	for i := range dep.Spec.Template.Spec.Volumes {
		vol := dep.Spec.Template.Spec.Volumes[i]
		if vol.Name != volumeName {
			continue
		}
		if vol.PersistentVolumeClaim == nil || vol.PersistentVolumeClaim.ClaimName != name {
			return fmt.Errorf("volume name already used by another volume")
		}
	}
	if !hasVolume(dep, volumeName) {
		dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: name,
				},
			},
		})
	}
	if !hasVolumeMountName(container, volumeName) {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
		})
	} else {
		for i := range container.VolumeMounts {
			if container.VolumeMounts[i].Name == volumeName {
				container.VolumeMounts[i].MountPath = mountPath
			}
		}
	}
	return rolloutRestartDeployment(ctx, session.Client, dep)
}

func (c *ProjectRuntimeConfigurator) unbindPVCSession(
	ctx context.Context,
	session *ProjectClusterSession,
	name string,
) error {
	name = strings.TrimSpace(name)
	dep, err := getDeployment(ctx, session.Client, session.Namespace, session.ProjectName)
	if err != nil {
		return err
	}
	remove := map[string]struct{}{}
	kept := dep.Spec.Template.Spec.Volumes[:0]
	for _, vol := range dep.Spec.Template.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == name {
			remove[vol.Name] = struct{}{}
			continue
		}
		kept = append(kept, vol)
	}
	if len(remove) == 0 {
		return nil
	}
	dep.Spec.Template.Spec.Volumes = kept
	for i := range dep.Spec.Template.Spec.Containers {
		item := &dep.Spec.Template.Spec.Containers[i]
		mounts := item.VolumeMounts[:0]
		for _, mount := range item.VolumeMounts {
			if _, drop := remove[mount.Name]; drop {
				continue
			}
			mounts = append(mounts, mount)
		}
		item.VolumeMounts = mounts
	}
	return rolloutRestartDeployment(ctx, session.Client, dep)
}

func toPVCVM(pvc *corev1.PersistentVolumeClaim, mount pvcMountInfo) ProjectPVCVM {
	created := ""
	if !pvc.CreationTimestamp.IsZero() {
		created = pvc.CreationTimestamp.UTC().Format("2006-01-02 15:04:05")
	}
	modes := make([]string, 0, len(pvc.Spec.AccessModes))
	for _, mode := range pvc.Spec.AccessModes {
		modes = append(modes, string(mode))
	}
	storage := ""
	if qty, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
		storage = qty.String()
	} else if qty, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		storage = qty.String()
	}
	class := ""
	if pvc.Spec.StorageClassName != nil {
		class = *pvc.Spec.StorageClassName
	}
	return ProjectPVCVM{
		Name:         pvc.Name,
		Namespace:    pvc.Namespace,
		Status:       string(pvc.Status.Phase),
		Storage:      storage,
		AccessModes:  modes,
		StorageClass: class,
		VolumeName:   pvc.Spec.VolumeName,
		MountName:    mount.VolumeName,
		MountPath:    mount.MountPath,
		Mounted:      mount.VolumeName != "" || mount.MountPath != "",
		CreatedAt:    created,
	}
}

func toStorageClassVM(sc *storagev1.StorageClass) StorageClassVM {
	policy := ""
	if sc.ReclaimPolicy != nil {
		policy = string(*sc.ReclaimPolicy)
	}
	return StorageClassVM{
		Name:          sc.Name,
		Provisioner:   sc.Provisioner,
		ReclaimPolicy: policy,
	}
}

func pvcBelongsToProject(pvc *corev1.PersistentVolumeClaim, projectName string, mounts map[string]pvcMountInfo) bool {
	if pvc == nil || projectName == "" {
		return false
	}
	if pvc.Name == projectName {
		return true
	}
	if pvc.Labels[sparkopsAppLabel] == projectName {
		return true
	}
	if _, ok := mounts[pvc.Name]; ok {
		return true
	}
	return false
}

func pvcMountsFromDeployment(ctx context.Context, session *ProjectClusterSession) map[string]pvcMountInfo {
	out := map[string]pvcMountInfo{}
	if session == nil || session.Client == nil {
		return out
	}
	dep, err := getDeployment(ctx, session.Client, session.Namespace, session.ProjectName)
	if err != nil {
		return out
	}
	collectDeploymentPVCs(dep, out)
	return out
}

func collectDeploymentPVCs(dep *appsv1.Deployment, out map[string]pvcMountInfo) {
	if dep == nil || out == nil {
		return
	}
	volToClaim := map[string]string{}
	for _, vol := range dep.Spec.Template.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName != "" {
			volToClaim[vol.Name] = vol.PersistentVolumeClaim.ClaimName
			info := out[vol.PersistentVolumeClaim.ClaimName]
			info.VolumeName = vol.Name
			out[vol.PersistentVolumeClaim.ClaimName] = info
		}
	}
	for _, c := range append(append([]corev1.Container{}, dep.Spec.Template.Spec.Containers...), dep.Spec.Template.Spec.InitContainers...) {
		for _, mount := range c.VolumeMounts {
			claim, ok := volToClaim[mount.Name]
			if !ok {
				continue
			}
			info := out[claim]
			info.VolumeName = mount.Name
			if info.MountPath == "" {
				info.MountPath = mount.MountPath
			}
			out[claim] = info
		}
	}
}

func parseAccessMode(raw string) (corev1.PersistentVolumeAccessMode, error) {
	switch strings.TrimSpace(raw) {
	case "", string(corev1.ReadWriteOnce):
		return corev1.ReadWriteOnce, nil
	case string(corev1.ReadOnlyMany):
		return corev1.ReadOnlyMany, nil
	case string(corev1.ReadWriteMany):
		return corev1.ReadWriteMany, nil
	case string(corev1.ReadWriteOncePod):
		return corev1.ReadWriteOncePod, nil
	default:
		return "", fmt.Errorf("invalid access mode")
	}
}

func volumeNameFromPVC(name string) string {
	if len(name) <= 63 && k8sNamePattern.MatchString(name) {
		return name
	}
	trimmed := name
	if len(trimmed) > 63 {
		trimmed = strings.Trim(trimmed[:63], "-")
	}
	if k8sNamePattern.MatchString(trimmed) {
		return trimmed
	}
	return "data"
}

func hasVolumeMountName(container *corev1.Container, name string) bool {
	for _, mount := range container.VolumeMounts {
		if mount.Name == name {
			return true
		}
	}
	return false
}
