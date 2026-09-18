package http

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type workflowHookRequest struct {
	Branch    string            `json:"branch"`
	Variables map[string]string `json:"variables"`
}

func (h *Handler) WorkflowWebhook(c *gin.Context) {
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	workflowID, err := kernel.ParseProductWorkflowID(c.Param("workflowId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workflow id", http.StatusBadRequest)
		return
	}
	var req workflowHookRequest
	_ = c.ShouldBindJSON(&req)
	input := map[string]any{}
	if strings.TrimSpace(req.Branch) != "" {
		input["branch"] = strings.TrimSpace(req.Branch)
	}
	if len(req.Variables) > 0 {
		input["variables"] = req.Variables
	}
	run, ignored, err := h.workflows.StartRun.StartFromWebhook(c.Request.Context(), productID, workflowID, c.Param("token"), input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	if ignored || run == nil {
		c.Status(http.StatusNoContent)
		return
	}
	httpx.OK(c, gin.H{"id": run.ID.String(), "status": run.Status})
}

func (h *Handler) GitLabWorkflowHook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "failed to read body", http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(c.GetHeader("X-Gitlab-Token"))
	if token == "" {
		token = strings.TrimSpace(c.Query("token"))
	}
	run, ignored, err := h.workflows.StartRun.StartFromGitLab(c.Request.Context(), token, body)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	if ignored || run == nil {
		c.Status(http.StatusNoContent)
		return
	}
	httpx.OK(c, gin.H{"id": run.ID.String(), "status": run.Status})
}
