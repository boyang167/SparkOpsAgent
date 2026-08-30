package install

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"text/template"

	plugintemplate "github.com/sparkops/sparkops-agent/backend/resource/plugin-template"
)

var scaffoldIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

type scaffoldData struct {
	ID   string
	Name string
	Pkg  string
}

// GenerateScaffold renders the plugin-template tree for id/name.
// Keys are ZIP-style paths under the top-level plugin id directory
// (e.g. "demo/plugin.yaml", "demo/backend/plugin.go").
func GenerateScaffold(id, name string) (map[string][]byte, error) {
	if !scaffoldIDPattern.MatchString(id) {
		return nil, fmt.Errorf("invalid plugin id %q", id)
	}
	if strings.TrimSpace(name) == "" {
		name = id
	}

	data := scaffoldData{
		ID:   id,
		Name: name,
		Pkg:  strings.ReplaceAll(id, "-", "_"),
	}

	out := make(map[string][]byte)
	err := fs.WalkDir(plugintemplate.FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || p == "." || path.Base(p) == "embed.go" {
			return nil
		}

		raw, err := plugintemplate.FS.ReadFile(p)
		if err != nil {
			return err
		}

		rendered, err := executeScaffoldTemplate(p, string(raw), data)
		if err != nil {
			return fmt.Errorf("render %s: %w", p, err)
		}

		rel := mapTemplatePath(p, id)
		out[path.Join(id, rel)] = []byte(rendered)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func executeScaffoldTemplate(name, content string, data scaffoldData) (string, error) {
	tmpl, err := template.New(name).Parse(content)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func mapTemplatePath(templatePath, id string) string {
	rel := strings.ReplaceAll(templatePath, "{{id}}", id)
	rel = strings.TrimSuffix(rel, ".tmpl")
	return rel
}
