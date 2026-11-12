package service

import (
	"fmt"
	"strings"

	deploydomain "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"gopkg.in/yaml.v3"
)

// PlanEnvironmentSync merges live source and target resources into an apply plan.
func PlanEnvironmentSync(
	source, target []model.EnvironmentSyncResource,
	option model.EnvironmentSyncProjectOption,
	targetNamespace string,
) (model.EnvironmentSyncPlan, error) {
	model.NormalizeEnvironmentSyncOption(&option)

	targetByKey := indexResources(target)
	manifests := make([]model.SyncManifest, 0, len(source))
	diffs := make([]model.EnvironmentSyncDiff, 0, len(source))

	for _, src := range source {
		key := resourceKey(src.Kind, src.Name)
		tgt, hasTarget := targetByKey[key]

		syncImage, syncYAML := model.ResourceSyncModes(option, src)
		if src.ManagedPullSecret {
			if !syncYAML {
				diffs = append(diffs, model.EnvironmentSyncDiff{
					Kind: src.Kind, Name: src.Name, Action: "excluded",
					Note: "resource type not selected for sync",
				})
				continue
			}
			diff := model.EnvironmentSyncDiff{
				Kind: src.Kind, Name: src.Name, Action: "update",
				Note: "managed pull secret reference",
			}
			if hasTarget {
				diff.Before = tgt.YAML
			} else {
				diff.Action = "create"
			}
			diffs = append(diffs, diff)
			manifests = append(manifests, model.SyncManifest{
				Kind: src.Kind, Name: src.Name, ManagedPullSecret: true,
			})
			continue
		}

		if !syncImage && !syncYAML {
			diff := model.EnvironmentSyncDiff{Kind: src.Kind, Name: src.Name, Action: "excluded"}
			if hasTarget {
				diff.Before = tgt.YAML
				diff.After = tgt.YAML
				diff.Note = "resource type not selected for sync"
			} else {
				diff.Note = "resource type not selected for sync"
			}
			diffs = append(diffs, diff)
			continue
		}

		resourceOption := option
		resourceOption.SyncImage = syncImage
		resourceOption.SyncYAML = syncYAML
		mergedYAML, diff, includeManifest, err := mergeResource(src, tgt, hasTarget, resourceOption, targetNamespace)
		if err != nil {
			return model.EnvironmentSyncPlan{}, err
		}
		diffs = append(diffs, diff)
		if includeManifest {
			manifests = append(manifests, model.SyncManifest{
				Kind: src.Kind, Name: src.Name, YAML: mergedYAML,
			})
		}
	}

	plan := model.EnvironmentSyncPlan{
		Manifests: model.SyncApplyOrder(manifests),
		Diffs:     diffs,
	}
	repo, tag := extractPrimaryImage(plan.Manifests)
	plan.ImageRepository = repo
	plan.ImageTag = tag
	return plan, nil
}

func mergeResource(
	src model.EnvironmentSyncResource,
	tgt model.EnvironmentSyncResource,
	hasTarget bool,
	option model.EnvironmentSyncProjectOption,
	targetNamespace string,
) (mergedYAML string, diff model.EnvironmentSyncDiff, includeManifest bool, err error) {
	diff = model.EnvironmentSyncDiff{Kind: src.Kind, Name: src.Name}
	if hasTarget {
		diff.Before = tgt.YAML
		if src.Kind == model.SyncManifestKindSecret {
			diff.Before, err = redactSecretYAML(diff.Before)
			if err != nil {
				return "", diff, false, err
			}
		}
	}

	if !option.SyncImage && !option.SyncYAML {
		diff.Action = "unchanged"
		if hasTarget {
			diff.After = diff.Before
		}
		return "", diff, false, nil
	}

	switch {
	case option.SyncImage && !option.SyncYAML:
		if !hasTarget || !isWorkloadKind(src.Kind) {
			diff.Action = "unchanged"
			if hasTarget {
				diff.After = diff.Before
			}
			return "", diff, false, nil
		}
		mergedYAML, err = mergeImageOnly(src.YAML, tgt.YAML, targetNamespace)
	case !option.SyncImage && option.SyncYAML:
		mergedYAML, err = mergeYAMLOnly(src.YAML, tgt.YAML, hasTarget, src.Kind, targetNamespace)
	case option.SyncImage && option.SyncYAML:
		mergedYAML, err = mergeBoth(src.YAML, src.Kind, targetNamespace)
	default:
		diff.Action = "unchanged"
		return "", diff, false, nil
	}
	if err != nil {
		return "", diff, false, err
	}

	if src.Kind == model.SyncManifestKindSecret {
		mergedYAML, err = redactSecretYAML(mergedYAML)
		if err != nil {
			return "", diff, false, err
		}
		diff.Note = "secret data is not copied"
	}

	diff.After = mergedYAML
	if !hasTarget {
		diff.Action = "create"
		includeManifest = true
		return mergedYAML, diff, includeManifest, nil
	}
	if strings.TrimSpace(normalizeYAML(diff.Before)) == strings.TrimSpace(normalizeYAML(mergedYAML)) {
		diff.Action = "unchanged"
		includeManifest = false
	} else {
		diff.Action = "update"
		includeManifest = true
	}
	return mergedYAML, diff, includeManifest, nil
}

