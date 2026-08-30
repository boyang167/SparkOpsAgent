package plugintemplate

import "embed"

// FS holds the plugin scaffold templates.
//
//go:embed plugin.yaml backend/plugin.go.tmpl frontend
var FS embed.FS
