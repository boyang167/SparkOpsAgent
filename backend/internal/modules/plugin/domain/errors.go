package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrPluginNotFound        = kernel.NewDomainError("PLUGIN_NOT_FOUND", "plugin not found")
	ErrPluginBindingNotFound = kernel.NewDomainError("PLUGIN_BINDING_NOT_FOUND", "plugin binding not found")
)