func mergeImageOnly(sourceYAML, targetYAML, targetNamespace string) (string, error) {
	targetDoc, err := parseYAMLDoc(targetYAML)
	if err != nil {
		return "", err
	}
	sourceDoc, err := parseYAMLDoc(sourceYAML)
	if err != nil {
		return "", err
	}
	sourceImages, err := extractContainerImages(sourceDoc)
	if err != nil {
		return "", err
	}
	targetImages, err := extractContainerImages(targetDoc)
	if err != nil {
		return "", err
	}
	for name, image := range sourceImages {
		if _, ok := targetImages[name]; !ok {
			continue
		}
		targetImages[name] = image
	}
	if err := applyContainerImages(targetDoc, targetImages); err != nil {
		return "", err
	}
	sanitizeResourceMetadata(targetDoc, targetNamespace)
	return marshalYAMLDoc(targetDoc)
}

func mergeYAMLOnly(sourceYAML, targetYAML string, hasTarget bool, kind, targetNamespace string) (string, error) {
	sourceDoc, err := parseYAMLDoc(sourceYAML)
	if err != nil {
		return "", err
	}
	sanitizeResourceMetadata(sourceDoc, targetNamespace)
	if !hasTarget || !isWorkloadKind(kind) {
		return marshalYAMLDoc(sourceDoc)
	}
	targetDoc, err := parseYAMLDoc(targetYAML)
	if err != nil {
		return "", err
	}
	targetImages, err := extractContainerImages(targetDoc)
	if err != nil {
		return "", err
	}
	if err := applyContainerImages(sourceDoc, targetImages); err != nil {
		return "", err
	}
	return marshalYAMLDoc(sourceDoc)
}

func mergeBoth(sourceYAML, kind, targetNamespace string) (string, error) {
	sourceDoc, err := parseYAMLDoc(sourceYAML)
	if err != nil {
		return "", err
	}
	sanitizeResourceMetadata(sourceDoc, targetNamespace)
	if kind == model.SyncManifestKindSecret {
		stripSecretData(sourceDoc)
	}
	return marshalYAMLDoc(sourceDoc)
}

func indexResources(items []model.EnvironmentSyncResource) map[string]model.EnvironmentSyncResource {
	out := make(map[string]model.EnvironmentSyncResource, len(items))
	for _, item := range items {
		out[resourceKey(item.Kind, item.Name)] = item
	}
	return out
}

func resourceKey(kind, name string) string {
	return strings.TrimSpace(kind) + "/" + strings.TrimSpace(name)
}

func isWorkloadKind(kind string) bool {
	return strings.TrimSpace(kind) == model.SyncManifestKindDeployment
}

func parseYAMLDoc(raw string) (map[string]any, error) {
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

func marshalYAMLDoc(doc map[string]any) (string, error) {
	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal yaml: %w", err)
	}
	return string(out), nil
}

func sanitizeResourceMetadata(doc map[string]any, targetNamespace string) {
	delete(doc, "status")
	metadata, ok := doc["metadata"].(map[string]any)
	if !ok {
		metadata = map[string]any{}
		doc["metadata"] = metadata
	}
	for _, key := range []string{
		"uid", "resourceVersion", "generation", "managedFields",
		"creationTimestamp", "deletionTimestamp", "deletionGracePeriodSeconds",
	} {
		delete(metadata, key)
	}
	if strings.TrimSpace(targetNamespace) != "" {
		metadata["namespace"] = targetNamespace
	}
}

