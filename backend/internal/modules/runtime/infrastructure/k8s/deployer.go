package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	platformk8s "github.com/sparkops/sparkops-agent/backend/internal/platform/k8s"
)

const (
	defaultRolloutTimeout = 10 * time.Minute
	defaultPollInterval   = time.Second
	fieldManager          = "sparkops-deploy"
)

var (
	deploymentsResource = schema.GroupVersionResource{
		Group: "apps", Version: "v1", Resource: "deployments",
	}
	servicesResource = schema.GroupVersionResource{
		Group: "", Version: "v1", Resource: "services",
	}
)

type clientProvider func(port.DeploymentTarget) (kubernetes.Interface, dynamic.Interface, error)

type ManifestDeployer struct {
	clients      clientProvider
	pollInterval time.Duration
}

func NewManifestDeployer(
	dynamicClient dynamic.Interface,
	typedClient kubernetes.Interface,
	pollInterval time.Duration,
) *ManifestDeployer {
	return newManifestDeployer(func(port.DeploymentTarget) (kubernetes.Interface, dynamic.Interface, error) {
		return typedClient, dynamicClient, nil
	}, pollInterval)
}

func NewKubeconfigManifestDeployer(pollInterval time.Duration) *ManifestDeployer {
	return newManifestDeployer(func(target port.DeploymentTarget) (kubernetes.Interface, dynamic.Interface, error) {
		return buildClients([]byte(target.Kubeconfig), target.Server)
	}, pollInterval)
}

func newManifestDeployer(clients clientProvider, pollInterval time.Duration) *ManifestDeployer {
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	return &ManifestDeployer{clients: clients, pollInterval: pollInterval}
}

func buildClients(
	kubeconfig []byte,
	expectedServer string,
) (kubernetes.Interface, dynamic.Interface, error) {
	if len(bytes.TrimSpace(kubeconfig)) == 0 {
		return nil, nil, domain.ErrKubeconfigMissing
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, nil, domain.ErrKubeconfigInvalid
	}
	if !sameServerEndpoint(cfg.Host, expectedServer) {
		return nil, nil, domain.ErrClusterServerMismatch
	}
	typedClient, dynamicClient, err := platformk8s.NewClientsForConfig(cfg)
	if err != nil {
		return nil, nil, domain.ErrKubeconfigInvalid
	}
	return typedClient, dynamicClient, nil
}

func sameServerEndpoint(actual, expected string) bool {
	actualScheme, actualHost, actualOK := normalizedServerEndpoint(actual)
	expectedScheme, expectedHost, expectedOK := normalizedServerEndpoint(expected)
	if !actualOK || !expectedOK {
		return false
	}
	return actualScheme == expectedScheme && actualHost == expectedHost
}

func normalizedServerEndpoint(raw string) (string, string, bool) {
	serverURL, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || serverURL.Host == "" {
		return "", "", false
	}
	scheme := strings.ToLower(serverURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", "", false
	}
	return scheme, strings.ToLower(serverURL.Host), true
}

func (d *ManifestDeployer) Deploy(
	ctx context.Context,
	target port.DeploymentTarget,
	manifests []port.Manifest,
) error {
	if len(strings.TrimSpace(target.Kubeconfig)) == 0 {
		return domain.ErrKubeconfigMissing
	}
	objects, err := parseManifests(target, manifests)
	if err != nil {
		return err
	}
	typedClient, dynamicClient, err := d.clients(target)
	if err != nil {
		return err
	}
	if typedClient == nil || dynamicClient == nil {
		return errors.New("kubernetes clients are not configured")
	}
	if err := applyObjects(ctx, dynamicClient, objects); err != nil {
		return err
	}

	timeout := target.RolloutTimeout
	if timeout <= 0 {
		timeout = defaultRolloutTimeout
	}
	rolloutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := d.waitForDeployment(rolloutCtx, typedClient, target); err != nil {
		return err
	}
	service, err := typedClient.CoreV1().Services(target.Namespace).Get(
		rolloutCtx, target.ServiceName, metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("validate service %q: %w", target.ServiceName, err)
	}
	if service.DeletionTimestamp != nil {
		return domain.ErrServiceDeleting
	}
	return nil
}

func parseManifests(target port.DeploymentTarget, manifests []port.Manifest) ([]*unstructured.Unstructured, error) {
	var (
		objects             []*unstructured.Unstructured
		foundDeploymentName bool
		foundServiceName    bool
	)
	for _, manifest := range manifests {
		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest.YAML), 4096)
		for {
			var raw map[string]any
			err := decoder.Decode(&raw)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, domain.ErrManifestInvalid
			}
			if len(raw) == 0 {
				continue
			}
			object := &unstructured.Unstructured{Object: raw}
			resource, supported := resourceFor(object.GroupVersionKind())
			if !supported {
				return nil, domain.ErrManifestUnsupported
			}
			if object.GetName() == "" || strings.TrimSpace(target.Namespace) == "" {
				return nil, domain.ErrManifestInvalid
			}
			if namespace := object.GetNamespace(); namespace != "" && namespace != target.Namespace {
				return nil, domain.ErrManifestNamespaceMismatch
			}
			object.SetNamespace(target.Namespace)
			switch resource {
			case deploymentsResource:
				if object.GetName() != target.DeploymentName {
					return nil, domain.ErrManifestInvalid
				}
				if err := validateDeploymentImageTag(object, target.ImageTag); err != nil {
					return nil, err
				}
				foundDeploymentName = true
			case servicesResource:
				if object.GetName() != target.ServiceName {
					return nil, domain.ErrManifestInvalid
				}
				foundServiceName = true
			}
			objects = append(objects, object)
		}
	}
	if !foundDeploymentName || !foundServiceName {
		return nil, domain.ErrManifestInvalid
	}
	return objects, nil
}

