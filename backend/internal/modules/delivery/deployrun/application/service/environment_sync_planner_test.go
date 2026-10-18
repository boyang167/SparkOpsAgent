package service_test

import (
	"fmt"
	"strings"
	"testing"

	deploydomain "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPlanEnvironmentSyncImageOnlyPreservesTargetSpec(t *testing.T) {
	source := deploymentResource("app", "source-ns", "registry/source/app:v2", 5)
	target := deploymentResource("app", "target-ns", "registry/target/app:v1", 2)

	plan, err := service.PlanEnvironmentSync(
		[]model.EnvironmentSyncResource{source},
		[]model.EnvironmentSyncResource{target},
		model.EnvironmentSyncProjectOption{SyncImage: true, SyncYAML: false},
		"target-ns",
	)

	require.NoError(t, err)
	require.Len(t, plan.Manifests, 1)
	require.Contains(t, plan.Manifests[0].YAML, "registry/source/app:v2")
	require.Contains(t, plan.Manifests[0].YAML, "replicas: 2")
	require.Equal(t, "registry/source/app", plan.ImageRepository)
	require.Equal(t, "v2", plan.ImageTag)
	require.Equal(t, "update", plan.Diffs[0].Action)
}

func TestPlanEnvironmentSyncYAMLOnlyPreservesTargetImage(t *testing.T) {
	source := deploymentResource("app", "source-ns", "registry/source/app:v2", 5)
	target := deploymentResource("app", "target-ns", "registry/target/app:v1", 2)

	plan, err := service.PlanEnvironmentSync(
		[]model.EnvironmentSyncResource{source},
		[]model.EnvironmentSyncResource{target},
		model.EnvironmentSyncProjectOption{SyncImage: false, SyncYAML: true},
		"target-ns",
	)

	require.NoError(t, err)
	require.Len(t, plan.Manifests, 1)
	require.Contains(t, plan.Manifests[0].YAML, "registry/target/app:v1")
	require.Contains(t, plan.Manifests[0].YAML, "replicas: 5")
	require.Equal(t, "registry/target/app", plan.ImageRepository)
	require.Equal(t, "v1", plan.ImageTag)
}

func TestPlanEnvironmentSyncBothUsesSourceImageAndYAML(t *testing.T) {
	source := deploymentResource("app", "source-ns", "registry/source/app:v2", 5)
	target := deploymentResource("app", "target-ns", "registry/target/app:v1", 2)

	plan, err := service.PlanEnvironmentSync(
		[]model.EnvironmentSyncResource{source},
		[]model.EnvironmentSyncResource{target},
		model.EnvironmentSyncProjectOption{SyncImage: true, SyncYAML: true},
		"target-ns",
	)

	require.NoError(t, err)
	require.Len(t, plan.Manifests, 1)
	require.Contains(t, plan.Manifests[0].YAML, "registry/source/app:v2")
	require.Contains(t, plan.Manifests[0].YAML, "replicas: 5")
}

func TestPlanEnvironmentSyncFirstSyncForcesBoth(t *testing.T) {
	source := deploymentResource("app", "source-ns", "registry/source/app:v2", 5)
	target := deploymentResource("app", "target-ns", "registry/target/app:v1", 2)

	plan, err := service.PlanEnvironmentSync(
		[]model.EnvironmentSyncResource{source},
		[]model.EnvironmentSyncResource{target},
		model.EnvironmentSyncProjectOption{SyncImage: false, SyncYAML: false, FirstSync: true},
		"target-ns",
	)

	require.NoError(t, err)
	require.Contains(t, plan.Manifests[0].YAML, "registry/source/app:v2")
	require.Contains(t, plan.Manifests[0].YAML, "replicas: 5")
}

