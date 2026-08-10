package openapi

import "embed"

// Files contains OpenAPI YAML specs for Swagger UI.
//
//go:embed *.yaml
var Files embed.FS