func validateDeploymentImageTag(object *unstructured.Unstructured, targetTag string) error {
	targetTag = strings.TrimSpace(targetTag)
	if targetTag == "" {
		return domain.ErrManifestImageTagMissing
	}
	containers, found, err := unstructured.NestedSlice(
		object.Object, "spec", "template", "spec", "containers",
	)
	if err != nil || !found || len(containers) == 0 {
		return domain.ErrManifestInvalid
	}
	for _, container := range containers {
		containerMap, ok := container.(map[string]any)
		if !ok {
			return domain.ErrManifestInvalid
		}
		image, ok := containerMap["image"].(string)
		if !ok {
			return domain.ErrManifestInvalid
		}
		if imageTag(image) == targetTag {
			return nil
		}
	}
	return domain.ErrManifestImageTagMismatch
}

func imageTag(image string) string {
	name := strings.TrimSpace(image)
	if digestIndex := strings.IndexByte(name, '@'); digestIndex >= 0 {
		name = name[:digestIndex]
	}
	lastColon := strings.LastIndexByte(name, ':')
	lastSlash := strings.LastIndexByte(name, '/')
	if lastColon <= lastSlash || lastColon == len(name)-1 {
		return ""
	}
	return name[lastColon+1:]
}

func resourceFor(gvk schema.GroupVersionKind) (schema.GroupVersionResource, bool) {
	switch {
	case gvk.Group == appsv1.GroupName && gvk.Version == "v1" && gvk.Kind == "Deployment":
		return deploymentsResource, true
	case gvk.Group == "" && gvk.Version == "v1" && gvk.Kind == "Service":
		return servicesResource, true
	default:
		return schema.GroupVersionResource{}, false
	}
}

type applyOperation struct {
	resource  schema.GroupVersionResource
	namespace string
	name      string
	kind      string
	payload   []byte
}

func prepareApplyOperations(objects []*unstructured.Unstructured) ([]applyOperation, error) {
	operations := make([]applyOperation, 0, len(objects))
	for _, object := range objects {
		resource, supported := resourceFor(object.GroupVersionKind())
		if !supported {
			return nil, domain.ErrManifestUnsupported
		}
		data, err := json.Marshal(object.Object)
		if err != nil {
			return nil, domain.ErrManifestInvalid
		}
		operations = append(operations, applyOperation{
			resource: resource, namespace: object.GetNamespace(),
			name: object.GetName(), kind: object.GetKind(), payload: data,
		})
	}
	return operations, nil
}

func applyObjects(
	ctx context.Context,
	client dynamic.Interface,
	objects []*unstructured.Unstructured,
) error {
	operations, err := prepareApplyOperations(objects)
	if err != nil {
		return err
	}
	for _, operation := range operations {
		_, err := client.Resource(operation.resource).Namespace(operation.namespace).Patch(
			ctx,
			operation.name,
			types.ApplyPatchType,
			operation.payload,
			metav1.PatchOptions{FieldManager: fieldManager, Force: ptr.To(true)},
		)
		if err != nil {
			return fmt.Errorf("apply %s %q: %w", operation.kind, operation.name, err)
		}
	}
	return nil
}

func (d *ManifestDeployer) waitForDeployment(
	ctx context.Context,
	client kubernetes.Interface,
	target port.DeploymentTarget,
) error {
	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()
	for {
		deployment, err := client.AppsV1().Deployments(target.Namespace).Get(
			ctx, target.DeploymentName, metav1.GetOptions{},
		)
		if err == nil && deploymentProgressDeadlineExceeded(deployment) {
			return domain.ErrDeploymentProgressDeadlineExceeded
		}
		if err == nil && deploymentReady(deployment) {
			return nil
		}
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("read deployment %q: %w", target.DeploymentName, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func deploymentReady(deployment *appsv1.Deployment) bool {
	if deployment.DeletionTimestamp != nil {
		return false
	}
	replicas := int32(1)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}
	return deployment.Status.ObservedGeneration >= deployment.Generation &&
		deployment.Status.AvailableReplicas >= replicas
}

func deploymentProgressDeadlineExceeded(deployment *appsv1.Deployment) bool {
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing &&
			condition.Status == "False" &&
			condition.Reason == "ProgressDeadlineExceeded" {
			return true
		}
	}
	return false
}

var _ port.ManifestDeployer = (*ManifestDeployer)(nil)
