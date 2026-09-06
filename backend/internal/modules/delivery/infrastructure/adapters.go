package infrastructure

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/jenkins"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"gorm.io/gorm"
)

func NewJenkinsExecutorProvider(cfg *config.Config, db *gorm.DB, box *crypto.SecretBox) (port.JenkinsExecutorProvider, error) {
	switch cfg.Delivery.Driver {
	case "jenkins":
		if db == nil || box == nil {
			return nil, fmt.Errorf("delivery jenkins provider requires database and secret encryption")
		}
		return jenkins.NewProjectExecutorProvider(db, box, cfg.Jenkins.JobSuffix), nil
	case "mock", "":
		return jenkins.NewStaticProvider(jenkins.NewMockExecutor()), nil
	default:
		return nil, fmt.Errorf("unknown delivery driver: %s", cfg.Delivery.Driver)
	}
}
