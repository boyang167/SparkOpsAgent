package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrPluginNotFound        = kernel.NewDomainError("PLUGIN_NOT_FOUND", "plugin not found")
	ErrPluginBindingNotFound = kernel.NewDomainError("PLUGIN_BINDING_NOT_FOUND", "plugin binding not found")
	ErrPluginIDConflict      = kernel.NewDomainError("PLUGIN_ID_CONFLICT", "plugin id already exists")
	ErrPluginInUse           = kernel.NewDomainError("PLUGIN_IN_USE", "plugin has enabled workspace bindings")
	ErrPluginNotDeployed     = kernel.NewDomainError("PLUGIN_NOT_DEPLOYED", "plugin is not deployed")
)