func stripSecretData(doc map[string]any) {
	delete(doc, "data")
	delete(doc, "stringData")
}

func redactSecretYAML(raw string) (string, error) {
	doc, err := parseYAMLDoc(raw)
	if err != nil {
		return "", err
	}
	stripSecretData(doc)
	return marshalYAMLDoc(doc)
}

func containerLists(doc map[string]any) ([]any, []any, bool) {
	containers, okContainers := nestedSlice(doc, "spec", "template", "spec", "containers")
	initContainers, okInit := nestedSlice(doc, "spec", "template", "spec", "initContainers")
	if !okContainers && !okInit {
		return nil, nil, false
	}
	if !okContainers {
		containers = nil
	}
	if !okInit {
		initContainers = nil
	}
	return containers, initContainers, true
}

func extractContainerImages(doc map[string]any) (map[string]string, error) {
	containers, initContainers, ok := containerLists(doc)
	if !ok {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	for _, list := range [][]any{containers, initContainers} {
		for _, item := range list {
			container, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name, _ := container["name"].(string)
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, dup := out[name]; dup {
				return nil, deploydomain.ErrEnvironmentSyncAmbiguousContainer
			}
			image, _ := container["image"].(string)
			out[name] = image
		}
	}
	return out, nil
}

func applyContainerImages(doc map[string]any, images map[string]string) error {
	containers, initContainers, ok := containerLists(doc)
	if !ok {
		return nil
	}
	applyList := func(list []any) error {
		for i, item := range list {
			container, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name, _ := container["name"].(string)
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			image, ok := images[name]
			if !ok {
				continue
			}
			container["image"] = image
			list[i] = container
		}
		return nil
	}
	if err := applyList(containers); err != nil {
		return err
	}
	if err := applyList(initContainers); err != nil {
		return err
	}
	if len(containers) > 0 {
		if err := setNestedSlice(doc, containers, "spec", "template", "spec", "containers"); err != nil {
			return err
		}
	}
	if len(initContainers) > 0 {
		if err := setNestedSlice(doc, initContainers, "spec", "template", "spec", "initContainers"); err != nil {
			return err
		}
	}
	return nil
}

func nestedSlice(doc map[string]any, path ...string) ([]any, bool) {
	cur := any(doc)
	for _, key := range path[:len(path)-1] {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur = m[key]
	}
	m, ok := cur.(map[string]any)
	if !ok {
		return nil, false
	}
	slice, ok := m[path[len(path)-1]].([]any)
	return slice, ok
}

func setNestedSlice(doc map[string]any, value []any, path ...string) error {
	cur := any(doc)
	for _, key := range path[:len(path)-1] {
		m, ok := cur.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid yaml path")
		}
		next, ok := m[key]
		if !ok {
			next = map[string]any{}
			m[key] = next
		}
		cur = next
	}
	m, ok := cur.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid yaml path")
	}
	m[path[len(path)-1]] = value
	return nil
}

func extractPrimaryImage(manifests []model.SyncManifest) (repo, tag string) {
	for _, item := range manifests {
		if item.Kind != model.SyncManifestKindDeployment || strings.TrimSpace(item.YAML) == "" {
			continue
		}
		doc, err := parseYAMLDoc(item.YAML)
		if err != nil {
			continue
		}
		images, err := extractContainerImages(doc)
		if err != nil || len(images) == 0 {
			continue
		}
		containers, _, ok := containerLists(doc)
		if ok && len(containers) > 0 {
			if first, ok := containers[0].(map[string]any); ok {
				if name, _ := first["name"].(string); name != "" {
					if image := images[name]; image != "" {
						return splitImageRepoTag(image)
					}
				}
			}
		}
		for _, image := range images {
			return splitImageRepoTag(image)
		}
	}
	return "", ""
}

func splitImageRepoTag(image string) (repo, tag string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return "", ""
	}
	if idx := strings.LastIndex(image, "@"); idx > 0 {
		return image[:idx], image[idx+1:]
	}
	if idx := strings.LastIndex(image, ":"); idx > 0 {
		if strings.Contains(image[idx+1:], "/") {
			return image, ""
		}
		return image[:idx], image[idx+1:]
	}
	return image, "latest"
}

func normalizeYAML(raw string) string {
	doc, err := parseYAMLDoc(raw)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	out, err := marshalYAMLDoc(doc)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	return strings.TrimSpace(out)
}