func TestPlanEnvironmentSyncNewWorkloadUsesSourceImage(t *testing.T) {
	source := deploymentResource("app", "source-ns", "registry/source/app:v2", 3)

	plan, err := service.PlanEnvironmentSync(
		[]model.EnvironmentSyncResource{source},
		nil,
		model.EnvironmentSyncProjectOption{SyncImage: false, SyncYAML: true},
		"target-ns",
	)

	require.NoError(t, err)
	require.Len(t, plan.Manifests, 1)
	require.Contains(t, plan.Manifests[0].YAML, "registry/source/app:v2")
	require.Contains(t, plan.Manifests[0].YAML, "replicas: 3")
	require.Equal(t, "create", plan.Diffs[0].Action)
}

func TestPlanEnvironmentSyncNeverCopiesSecretDataOrDeletesTargetOnlyResources(t *testing.T) {
	source := secretResource("app-secret", "source-ns", map[string]string{"token": "secret-value"})
	targetOnly := model.EnvironmentSyncResource{
		Kind: model.SyncManifestKindService,
		Name: "target-only",
		YAML: "apiVersion: v1\nkind: Service\nmetadata:\n  name: target-only\n  namespace: target-ns\n",
	}

	plan, err := service.PlanEnvironmentSync(
		[]model.EnvironmentSyncResource{source},
		[]model.EnvironmentSyncResource{targetOnly},
		model.EnvironmentSyncProjectOption{
			SyncImage: false, SyncYAML: true,
			SyncResourceKinds: []string{model.SyncManifestKindConfigMap},
		},
		"target-ns",
	)

	require.NoError(t, err)
	require.Empty(t, plan.Manifests)
	require.Equal(t, "excluded", plan.Diffs[0].Action)
	for _, diff := range plan.Diffs {
		require.NotEqual(t, "delete", diff.Action)
	}
}

func TestPlanEnvironmentSyncIncludesManagedPullSecretWhenDeploymentSelected(t *testing.T) {
	source := model.EnvironmentSyncResource{
		Kind: model.SyncManifestKindSecret, Name: "pull-secret", ManagedPullSecret: true,
	}

	plan, err := service.PlanEnvironmentSync(
		[]model.EnvironmentSyncResource{source},
		nil,
		model.EnvironmentSyncProjectOption{
			SyncImage: false, SyncYAML: true,
			SyncResourceKinds: []string{model.SyncManifestKindDeployment},
		},
		"target-ns",
	)

	require.NoError(t, err)
	require.Len(t, plan.Manifests, 1)
	require.True(t, plan.Manifests[0].ManagedPullSecret)
}

func TestPlanEnvironmentSyncExcludesUnselectedResourceKinds(t *testing.T) {
	source := []model.EnvironmentSyncResource{
		deploymentResource("app", "source-ns", "registry/source/app:v2", 2),
		{
			Kind: model.SyncManifestKindService, Name: "app",
			YAML: "apiVersion: v1\nkind: Service\nmetadata:\n  name: app\n  namespace: source-ns\n",
		},
	}

	plan, err := service.PlanEnvironmentSync(
		source,
		nil,
		model.EnvironmentSyncProjectOption{
			SyncImage: false, SyncYAML: true,
			SyncResourceKinds: []string{model.SyncManifestKindService},
		},
		"target-ns",
	)

	require.NoError(t, err)
	require.Len(t, plan.Manifests, 1)
	require.Equal(t, model.SyncManifestKindService, plan.Manifests[0].Kind)
	require.Equal(t, "excluded", plan.Diffs[0].Action)
}

func TestPlanEnvironmentSyncSanitizesClusterMetadata(t *testing.T) {
	source := deploymentResource("app", "source-ns", "registry/source/app:v2", 1)

	plan, err := service.PlanEnvironmentSync(
		[]model.EnvironmentSyncResource{source},
		nil,
		model.EnvironmentSyncProjectOption{SyncImage: true, SyncYAML: true},
		"target-ns",
	)

	require.NoError(t, err)
	yamlDoc := plan.Manifests[0].YAML
	require.Contains(t, yamlDoc, "namespace: target-ns")
	for _, forbidden := range []string{"uid:", "resourceVersion:", "generation:", "managedFields:", "creationTimestamp:", "status:"} {
		require.NotContains(t, yamlDoc, forbidden, "forbidden metadata %s", forbidden)
	}
}

