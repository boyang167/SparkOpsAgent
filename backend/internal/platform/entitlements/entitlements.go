package entitlements

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/gorm"
)

// WorkspaceEntitlements describes licensed / enabled product modules for a workspace.
type WorkspaceEntitlements struct {
	Agent         bool     `json:"agent"`
	Knowledge     bool     `json:"knowledge"`
	Delivery      bool     `json:"delivery"`
	Filesystem    bool     `json:"filesystem"`
	K8s           bool     `json:"k8s"`
	Observability bool     `json:"observability"`
	Mcp           bool     `json:"mcp"`
	Plugins       []string `json:"plugins"`
}

func Default() WorkspaceEntitlements {
	return WorkspaceEntitlements{
		Agent:         true,
		Knowledge:     true,
		Delivery:      true,
		Filesystem:    true,
		Observability: true,
		K8s:           false,
		Mcp:           false,
		Plugins:       []string{},
	}
}

// UnmarshalJSON keeps filesystem enabled for payloads produced before the
// entitlement was introduced, while preserving an explicit false.
func (e *WorkspaceEntitlements) UnmarshalJSON(data []byte) error {
	type alias WorkspaceEntitlements
	if e.Agent == false && e.Knowledge == false && e.Delivery == false &&
		e.Filesystem == false && e.K8s == false && e.Observability == false &&
		e.Mcp == false && e.Plugins == nil {
		*e = Default()
	}
	decoded := alias(*e)
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*e = WorkspaceEntitlements(decoded)
	return nil
}

type sysCfgRow struct {
	Config string `gorm:"column:config"`
}

func countClusters(ctx context.Context, db *gorm.DB) (int64, error) {
	var count int64
	err := db.WithContext(ctx).
		Table("platform_admin.cluster_info").
		Where("deleted_at IS NULL").
		Count(&count).Error
	return count, err
}

func applyFeatureMap(ents *WorkspaceEntitlements, features map[string]bool) {
	if v, ok := features["agent"]; ok {
		ents.Agent = v
	}
	if v, ok := features["knowledge"]; ok {
		ents.Knowledge = v
	}
	if v, ok := features["delivery"]; ok {
		ents.Delivery = v
	}
	if v, ok := features["filesystem"]; ok {
		ents.Filesystem = v
	}
	if v, ok := features["k8s"]; ok {
		ents.K8s = v
	}
	if v, ok := features["observability"]; ok {
		ents.Observability = v
	}
	if v, ok := features["mcp"]; ok {
		ents.Mcp = v
	}
}

func asBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1" || v == "yes" || v == "on"
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return false
	}
}

// PathEntitlement maps a menu path to a module key. Empty means always visible.
func PathEntitlement(path string) string {
	if path == "" || !strings.HasPrefix(path, "/") {
		return ""
	}
	if strings.HasPrefix(path, "/plugins/") {
		rest := strings.TrimPrefix(path, "/plugins/")
		if id, _, ok := strings.Cut(rest, "/"); ok && id != "" {
			return "plugin:" + id
		} else if !ok && rest != "" {
			return "plugin:" + rest
		}
	}
	switch {
	case strings.HasPrefix(path, "/knowledge"):
		return "knowledge"
	case strings.HasPrefix(path, "/files"):
		return "filesystem"
	case strings.HasPrefix(path, "/delivery"), strings.HasPrefix(path, "/publish"),
		strings.HasPrefix(path, "/products"),
		strings.HasPrefix(path, "/config/secrets"):
		return "delivery"
	case strings.HasPrefix(path, "/monitor"):
		return "observability"
	case strings.HasPrefix(path, "/config/webhooks"), strings.HasPrefix(path, "/config/mcp"):
		return "mcp"
	case strings.HasPrefix(path, "/runtime") || strings.HasPrefix(path, "/cronjobs"):
		return "k8s"
	case strings.HasPrefix(path, "/projects"), strings.HasPrefix(path, "/templates"):
		return "agent"
	default:
		return ""
	}
}

func (e WorkspaceEntitlements) Allows(path string) bool {
	return e.AllowsModule(PathEntitlement(path))
}
