package query

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var deploymentEnvNameRe = regexp.MustCompile(`(?m)^\s*-\s+name:\s+([^\s#]+)\s*$`)

type projectEnvEntry struct {
	Name  string
	Value string
}

func buildProjectEnvEntries(raw string) []projectEnvEntry {
	config := parseProjectConfigMap(raw)
	if len(config) == 0 {
		return nil
	}
	keys := make([]string, 0, len(config))
	for k := range config {
		k = strings.TrimSpace(k)
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]projectEnvEntry, 0, len(keys))
	for _, k := range keys {
		out = append(out, projectEnvEntry{Name: k, Value: config[k]})
	}
	return out
}

func buildProjectEnvData(raw string) []map[string]interface{} {
	entries := buildProjectEnvEntries(raw)
	out := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		out = append(out, map[string]interface{}{
			"name":  e.Name,
			"value": e.Value,
		})
	}
	return out
}

func formatProjectEnvYAML(entries []projectEnvEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "        - name: %s\n          value: %s\n", escapeYAML(e.Name), escapeYAML(e.Value))
	}
	return b.String()
}

func existingDeploymentEnvNames(yaml string) map[string]bool {
	names := map[string]bool{}
	for _, m := range deploymentEnvNameRe.FindAllStringSubmatch(yaml, -1) {
		if len(m) < 2 {
			continue
		}
		name := strings.Trim(m[1], `"'`)
		if name != "" {
			names[name] = true
		}
	}
	return names
}

func injectProjectConfigEnv(yamlContent, projectConfig string) string {
	entries := buildProjectEnvEntries(projectConfig)
	if len(entries) == 0 {
		return yamlContent
	}
	existing := existingDeploymentEnvNames(yamlContent)
	toAdd := make([]projectEnvEntry, 0, len(entries))
	for _, e := range entries {
		if !existing[e.Name] {
			toAdd = append(toAdd, e)
		}
	}
	if len(toAdd) == 0 {
		return yamlContent
	}

	itemBlock := strings.TrimRight(formatProjectEnvYAML(toAdd), "\n")
	hasEnv := strings.Contains(yamlContent, "\n        env:")
	block := itemBlock
	if !hasEnv {
		block = "        env:\n" + itemBlock
	}

	anchors := []string{"\n        envFrom:", "\n        volumeMounts:", "\n        resources:"}
	for _, anchor := range anchors {
		if idx := strings.Index(yamlContent, anchor); idx >= 0 {
			return yamlContent[:idx] + "\n" + block + yamlContent[idx:]
		}
	}

	if hasEnv {
		return yamlContent + "\n" + itemBlock
	}
	if strings.HasSuffix(yamlContent, "\n") {
		return yamlContent + block + "\n"
	}
	return yamlContent + "\n" + block + "\n"
}