func TestPlanEnvironmentSyncRejectsAmbiguousContainerMatch(t *testing.T) {
	source := deploymentWithDuplicateContainerNames("app", "source-ns", "registry/source/app:v2")
	target := deploymentResource("app", "target-ns", "registry/target/app:v1", 1)

	_, err := service.PlanEnvironmentSync(
		[]model.EnvironmentSyncResource{source},
		[]model.EnvironmentSyncResource{target},
		model.EnvironmentSyncProjectOption{SyncImage: true, SyncYAML: false},
		"target-ns",
	)

	require.ErrorIs(t, err, deploydomain.ErrEnvironmentSyncAmbiguousContainer)
}

func TestPlanEnvironmentSyncManagedPullSecretHasNoYAML(t *testing.T) {
	source := model.EnvironmentSyncResource{
		Kind: model.SyncManifestKindSecret, Name: "registry-pull", ManagedPullSecret: true,
	}

	plan, err := service.PlanEnvironmentSync(
		[]model.EnvironmentSyncResource{source},
		nil,
		model.EnvironmentSyncProjectOption{SyncImage: false, SyncYAML: true},
		"target-ns",
	)

	require.NoError(t, err)
	require.Len(t, plan.Manifests, 1)
	require.True(t, plan.Manifests[0].ManagedPullSecret)
	require.Empty(t, plan.Manifests[0].YAML)
}

func TestPlanEnvironmentSyncApplyOrder(t *testing.T) {
	source := []model.EnvironmentSyncResource{
		deploymentResource("app", "source-ns", "registry/source/app:v2", 1),
		configMapResource("app-cfg", "source-ns", "key: value"),
	}
	target := []model.EnvironmentSyncResource{
		deploymentResource("app", "target-ns", "registry/target/app:v1", 1),
		configMapResource("app-cfg", "target-ns", "key: old"),
	}

	plan, err := service.PlanEnvironmentSync(
		source, target,
		model.EnvironmentSyncProjectOption{SyncImage: false, SyncYAML: true},
		"target-ns",
	)

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(plan.Manifests), 2)
	require.Equal(t, model.SyncManifestKindConfigMap, plan.Manifests[0].Kind)
}

func deploymentResource(name, ns, image string, replicas int) model.EnvironmentSyncResource {
	yamlDoc := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  uid: cluster-uid
  resourceVersion: "123"
  generation: 5
  creationTimestamp: "2026-01-01T00:00:00Z"
  managedFields:
  - manager: kube-controller-manager
spec:
  replicas: %d
  template:
    spec:
      containers:
      - name: app
        image: %s
`, name, ns, replicas, image)
	return model.EnvironmentSyncResource{
		Kind: model.SyncManifestKindDeployment, Name: name, YAML: yamlDoc,
	}
}

func deploymentWithDuplicateContainerNames(name, ns, image string) model.EnvironmentSyncResource {
	yamlDoc := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: app
        image: %s
      - name: app
        image: %s
`, name, ns, image, image)
	return model.EnvironmentSyncResource{
		Kind: model.SyncManifestKindDeployment, Name: name, YAML: yamlDoc,
	}
}

func secretResource(name, ns string, data map[string]string) model.EnvironmentSyncResource {
	doc := map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      name,
			"namespace": ns,
		},
		"type": "Opaque",
		"data": data,
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return model.EnvironmentSyncResource{
		Kind: model.SyncManifestKindSecret, Name: name, YAML: string(out),
	}
}

func configMapResource(name, ns, data string) model.EnvironmentSyncResource {
	yamlDoc := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  %s
`, name, ns, strings.TrimSpace(data))
	return model.EnvironmentSyncResource{
		Kind: model.SyncManifestKindConfigMap, Name: name, YAML: yamlDoc,
	}
}
