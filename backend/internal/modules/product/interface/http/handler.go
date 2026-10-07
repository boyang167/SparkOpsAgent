package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	createProduct       *command.CreateProductHandler
	updateProduct       *command.UpdateProductHandler
	createPlan          *command.CreateProductPlanHandler
	createRequirement   *command.CreateProductRequirementHandler
	approveRequirement  *command.ApproveRequirementHandler
	rejectRequirement   *command.RejectRequirementHandler
	releaseRequirement  *command.ReleaseRequirementHandler
	listProducts        *query.ListProductsHandler
	getProduct          *query.GetProductHandler
	listPlans           *query.ListProductPlansHandler
	listRequirements    *query.ListProductRequirementsHandler
	listProductProjects *query.ListProductProjectsHandler
	getProductOverview  *query.GetProductOverviewHandler
	releasePlans        ReleasePlanHandlers
	sprints             SprintHandlers
	sprintBoardWS       *SprintBoardWSHandler
	releasePlanCollabWS *ReleasePlanCollabWSHandler
	environments        EnvironmentHandlers
}

func NewHandler(
	createProduct *command.CreateProductHandler,
	updateProduct *command.UpdateProductHandler,
	createPlan *command.CreateProductPlanHandler,
	createRequirement *command.CreateProductRequirementHandler,
	approveRequirement *command.ApproveRequirementHandler,
	rejectRequirement *command.RejectRequirementHandler,
	releaseRequirement *command.ReleaseRequirementHandler,
	listProducts *query.ListProductsHandler,
	getProduct *query.GetProductHandler,
	listPlans *query.ListProductPlansHandler,
	listRequirements *query.ListProductRequirementsHandler,
	listProductProjects *query.ListProductProjectsHandler,
	getProductOverview *query.GetProductOverviewHandler,
	releasePlans ReleasePlanHandlers,
	sprints SprintHandlers,
	sprintBoardWS *SprintBoardWSHandler,
	releasePlanCollabWS *ReleasePlanCollabWSHandler,
	environments EnvironmentHandlers,
) *Handler {
	return &Handler{
		createProduct: createProduct, updateProduct: updateProduct,
		createPlan: createPlan, createRequirement: createRequirement,
		approveRequirement: approveRequirement, rejectRequirement: rejectRequirement,
		releaseRequirement: releaseRequirement,
		listProducts: listProducts, getProduct: getProduct,
		listPlans: listPlans, listRequirements: listRequirements,
		listProductProjects: listProductProjects, getProductOverview: getProductOverview,
		releasePlans: releasePlans, sprints: sprints, sprintBoardWS: sprintBoardWS,
		releasePlanCollabWS: releasePlanCollabWS, environments: environments,
	}
}

func memberAndWorkspace(c *gin.Context) (kernel.WorkspaceID, kernel.MemberID, bool) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	return wsID, memberID, true
}

type createProductRequest struct {
	Name        string `json:"name" binding:"required"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	NextVersion string `json:"next_version"`
}

type updateProductRequest struct {
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	NextVersion string `json:"next_version"`
}

type createPlanRequest struct {
	Name        string         `json:"name" binding:"required"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	OtherInfo   map[string]any `json:"other_info"`
}

type createRequirementRequest struct {
	ProductPlanID string         `json:"product_plan_id"`
	ProjectID     string         `json:"project_id"`
	ProjectName   string         `json:"project_name"`
	ProjectTag    string         `json:"project_tag"`
	Description   string         `json:"description"`
	Operator      string         `json:"operator"`
	OtherInfo     map[string]any `json:"other_info"`
}

func (h *Handler) ListProducts(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	items, err := h.listProducts.Handle(c.Request.Context(), wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) CreateProduct(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	var req createProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	p, err := h.createProduct.Handle(c.Request.Context(), wsID, memberID, req.Name, req.DisplayName, req.Description, req.Version, req.NextVersion)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": p.ID.String(), "name": p.Name.String(), "display_name": p.DisplayName,
		"description": p.Description, "version": p.Version, "next_version": p.NextVersion,
	})
}

func (h *Handler) GetProduct(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	item, err := h.getProduct.Handle(c.Request.Context(), wsID, memberID, productID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) GetProductOverview(c *gin.Context) {
	wsID, _, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	item, err := h.getProductOverview.Handle(c.Request.Context(), wsID, productID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) ListProductProjects(c *gin.Context) {
	wsID, _, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	items, err := h.listProductProjects.Handle(c.Request.Context(), wsID, productID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) UpdateProduct(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	var req updateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	p, err := h.updateProduct.Handle(c.Request.Context(), wsID, memberID, productID, req.DisplayName, req.Description, req.Version, req.NextVersion)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": p.ID.String(), "name": p.Name.String(), "display_name": p.DisplayName,
		"description": p.Description, "version": p.Version, "next_version": p.NextVersion,
	})
}

func (h *Handler) ListPlans(c *gin.Context) {
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	items, err := h.listPlans.Handle(c.Request.Context(), productID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) CreatePlan(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	var req createPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	plan, err := h.createPlan.Handle(c.Request.Context(), productID, memberID, req.Name, req.Version, req.Description, req.OtherInfo)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": plan.ID.String(), "product_id": plan.ProductID.String(), "name": plan.Name,
		"version": plan.Version, "status": plan.Status,
	})
}

func (h *Handler) ListRequirements(c *gin.Context) {
	markRequirementDeprecated(c)
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	items, err := h.listRequirements.Handle(c.Request.Context(), productID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) CreateRequirement(c *gin.Context) {
	markRequirementDeprecated(c)
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	var req createRequirementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	planID, _ := kernel.ParseProductPlanID(req.ProductPlanID)
	projectID, _ := kernel.ParseProjectID(req.ProjectID)
	item, err := h.createRequirement.Handle(c.Request.Context(), productID, planID, projectID, req.ProjectName, req.ProjectTag, req.Description, req.Operator, req.OtherInfo)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": item.ID.String(), "product_id": item.ProductID.String(),
		"status": item.Status, "description": item.Description,
	})
}

func (h *Handler) ApproveRequirement(c *gin.Context) {
	markRequirementDeprecated(c)
	productID, reqID, ok := parseProductRequirementIDs(c)
	if !ok {
		return
	}
	item, err := h.approveRequirement.Handle(c.Request.Context(), productID, reqID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": item.ID.String(), "status": item.Status})
}

func (h *Handler) RejectRequirement(c *gin.Context) {
	productID, reqID, ok := parseProductRequirementIDs(c)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	item, err := h.rejectRequirement.Handle(c.Request.Context(), productID, reqID, req.Reason)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": item.ID.String(), "status": item.Status})
}

func (h *Handler) ReleaseRequirement(c *gin.Context) {
	markRequirementDeprecated(c)
	productID, reqID, ok := parseProductRequirementIDs(c)
	if !ok {
		return
	}
	item, err := h.releaseRequirement.Handle(c.Request.Context(), productID, reqID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": item.ID.String(), "status": item.Status, "build_id": item.BuildID.String()})
}

func parseProductRequirementIDs(c *gin.Context) (kernel.ProductID, kernel.ProductRequirementID, bool) {
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return kernel.ProductID{}, kernel.ProductRequirementID{}, false
	}
	reqID, err := kernel.ParseProductRequirementID(c.Param("reqId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid requirement id", http.StatusBadRequest)
		return kernel.ProductID{}, kernel.ProductRequirementID{}, false
	}
	return productID, reqID, true
}
