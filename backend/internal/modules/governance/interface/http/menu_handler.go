package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type MenuHandler struct {
	listTree       *query.ListMenuTreeHandler
	listNav        *query.ListNavMenusHandler
	listMyButtons  *query.ListMyMenuButtonsHandler
	getMenu        *query.GetMenuHandler
	getRoles       *query.GetMenuRolesHandler
	getRoleMenus   *query.GetRoleMenusHandler
	listWsButtons  *query.ListWorkspaceMenuButtonsHandler
	getRoleButtons *query.GetRoleMenuButtonsHandler
	create         *command.CreateMenuHandler
	update         *command.UpdateMenuHandler
	delete         *command.DeleteMenuHandler
	setRoles       *command.SetMenuRolesHandler
	setRoleMenus   *command.SetRoleMenusHandler
	setRoleButtons *command.SetRoleMenuButtonsHandler
}

func NewMenuHandler(
	listTree *query.ListMenuTreeHandler,
	listNav *query.ListNavMenusHandler,
	listMyButtons *query.ListMyMenuButtonsHandler,
	getMenu *query.GetMenuHandler,
	getRoles *query.GetMenuRolesHandler,
	getRoleMenus *query.GetRoleMenusHandler,
	listWsButtons *query.ListWorkspaceMenuButtonsHandler,
	getRoleButtons *query.GetRoleMenuButtonsHandler,
	create *command.CreateMenuHandler,
	update *command.UpdateMenuHandler,
	deleteMenu *command.DeleteMenuHandler,
	setRoles *command.SetMenuRolesHandler,
	setRoleMenus *command.SetRoleMenusHandler,
	setRoleButtons *command.SetRoleMenuButtonsHandler,
) *MenuHandler {
	return &MenuHandler{
		listTree: listTree, listNav: listNav, listMyButtons: listMyButtons,
		getMenu: getMenu, getRoles: getRoles, getRoleMenus: getRoleMenus,
		listWsButtons: listWsButtons, getRoleButtons: getRoleButtons,
		create: create, update: update, delete: deleteMenu,
		setRoles: setRoles, setRoleMenus: setRoleMenus, setRoleButtons: setRoleButtons,
	}
}

func (h *MenuHandler) ListNav(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	items, err := h.listNav.Handle(c.Request.Context(), wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *MenuHandler) ListMyButtons(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	path := c.Query("path")
	if path == "" {
		httpx.Fail(c, "INVALID_REQUEST", "path query required", http.StatusBadRequest)
		return
	}
	result, err := h.listMyButtons.Handle(c.Request.Context(), wsID, memberID, path)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *MenuHandler) ListTree(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	items, err := h.listTree.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *MenuHandler) Get(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	menu, err := h.getMenu.Handle(c.Request.Context(), wsID, c.Param("menuId"))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, menuPayload(menu))
}

type menuWriteRequest struct {
	ParentID            *string             `json:"parent_id"`
	Name                string              `json:"name" binding:"required"`
	Path                string              `json:"path" binding:"required"`
	Title               string              `json:"title" binding:"required"`
	Icon                string              `json:"icon"`
	Component           string              `json:"component"`
	Hidden              bool                `json:"hidden"`
	Sort                int                 `json:"sort"`
	KeepAlive           bool                `json:"keep_alive"`
	ActiveMenu          string              `json:"active_menu"`
	CloseTab            bool                `json:"close_tab"`
	IsBasic             bool                `json:"is_basic"`
	TransitionAnimation string              `json:"transition_animation"`
	Params              []menuParamRequest  `json:"params"`
	Buttons             []menuButtonRequest `json:"buttons"`
}

type menuParamRequest struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Key   string `json:"key"`
	Value string `json:"value"`
	Sort  int    `json:"sort"`
}

type menuButtonRequest struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Code    string   `json:"code"`
	Remark  string   `json:"remark"`
	Sort    int      `json:"sort"`
	RoleIDs []string `json:"role_ids"`
}

func toCreateMenuInput(req menuWriteRequest) command.CreateMenuInput {
	return command.CreateMenuInput{
		ParentID: req.ParentID, Name: req.Name, Path: req.Path, Title: req.Title,
		Icon: req.Icon, Component: req.Component, Hidden: req.Hidden,
		Sort: req.Sort, KeepAlive: req.KeepAlive, ActiveMenu: req.ActiveMenu,
		CloseTab: req.CloseTab, IsBasic: req.IsBasic, TransitionAnimation: req.TransitionAnimation,
		Params: toMenuParams(req.Params), Buttons: toMenuButtons(req.Buttons),
	}
}

