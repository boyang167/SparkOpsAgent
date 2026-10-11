package k8s

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
)

const managedBySelector = "app.kubernetes.io/managed-by=sparkops"

type manifestRef struct {
	kind string
	name string
}

func (d *ManifestDeployer) Remove(ctx context.Context, target port.DeploymentTarget, spec port.RemoveSpec) error {
	if strings.TrimSpace(target.Kubeconfig) == "" {
		return domain.ErrKubeconfigMissing
	}
	namespace := strings.TrimSpace(target.Namespace)
	if namespace == "" {
		return domain.ErrManifestInvalid
	}
	client, _, err := d.clients(target)
	if err != nil {
		return err
	}
	if client == nil {
		return errors.New("kubernetes clients are not configured")
	}
	refs := manifestRefs(spec.Manifests)
	selector := appSelector(spec.AppLabel)
	deployments := namesOf(refs, "Deployment")
	services := namesOf(refs, "Service")
	if selector != "" {
		deployments = appendUniqueStrings(deployments, listDeploymentNames(ctx, client, namespace, selector)...)
		services = appendUniqueStrings(services, listServiceNames(ctx, client, namespace, selector)...)
	}

	var errs []error
	claims := make([]string, 0)
	if spec.DeleteStorage {
		claims = appendUniqueStrings(claims, deploymentClaimNames(ctx, client, namespace, deployments)...)
	}
	policy := metav1.DeletePropagationBackground
	opts := metav1.DeleteOptions{PropagationPolicy: &policy}
	for _, name := range deployments {
		errs = append(errs, ignoreNotFound("Deployment", name,
			client.AppsV1().Deployments(namespace).Delete(ctx, name, opts)))
	}
	for _, name := range services {
		errs = append(errs, ignoreNotFound("Service", name,
			client.CoreV1().Services(namespace).Delete(ctx, name, opts)))
	}
	ingresses := namesOf(refs, "Ingress")
	if selector != "" {
		list, err := client.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			errs = append(errs, fmt.Errorf("list ingresses: %w", err))
		} else {
			for _, item := range list.Items {
				ingresses = appendUniqueStrings(ingresses, item.Name)
			}
		}
	}
	for _, name := range ingresses {
		errs = append(errs, ignoreNotFound("Ingress", name,
			client.NetworkingV1().Ingresses(namespace).Delete(ctx, name, opts)))
	}
	if spec.DeleteConfig {
		errs = append(errs, removeConfig(ctx, client, namespace, selector, refs, opts)...)
	}
	if spec.DeleteStorage {
		claims = appendUniqueStrings(claims, namesOf(refs, "PersistentVolumeClaim")...)
		if selector != "" {
			list, err := client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
			if err != nil {
				errs = append(errs, fmt.Errorf("list persistent volume claims: %w", err))
			} else {
				for _, item := range list.Items {
					claims = appendUniqueStrings(claims, item.Name)
				}
			}
		}
		for _, name := range claims {
			errs = append(errs, ignoreNotFound("PersistentVolumeClaim", name,
				client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, opts)))
		}
	}
	return errors.Join(errs...)
}

func removeConfig(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, selector string,
	refs []manifestRef,
	opts metav1.DeleteOptions,
) []error {
	var errs []error
	configMaps := namesOf(refs, "ConfigMap")
	secrets := namesOf(refs, "Secret")
	if selector != "" {
		if list, err := client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector}); err != nil {
			errs = append(errs, fmt.Errorf("list config maps: %w", err))
		} else {
			for _, item := range list.Items {
				configMaps = appendUniqueStrings(configMaps, item.Name)
			}
		}
		if list, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector}); err != nil {
			errs = append(errs, fmt.Errorf("list secrets: %w", err))
		} else {
			for _, item := range list.Items {
				secrets = appendUniqueStrings(secrets, item.Name)
			}
		}
	}
	for _, name := range configMaps {
		errs = append(errs, ignoreNotFound("ConfigMap", name,
			client.CoreV1().ConfigMaps(namespace).Delete(ctx, name, opts)))
	}
	for _, name := range secrets {
		existing, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("get Secret %q: %w", name, err))
			continue
		}
		if existing.Type == corev1.SecretTypeDockerConfigJson {
			continue
		}
		errs = append(errs, ignoreNotFound("Secret", name,
			client.CoreV1().Secrets(namespace).Delete(ctx, name, opts)))
	}
	return errs
}

func appSelector(appLabel string) string {
	label := strings.TrimSpace(appLabel)
	if label == "" {
		return ""
	}
	return "app=" + label + "," + managedBySelector
}

func manifestRefs(manifests []port.Manifest) []manifestRef {
	refs := make([]manifestRef, 0, len(manifests))
	for _, manifest := range manifests {
		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest.YAML), 4096)
		for {
			var raw map[string]any
			if err := decoder.Decode(&raw); err != nil {
				break
			}
			kind, _ := raw["kind"].(string)
			metadata, _ := raw["metadata"].(map[string]any)
			name, _ := metadata["name"].(string)
			if strings.TrimSpace(kind) != "" && strings.TrimSpace(name) != "" {
				refs = append(refs, manifestRef{kind: kind, name: name})
			}
		}
	}
	return refs
}

func namesOf(refs []manifestRef, kind string) []string {
	names := make([]string, 0)
	for _, ref := range refs {
		if ref.kind == kind {
			names = appendUniqueStrings(names, ref.name)
		}
	}
	return names
}

func listDeploymentNames(ctx context.Context, client kubernetes.Interface, namespace, selector string) []string {
	list, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.Name)
	}
	return names
}

func listServiceNames(ctx context.Context, client kubernetes.Interface, namespace, selector string) []string {
	list, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.Name)
	}
	return names
}

func deploymentClaimNames(ctx context.Context, client kubernetes.Interface, namespace string, deployments []string) []string {
	claims := make([]string, 0)
	for _, name := range deployments {
		deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		for _, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				claims = appendUniqueStrings(claims, volume.PersistentVolumeClaim.ClaimName)
			}
		}
	}
	return claims
}

func ignoreNotFound(kind, name string, err error) error {
	if err == nil || apierrors.IsNotFound(err) {
		return nil
	}
	return fmt.Errorf("delete %s %q: %w", kind, name, err)
}

var _ port.ManifestRemover = (*ManifestDeployer)(nil)
