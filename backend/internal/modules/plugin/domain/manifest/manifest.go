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
}

type MenuSpec struct {
	Title     string `yaml:"title"`
	Path      string `yaml:"path"`
	Component string `yaml:"component"`
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
		if _, ok := allowedMethods[api.Method]; !ok {
			return invalid(fmt.Sprintf("api method %q is invalid", api.Method))
		}
		if !strings.HasPrefix(api.Path, m.RoutePrefix) {
			return invalid(fmt.Sprintf("api path %q must start with route_prefix %q", api.Path, m.RoutePrefix))
		}
	}

	menuPrefix := "/plugins/" + m.ID
	for _, menu := range m.Menus {
		if !strings.HasPrefix(menu.Path, menuPrefix) {
			return invalid(fmt.Sprintf("menu path %q must start with %q", menu.Path, menuPrefix))
		}
	}

	if len(m.Frontend.Pages) == 0 {
		return invalid("at least one frontend page is required")
	}

	return nil
}

func invalid(msg string) error {
	return kernel.NewDomainError("PLUGIN_MANIFEST_INVALID", msg)
}
