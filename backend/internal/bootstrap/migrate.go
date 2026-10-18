package bootstrap

import (
	"fmt"

	"gorm.io/gorm"

	agentpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/infrastructure/persistence"
	runtimepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/persistence"
	deployrunpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/persistence"
	deliverypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	govpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	identitypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
	knowledgepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/persistence"
	notificationpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/notification/infrastructure/persistence"
	platformadminpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/infrastructure/persistence"
	pluginpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/infrastructure/persistence"
	productpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/persistence"
	pwfpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/persistence"
	sppersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/infrastructure/persistence"
	tenantpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/infrastructure/persistence"
	workflowpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/infrastructure/persistence"
	wspersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/persistence"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
)

// AutoMigrate creates BC schemas and tables via GORM.
func AutoMigrate(db *gorm.DB) error {
	if postgres.SQLMigrationsApplied(db) {
		return migratePluginSchemas(db)
	}
	steps := []struct {
		name string
		fn   func(*gorm.DB) error
	}{
		{"identity", identitypersistence.AutoMigrate},
		{"tenant", tenantpersistence.AutoMigrate},
		{"governance", govpersistence.AutoMigrate},
		{"workspace_app", wspersistence.AutoMigrate},
		{"product", productpersistence.AutoMigrate},
		{"sprint_process", sppersistence.AutoMigrate},
		{"product_workflow", pwfpersistence.AutoMigrate},
		{"platform_admin", platformadminpersistence.AutoMigrate},
		{"agent", agentpersistence.AutoMigrate},
		{"workflow", workflowpersistence.AutoMigrate},
		{"knowledge", knowledgepersistence.AutoMigrate},
		{"notification", notificationpersistence.AutoMigrate},
		{"delivery", deliverypersistence.AutoMigrate},
		{"delivery_deployrun", deployrunpersistence.AutoMigrate},
		{"agent_runtime", runtimepersistence.AutoMigrate},
		{"messaging", messagingpersistence.AutoMigrate},
		{"plugin", pluginpersistence.AutoMigrate},
		{"plugin_kubernetes", k8sschema.AutoMigrate},
	}
	for _, step := range steps {
		if err := step.fn(db); err != nil {
			return fmt.Errorf("migrate %s: %w", step.name, err)
		}
	}
	return nil
}

// migratePluginSchemas covers plugin-owned schemas, which backend/migrations does not create.
func migratePluginSchemas(db *gorm.DB) error {
	if err := k8sschema.AutoMigrate(db); err != nil {
		return fmt.Errorf("migrate plugin_kubernetes: %w", err)
	}
	return nil
}