func toUpdateMenuInput(req menuWriteRequest) command.UpdateMenuInput {
	return command.UpdateMenuInput{
		ParentID: req.ParentID, Name: req.Name, Path: req.Path, Title: req.Title,
		Icon: req.Icon, Component: req.Component, Hidden: req.Hidden,
		Sort: req.Sort, KeepAlive: req.KeepAlive, ActiveMenu: req.ActiveMenu,
		CloseTab: req.CloseTab, IsBasic: req.IsBasic, TransitionAnimation: req.TransitionAnimation,
		Params: toMenuParams(req.Params), Buttons: toMenuButtons(req.Buttons),
	}
}

func toMenuParams(items []menuParamRequest) []model.MenuParam {
	out := make([]model.MenuParam, 0, len(items))
	for _, item := range items {
		out = append(out, model.MenuParam{
			ID: item.ID, Type: item.Type, Key: item.Key, Value: item.Value, Sort: item.Sort,
		})
	}
	return out
}

func toMenuButtons(items []menuButtonRequest) []model.MenuButton {
	out := make([]model.MenuButton, 0, len(items))
	for _, item := range items {
		out = append(out, model.MenuButton{
			ID: item.ID, Name: item.Name, Code: item.Code,
			Remark: item.Remark, Sort: item.Sort, RoleIDs: item.RoleIDs,
		})
	}
	return out
}

func (h *MenuHandler) Create(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	var req menuWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	menu, err := h.create.Handle(c.Request.Context(), wsID, toCreateMenuInput(req))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, menuPayload(menu))
}

func (h *MenuHandler) Update(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	var req menuWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.update.Handle(c.Request.Context(), wsID, c.Param("menuId"), toUpdateMenuInput(req)); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": true})
}

func (h *MenuHandler) Delete(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	if err := h.delete.Handle(c.Request.Context(), wsID, c.Param("menuId")); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *MenuHandler) GetRoles(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	roleIDs, err := h.getRoles.Handle(c.Request.Context(), wsID, c.Param("menuId"))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"role_ids": roleIDs})
}

type setMenuRolesRequest struct {
	RoleIDs []string `json:"role_ids"`
}

func (h *MenuHandler) SetRoles(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	var req setMenuRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.setRoles.Handle(c.Request.Context(), wsID, c.Param("menuId"), req.RoleIDs); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": true})
}

func (h *MenuHandler) GetRoleMenus(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	menuIDs, err := h.getRoleMenus.Handle(c.Request.Context(), wsID, c.Param("roleId"))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"menu_ids": menuIDs})
}

type setRoleMenusRequest struct {
	MenuIDs []string `json:"menu_ids"`
}

func (h *MenuHandler) SetRoleMenus(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	var req setRoleMenusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.setRoleMenus.Handle(c.Request.Context(), wsID, c.Param("roleId"), req.MenuIDs); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": true})
}

func (h *MenuHandler) ListWorkspaceMenuButtons(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	items, err := h.listWsButtons.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *MenuHandler) GetRoleMenuButtons(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	buttonIDs, err := h.getRoleButtons.Handle(c.Request.Context(), wsID, c.Param("roleId"))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"button_ids": buttonIDs})
}

type setRoleMenuButtonsRequest struct {
	ButtonIDs []string `json:"button_ids"`
}

func (h *MenuHandler) SetRoleMenuButtons(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "workspace required", http.StatusBadRequest)
		return
	}
	var req setRoleMenuButtonsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.setRoleButtons.Handle(c.Request.Context(), wsID, c.Param("roleId"), req.ButtonIDs); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": true})
}

func menuPayload(menu *model.Menu) gin.H {
	params := make([]gin.H, 0, len(menu.Params))
	for _, p := range menu.Params {
		params = append(params, gin.H{
			"id": p.ID, "type": p.Type, "key": p.Key, "value": p.Value, "sort": p.Sort,
		})
	}
	buttons := make([]gin.H, 0, len(menu.Buttons))
	for _, b := range menu.Buttons {
		buttons = append(buttons, gin.H{
			"id": b.ID, "name": b.Name, "code": b.Code, "remark": b.Remark,
			"sort": b.Sort, "role_ids": b.RoleIDs,
		})
	}
	return gin.H{
		"id": menu.ID, "parent_id": menu.ParentID, "name": menu.Name, "path": menu.Path,
		"title": menu.Title, "icon": menu.Icon, "component": menu.Component,
		"hidden": menu.Hidden, "sort": menu.Sort, "keep_alive": menu.KeepAlive,
		"active_menu": menu.ActiveMenu, "close_tab": menu.CloseTab, "is_basic": menu.IsBasic,
		"transition_animation": menu.TransitionAnimation,
		"params": params, "buttons": buttons,
	}
}
