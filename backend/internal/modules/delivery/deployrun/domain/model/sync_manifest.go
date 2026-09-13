package model

import (
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"gopkg.in/yaml.v3"
)

const (
	SyncManifestKindDeployment = "Deployment"
	SyncManifestKindService    = "Service"
	SyncManifestKindConfigMap  = "ConfigMap"
	SyncManifestKindSecret     = "Secret"
	SyncManifestKindIngress    = "Ingress"
)

var allowedSyncManifestKinds = map[string]struct{}{
	SyncManifestKindDeployment: {},
	SyncManifestKindService:    {},
	SyncManifestKindConfigMap:  {},
	SyncManifestKindSecret:     {},
	SyncManifestKindIngress:    {},
}

// SyncManifest is one selected Kubernetes document applied during sync_only.
type SyncManifest struct {
	Kind              string `json:"kind"`
	Name              string `json:"name"`
	YAML              string `json:"yaml,omitempty"`
	ManagedPullSecret bool   `json:"managed_pull_secret,omitempty"`
}

func ValidateSyncManifests(items []SyncManifest, projectNamespace string) error {
	if len(items) == 0 {
		return domain.ErrDeploymentSyncManifestsEmpty
	}
	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		kind := strings.TrimSpace(item.Kind)
		name := strings.TrimSpace(item.Name)
		if kind == "" || name == "" {
			return fmt.Errorf("%w: item %d missing kind or name", domain.ErrDeploymentSyncManifestInvalid, i)
		}
		if _, ok := allowedSyncManifestKinds[kind]; !ok {
			return fmt.Errorf("%w: unsupported kind %q", domain.ErrDeploymentSyncManifestInvalid, kind)
		}
		key := kind + "/" + name
		if _, dup := seen[key]; dup {
			return fmt.Errorf("%w: duplicate %s", domain.ErrDeploymentSyncManifestInvalid, key)
		}
		seen[key] = struct{}{}

		if item.ManagedPullSecret {
			if kind != SyncManifestKindSecret {
				return fmt.Errorf("%w: managed_pull_secret requires Secret", domain.ErrDeploymentSyncManifestInvalid)
			}
			continue
		}
		if strings.TrimSpace(item.YAML) == "" {
			return fmt.Errorf("%w: %s/%s yaml is empty", domain.ErrDeploymentSyncManifestInvalid, kind, name)
		}
		if err := validateSyncManifestYAML(item.YAML, kind, name, projectNamespace); err != nil {
			return err
		}
	}
	return nil
}

func validateSyncManifestYAML(raw, expectedKind, expectedName, projectNamespace string) error {
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return fmt.Errorf("%w: parse yaml: %v", domain.ErrDeploymentSyncManifestInvalid, err)
	}
	kind, _ := doc["kind"].(string)
	if strings.TrimSpace(kind) != expectedKind {
		return fmt.Errorf("%w: kind mismatch want %s got %s", domain.ErrDeploymentSyncManifestInvalid, expectedKind, kind)
	}
	metadata, _ := doc["metadata"].(map[string]any)
	if metadata == nil {
		return fmt.Errorf("%w: missing metadata", domain.ErrDeploymentSyncManifestInvalid)
	}
	name, _ := metadata["name"].(string)
	if strings.TrimSpace(name) != expectedName {
		return fmt.Errorf("%w: name mismatch want %s got %s", domain.ErrDeploymentSyncManifestInvalid, expectedName, name)
	}
	if ns, ok := metadata["namespace"].(string); ok && strings.TrimSpace(ns) != "" {
		if projectNamespace != "" && strings.TrimSpace(ns) != strings.TrimSpace(projectNamespace) {
			return fmt.Errorf("%w: namespace mismatch", domain.ErrDeploymentSyncManifestInvalid)
		}
	}
	return nil
}

// SyncApplyOrder sorts manifests for Apply: ConfigMap → Secret → Service → Ingress → Deployment.
func SyncApplyOrder(items []SyncManifest) []SyncManifest {
	rank := map[string]int{
		SyncManifestKindConfigMap:  0,
		SyncManifestKindSecret:     1,
		SyncManifestKindService:    2,
		SyncManifestKindIngress:    3,
		SyncManifestKindDeployment: 4,
	}
	out := append([]SyncManifest(nil), items...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if rank[out[j].Kind] < rank[out[i].Kind] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
