package model

import "sort"

// Selectable Kubernetes resource kinds for environment YAML sync.
var environmentSyncSelectableKinds = []string{
	SyncManifestKindDeployment,
	SyncManifestKindService,
	SyncManifestKindConfigMap,
	SyncManifestKindIngress,
}

func AllEnvironmentSyncResourceKinds() []string {
	out := make([]string, len(environmentSyncSelectableKinds))
	copy(out, environmentSyncSelectableKinds)
	return out
}

func NormalizeEnvironmentSyncOption(option *EnvironmentSyncProjectOption) {
	if option == nil {
		return
	}
	if option.FirstSync {
		option.SyncImage = true
		option.SyncYAML = true
		option.SyncResourceKinds = AllEnvironmentSyncResourceKinds()
		return
	}
	if !option.SyncYAML {
		option.SyncResourceKinds = nil
		return
	}
	if len(option.SyncResourceKinds) == 0 {
		option.SyncResourceKinds = AllEnvironmentSyncResourceKinds()
		return
	}
	option.SyncResourceKinds = filterAllowedEnvironmentSyncKinds(option.SyncResourceKinds)
}

func filterAllowedEnvironmentSyncKinds(kinds []string) []string {
	allowed := map[string]struct{}{}
	for _, kind := range environmentSyncSelectableKinds {
		allowed[kind] = struct{}{}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		if _, ok := allowed[kind]; !ok {
			continue
		}
		if _, dup := seen[kind]; dup {
			continue
		}
		seen[kind] = struct{}{}
		out = append(out, kind)
	}
	sort.Strings(out)
	return out
}

func SameSyncResourceKinds(a, b []string) bool {
	a = append([]string(nil), a...)
	b = append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func environmentSyncKindSelected(option EnvironmentSyncProjectOption, kind string) bool {
	if !option.SyncYAML {
		return false
	}
	kinds := option.SyncResourceKinds
	if len(kinds) == 0 {
		return true
	}
	for _, item := range kinds {
		if item == kind {
			return true
		}
	}
	return false
}

func shouldSyncManagedPullSecret(option EnvironmentSyncProjectOption) bool {
	if option.SyncImage && isEnvironmentSyncWorkloadKind(SyncManifestKindDeployment) {
		return true
	}
	return environmentSyncKindSelected(option, SyncManifestKindDeployment)
}

func isEnvironmentSyncWorkloadKind(kind string) bool {
	return kind == SyncManifestKindDeployment
}

func ResourceSyncModes(option EnvironmentSyncProjectOption, resource EnvironmentSyncResource) (syncImage, syncYAML bool) {
	if resource.ManagedPullSecret {
		if shouldSyncManagedPullSecret(option) {
			return false, true
		}
		return false, false
	}
	if option.SyncImage && isEnvironmentSyncWorkloadKind(resource.Kind) {
		syncImage = true
	}
	if option.SyncYAML && environmentSyncKindSelected(option, resource.Kind) {
		syncYAML = true
	}
	return syncImage, syncYAML
}
