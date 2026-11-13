package service

import (
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

func ReconcilerConfigFrom(cfg config.FilesystemReconcilerConfig) ReconcilerConfig {
	return ReconcilerConfig{
		Interval:      cfg.Interval,
		LeaseTTL:      cfg.LeaseTTL,
		BatchSize:     cfg.BatchSize,
		GCGracePeriod: cfg.GCGracePeriod,
	}.withDefaults()
}
