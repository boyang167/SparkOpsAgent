package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if de, ok := kernel.IsDomainError(err); ok {
		status := http.StatusBadRequest
		switch de.Code() {
		case "IDENTITY_INVALID_CREDENTIALS", "AUTH_UNAUTHORIZED", "AUTH_INVALID_TOKEN":
			status = http.StatusUnauthorized
		case "IDENTITY_MEMBER_NOT_FOUND":
			status = http.StatusNotFound
		case "IDENTITY_EMAIL_EXISTS":
			status = http.StatusConflict
		case "GOVERNANCE_ROLE_EXISTS":
			status = http.StatusConflict
		case "IDENTITY_CANNOT_MODIFY_SELF", "IDENTITY_CANNOT_DELETE_SELF", "IDENTITY_LAST_SUPER_ADMIN":
			status = http.StatusForbidden
		case "GOVERNANCE_ROLE_NOT_FOUND", "GOVERNANCE_BINDING_NOT_FOUND", "GOVERNANCE_MENU_NOT_FOUND":
			status = http.StatusNotFound
		case "WORKSPACE_APP_PROJECT_NOT_FOUND", "AGENT_NOT_FOUND", "AGENT_VERSION_NOT_FOUND", "PRODUCT_NOT_FOUND", "PRODUCT_PLAN_NOT_FOUND", "PRODUCT_REQUIREMENT_NOT_FOUND", "PLATFORM_ADMIN_SYSCONFIG_NOT_FOUND", "PLATFORM_ADMIN_CLUSTER_NOT_FOUND", "PLATFORM_ADMIN_SYNC_JOB_NOT_FOUND", "PLATFORM_ADMIN_PROCLAIM_NOT_FOUND":
			status = http.StatusNotFound
		case "WORKSPACE_APP_PROJECT_EXISTS", "AGENT_NAME_EXISTS":
			status = http.StatusConflict
		case "WORKSPACE_APP_INVALID_NAME", "AGENT_INVALID_NAME", "WORKSPACE_APP_PROJECT_NOT_ACTIVE", "AGENT_PROJECT_NOT_ACTIVE", "AGENT_NOT_PUBLISHED", "WORKSPACE_APP_INVALID_TEMPLATE_NAME", "WORKSPACE_APP_INVALID_DOCKERFILE_NAME", "PRODUCT_INVALID_NAME":
			status = http.StatusBadRequest
		case "AGENT_ARCHIVED", "WORKSPACE_APP_PROJECT_ARCHIVED", "RUNTIME_AGENT_NOT_PUBLISHED":
			status = http.StatusBadRequest
		case "RUNTIME_AGENT_NOT_FOUND", "RUNTIME_CONVERSATION_NOT_FOUND", "RUNTIME_RUN_NOT_FOUND":
			status = http.StatusNotFound
		case "RUNTIME_ACCESS_DENIED":
			status = http.StatusForbidden
		case "DELIVERY_BUILD_NOT_FOUND":
			status = http.StatusNotFound
		case "DELIVERY_PROJECT_ACCESS_DENIED":
			status = http.StatusForbidden
		case "DELIVERY_PROJECT_NOT_ACTIVE", "DELIVERY_BUILD_RUNNING", "DELIVERY_BUILD_NOT_RUNNING", "DELIVERY_BUILD_NOT_ROLLBACKABLE":
			status = http.StatusBadRequest
		case "K8S_POD_NOT_FOUND":
			status = http.StatusNotFound
		case "K8S_PROJECT_ACCESS_DENIED":
			status = http.StatusForbidden
		}
		Fail(c, de.Code(), de.Error(), status)
		return
	}
	Fail(c, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError)
}
