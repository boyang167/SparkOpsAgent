package policy

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

func NewEngine(cfg config.PolicyConfig, db *gorm.DB) (Engine, error) {
	switch cfg.Driver {
	case "", "casbin":
		return NewCasbinEngine(db)
	case "opa":
		if cfg.OPA.URL == "" {
			return nil, fmt.Errorf("opa url required when policy.driver=opa")
		}
		return NewOPAEngine(cfg.OPA), nil
	default:
		return nil, fmt.Errorf("unknown policy driver: %s", cfg.Driver)
	}
}
