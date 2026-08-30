package entitlements

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

// WorkspaceEntitlements describes licensed / enabled product modules for a workspace.
type WorkspaceEntitlements struct {
	Agent         bool     `json:"agent"`
	Knowledge     bool     `json:"knowledge"`
	Delivery      bool     `json:"delivery"`
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
		Observability: true,
		K8s:           false,
		Mcp:           false,
		Plugins:       []string{},
	}
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
	case strings.HasPrefix(path, "/delivery"), strings.HasPrefix(path, "/publish"),
		strings.HasPrefix(path, "/config/secrets"):
		return "delivery"
	case strings.HasPrefix(path, "/monitor"):
		return "observability"
	case strings.HasPrefix(path, "/config/webhooks"):
		return "mcp"
	case strings.HasPrefix(path, "/runtime") || strings.HasPrefix(path, "/cronjobs") ||
		strings.HasPrefix(path, "/config") || strings.HasPrefix(path, "/discovery") ||
		strings.HasPrefix(path, "/security"):
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
