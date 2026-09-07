package service

import (
	"errors"
	"fmt"
	"io"
	"strings"

	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func extractManifestResourceNames(manifests []runtimeport.Manifest) (deploymentName, serviceName string, err error) {
	for _, manifest := range manifests {
		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest.YAML), 4096)
		for {
			var raw map[string]any
			decodeErr := decoder.Decode(&raw)
			if errors.Is(decodeErr, io.EOF) {
				break
			}
			if decodeErr != nil {
				return "", "", fmt.Errorf("parse manifest resource names: %w", decodeErr)
			}
			if len(raw) == 0 {
				continue
			}
			kind, _ := raw["kind"].(string)
			meta, _ := raw["metadata"].(map[string]any)
			name, _ := meta["name"].(string)
			name = strings.TrimSpace(name)
			switch kind {
			case "Deployment":
				deploymentName = name
			case "Service":
				serviceName = name
			}
		}
	}
	if deploymentName == "" || serviceName == "" {
		return "", "", fmt.Errorf("manifests must include Deployment and Service metadata.name")
	}
	return deploymentName, serviceName, nil
}
