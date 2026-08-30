package manifest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gopkg.in/yaml.v3"
)

var (
	idPattern      = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)
	allowedMethods = map[string]struct{}{
		"GET": {}, "POST": {}, "PUT": {}, "PATCH": {}, "DELETE": {},
	}
)

type Manifest struct {
	ID          string       `yaml:"id"`
	Version     string       `yaml:"version"`
	DisplayName string       `yaml:"display_name"`
	RoutePrefix string       `yaml:"route_prefix"`
	Entitlement string       `yaml:"entitlement"`
	Menus       []MenuSpec   `yaml:"menus"`
	APIs        []APISpec    `yaml:"apis"`
	Frontend    FrontendSpec `yaml:"frontend"`
	Assets      []string     `yaml:"assets"`
}

type MenuSpec struct {
	Title     string `yaml:"title"`
	Path      string `yaml:"path"`
	Component string `yaml:"component"`
	Parent    string `yaml:"parent"`
}

type APISpec struct {
	Method string `yaml:"method"`
	Path   string `yaml:"path"`
}

type FrontendSpec struct {
	Pages []string `yaml:"pages"`
}

func Parse(raw []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) Validate() error {
	if m.ID == "" {
		return invalid("id is required")
	}
	if !idPattern.MatchString(m.ID) {
		return invalid(fmt.Sprintf("id %q is invalid", m.ID))
	}

	expectedEntitlement := "plugin:" + m.ID
	if m.Entitlement != expectedEntitlement {
		return invalid(fmt.Sprintf("entitlement must be %q", expectedEntitlement))
	}

	expectedRoutePrefix := "/api/v1/plugins/" + m.ID
	if m.RoutePrefix != expectedRoutePrefix {
		return invalid(fmt.Sprintf("route_prefix must be %q", expectedRoutePrefix))
	}

	if len(m.APIs) == 0 {
		return invalid("at least one api is required")
	}
	for _, api := range m.APIs {
		if err := validateAPIMethod(api, m.RoutePrefix); err != nil {
			return err
		}
	}

	menuPrefix := "/plugins/" + m.ID
	menuPaths := make(map[string]struct{}, len(m.Menus))
	parentMap := make(map[string]string, len(m.Menus))
	for _, menu := range m.Menus {
		if !strings.HasPrefix(menu.Path, menuPrefix) {
			return invalid(fmt.Sprintf("menu path %q must start with %q", menu.Path, menuPrefix))
		}
		menuPaths[menu.Path] = struct{}{}
		if menu.Parent != "" {
			parentMap[menu.Path] = menu.Parent
		}
	}
	for _, menu := range m.Menus {
		if err := validateMenuParent(menu, menuPaths, parentMap); err != nil {
			return err
		}
	}

	for _, asset := range m.Assets {
		if err := validateAsset(asset); err != nil {
			return err
		}
	}

	if len(m.Frontend.Pages) == 0 {
		return invalid("at least one frontend page is required")
	}

	return nil
}

func validateAPIMethod(api APISpec, routePrefix string) error {
	if api.Method == "ANY" {
		if !strings.HasSuffix(api.Path, "/*") {
			return invalid(fmt.Sprintf("api method ANY requires path %q to end with /*", api.Path))
		}
		if !strings.HasPrefix(api.Path, routePrefix) {
			return invalid(fmt.Sprintf("api path %q must start with route_prefix %q", api.Path, routePrefix))
		}
		return nil
	}
	if _, ok := allowedMethods[api.Method]; !ok {
		return invalid(fmt.Sprintf("api method %q is invalid", api.Method))
	}
	if !strings.HasPrefix(api.Path, routePrefix) {
		return invalid(fmt.Sprintf("api path %q must start with route_prefix %q", api.Path, routePrefix))
	}
	return nil
}

func validateMenuParent(menu MenuSpec, menuPaths map[string]struct{}, parentMap map[string]string) error {
	if menu.Parent == "" {
		return nil
	}
	if menu.Parent == menu.Path {
		return invalid(fmt.Sprintf("menu %q cannot be its own parent", menu.Path))
	}
	if _, ok := menuPaths[menu.Parent]; !ok {
		return invalid(fmt.Sprintf("menu parent %q is not defined in this manifest", menu.Parent))
	}
	depth := 1
	current := menu.Path
	for {
		parent, ok := parentMap[current]
		if !ok {
			break
		}
		depth++
		if depth > 3 {
			return invalid(fmt.Sprintf("menu parent chain for %q exceeds maximum depth of 3", menu.Path))
		}
		current = parent
	}
	return nil
}

func validateAsset(asset string) error {
	if strings.HasPrefix(asset, "/") || strings.HasPrefix(asset, "\\") {
		return invalid(fmt.Sprintf("asset %q must be a relative path", asset))
	}
	if strings.Contains(asset, "\\") {
		return invalid(fmt.Sprintf("asset %q must use forward slashes", asset))
	}
	if !strings.HasPrefix(asset, "backend/") {
		return invalid(fmt.Sprintf("asset %q must start with backend/", asset))
	}
	if !strings.HasSuffix(asset, ".tar") {
		return invalid(fmt.Sprintf("asset %q must end with .tar", asset))
	}
	for _, segment := range strings.Split(asset, "/") {
		if segment == "" {
			return invalid(fmt.Sprintf("asset %q contains empty path segment", asset))
		}
		if segment == "." || segment == ".." {
			return invalid(fmt.Sprintf("asset %q contains invalid path segment %q", asset, segment))
		}
	}
	return nil
}

func invalid(msg string) error {
	return kernel.NewDomainError("PLUGIN_MANIFEST_INVALID", msg)
}
