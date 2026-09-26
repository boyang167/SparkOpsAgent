package query

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func ReplaceDeploymentImage(raw, image string) (string, error) {
	var manifest map[string]any
	if err := yaml.Unmarshal([]byte(raw), &manifest); err != nil {
		return "", fmt.Errorf("parse deployment manifest: %w", err)
	}
	spec, ok := manifest["spec"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("deployment manifest missing spec")
	}
	template, ok := spec["template"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("deployment manifest missing pod template")
	}
	podSpec, ok := template["spec"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("deployment manifest missing pod spec")
	}
	containers, ok := podSpec["containers"].([]any)
	if !ok || len(containers) == 0 {
		return "", fmt.Errorf("deployment manifest has no containers")
	}
	for _, item := range containers {
		container, ok := item.(map[string]any)
		if !ok {
			continue
		}
		container["image"] = image
	}
	encoded, err := yaml.Marshal(manifest)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
