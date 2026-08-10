package bootstrap

import (
	"fmt"

	"gorm.io/gorm"

	agentpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/infrastructure/persistence"
	runtimepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/persistence"
	deliverypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	knowledgepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/persistence"
	notificationpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/notification/infrastructure/persistence"
	platformadminpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/infrastructure/persistence"
	productpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/persistence"
	workflowpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/infrastructure/persistence"
	govpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	identitypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
	tenantpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/infrastructure/persistence"
	wspersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/persistence"
)

// AutoMigrate creates BC schemas and tables via GORM.
func AutoMigrate(db *gorm.DB) error {
	steps := []struct {
		name string
		fn   func(*gorm.DB) error
	}{
		{"identity", identitypersistence.AutoMigrate},
		{"tenant", tenantpersistence.AutoMigrate},
		{"governance", govpersistence.AutoMigrate},
		{"workspace_app", wspersistence.AutoMigrate},
		{"product", productpersistence.AutoMigrate},
		{"platform_admin", platformadminpersistence.AutoMigrate},
		{"agent", agentpersistence.AutoMigrate},
		{"workflow", workflowpersistence.AutoMigrate},
		{"knowledge", knowledgepersistence.AutoMigrate},
		{"notification", notificationpersistence.AutoMigrate},
		{"delivery", deliverypersistence.AutoMigrate},
		{"agent_runtime", runtimepersistence.AutoMigrate},
	}
	for _, step := range steps {
		if err := step.fn(db); err != nil {
			return fmt.Errorf("migrate %s: %w", step.name, err)
		}
	}
	return nil
}
