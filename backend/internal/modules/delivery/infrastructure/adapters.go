package infrastructure

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/jenkins"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

func NewJenkinsExecutor(cfg *config.Config) (port.JenkinsExecutor, error) {
	switch cfg.Delivery.Driver {
	case "jenkins":
		return jenkins.NewExecutor(cfg.JenkinsEffective())
	case "mock", "":
		return jenkins.NewMockExecutor(), nil
	default:
		return nil, fmt.Errorf("unknown delivery driver: %s", cfg.Delivery.Driver)
	}
}
