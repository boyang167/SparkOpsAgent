package keyval

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var ErrInvalidResolvedDefinition = kernel.NewDomainError("WORKFLOW_TEMPLATE_INVALID_RESOLVED", "resolved definition is not valid JSON")
