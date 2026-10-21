package rbac

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type NamespaceRole struct {
	Namespace string   `json:"namespace"`
	Roles     []string `json:"roles"`
}

type Spec struct {
	WorkspaceID    string
	ClusterID      string
	MemberID       string
	ClusterRoles   []string
	NamespaceRoles []NamespaceRole
}

func Apply(ctx context.Context, client kubernetes.Interface, spec Spec) error {
	if client == nil {
		return fmt.Errorf("kubernetes client is required")
	}
	labels := managedLabels(spec.WorkspaceID, spec.ClusterID, spec.MemberID)
	saName := ServiceAccountName(spec.MemberID)
	if err := ensureNamespace(ctx, client, labels); err != nil {
		return err
	}
	if err := ensureServiceAccount(ctx, client, saName, labels); err != nil {
		return err
	}
	desiredCRB := map[string]string{}
	for _, role := range unique(spec.ClusterRoles) {
		desiredCRB[ClusterRoleBindingName(spec.MemberID, role)] = role
	}
	if err := syncClusterRoleBindings(ctx, client, spec, saName, labels, desiredCRB); err != nil {
		return err
	}
	desiredRB := map[string]namespaceRoleRef{}
	for _, item := range spec.NamespaceRoles {
		ns := item.Namespace
		if ns == "" {
			continue
		}
		for _, role := range unique(item.Roles) {
			desiredRB[RoleBindingName(spec.MemberID, ns, role)] = namespaceRoleRef{Namespace: ns, Role: role}
		}
	}
	return syncRoleBindings(ctx, client, spec, saName, labels, desiredRB)
}

func Remove(ctx context.Context, client kubernetes.Interface, workspaceID, clusterID, memberID string) error {
	if client == nil {
		return fmt.Errorf("kubernetes client is required")
	}
	selector := managedSelector(workspaceID, clusterID, memberID)
	if err := deleteMatchingClusterRoleBindings(ctx, client, selector); err != nil {
		return err
	}
	if err := deleteMatchingRoleBindings(ctx, client, selector); err != nil {
		return err
	}
	saName := ServiceAccountName(memberID)
	err := client.CoreV1().ServiceAccounts(ManagedNamespace).Delete(ctx, saName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func ensureNamespace(ctx context.Context, client kubernetes.Interface, labels map[string]string) error {
	_, err := client.CoreV1().Namespaces().Get(ctx, ManagedNamespace, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ManagedNamespace, Labels: labels},
	}, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func ensureServiceAccount(ctx context.Context, client kubernetes.Interface, name string, labels map[string]string) error {
	_, err := client.CoreV1().ServiceAccounts(ManagedNamespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	_, err = client.CoreV1().ServiceAccounts(ManagedNamespace).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ManagedNamespace, Labels: labels},
	}, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func syncClusterRoleBindings(ctx context.Context, client kubernetes.Interface, spec Spec, saName string, labels map[string]string, desired map[string]string) error {
	existing, err := client.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{
		LabelSelector: managedSelector(spec.WorkspaceID, spec.ClusterID, spec.MemberID),
	})
	if err != nil {
		return err
	}
	keep := map[string]struct{}{}
	for _, item := range existing.Items {
		role, ok := desired[item.Name]
		if !ok || item.RoleRef.Name != role {
			if err := client.RbacV1().ClusterRoleBindings().Delete(ctx, item.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			continue
		}
		keep[item.Name] = struct{}{}
	}
	for name, role := range desired {
		if _, ok := keep[name]; ok {
			continue
		}
		_, err := client.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      saName,
				Namespace: ManagedNamespace,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     role,
			},
		}, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

type namespaceRoleRef struct {
	Namespace string
	Role      string
}

func syncRoleBindings(ctx context.Context, client kubernetes.Interface, spec Spec, saName string, labels map[string]string, desired map[string]namespaceRoleRef) error {
	existing, err := listManagedRoleBindings(ctx, client, managedSelector(spec.WorkspaceID, spec.ClusterID, spec.MemberID))
	if err != nil {
		return err
	}
	keep := map[string]struct{}{}
	for _, item := range existing {
		ref, ok := desired[item.Name]
		if !ok || item.Namespace != ref.Namespace || item.RoleRef.Name != ref.Role {
			if err := client.RbacV1().RoleBindings(item.Namespace).Delete(ctx, item.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			continue
		}
		keep[item.Name] = struct{}{}
	}
	for name, ref := range desired {
		if _, ok := keep[name]; ok {
			continue
		}
		_, err := client.RbacV1().RoleBindings(ref.Namespace).Create(ctx, &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ref.Namespace, Labels: labels},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      saName,
				Namespace: ManagedNamespace,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     ref.Role,
			},
		}, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func listManagedRoleBindings(ctx context.Context, client kubernetes.Interface, selector string) ([]rbacv1.RoleBinding, error) {
	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var out []rbacv1.RoleBinding
	seen := map[string]struct{}{}
	for _, ns := range namespaces.Items {
		seen[ns.Name] = struct{}{}
		items, err := client.RbacV1().RoleBindings(ns.Name).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return nil, err
		}
		out = append(out, items.Items...)
	}
	if _, ok := seen["default"]; !ok {
		items, err := client.RbacV1().RoleBindings("default").List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return nil, err
		}
		out = append(out, items.Items...)
	}
	return out, nil
}

func deleteMatchingClusterRoleBindings(ctx context.Context, client kubernetes.Interface, selector string) error {
	items, err := client.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return err
	}
	for _, item := range items.Items {
		if err := client.RbacV1().ClusterRoleBindings().Delete(ctx, item.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func deleteMatchingRoleBindings(ctx context.Context, client kubernetes.Interface, selector string) error {
	items, err := listManagedRoleBindings(ctx, client, selector)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := client.RbacV1().RoleBindings(item.Namespace).Delete(ctx, item.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func unique(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
