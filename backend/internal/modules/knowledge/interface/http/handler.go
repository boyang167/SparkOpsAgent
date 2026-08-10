package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	createKB   *command.CreateKnowledgeBaseHandler
	updateKB   *command.UpdateKnowledgeBaseHandler
	archiveKB  *command.ArchiveKnowledgeBaseHandler
	createDoc  *command.CreateDocumentHandler
	updateDoc  *command.UpdateDocumentHandler
	archiveDoc *command.ArchiveDocumentHandler
	listKB     *query.ListKnowledgeBasesHandler
	getKB      *query.GetKnowledgeBaseHandler
	listDocs   *query.ListDocumentsHandler
	getDoc     *query.GetDocumentHandler
	searchDocs *query.SearchDocumentsHandler
}

func NewHandler(
	createKB *command.CreateKnowledgeBaseHandler,
	updateKB *command.UpdateKnowledgeBaseHandler,
	archiveKB *command.ArchiveKnowledgeBaseHandler,
	createDoc *command.CreateDocumentHandler,
	updateDoc *command.UpdateDocumentHandler,
	archiveDoc *command.ArchiveDocumentHandler,
	listKB *query.ListKnowledgeBasesHandler,
	getKB *query.GetKnowledgeBaseHandler,
	listDocs *query.ListDocumentsHandler,
	getDoc *query.GetDocumentHandler,
	searchDocs *query.SearchDocumentsHandler,
) *Handler {
	return &Handler{
		createKB: createKB, updateKB: updateKB, archiveKB: archiveKB,
		createDoc: createDoc, updateDoc: updateDoc, archiveDoc: archiveDoc,
		listKB: listKB, getKB: getKB, listDocs: listDocs, getDoc: getDoc, searchDocs: searchDocs,
	}
}

type createKnowledgeBaseRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type updateKnowledgeBaseRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type createDocumentRequest struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content"`
}

type updateDocumentRequest struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content"`
}

type searchDocumentsRequest struct {
	Query string `json:"query" binding:"required"`
	Limit int    `json:"limit"`
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

func (h *Handler) CreateKnowledgeBase(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	var req createKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	kb, err := h.createKB.Handle(c.Request.Context(), wsID, memberID, req.Name, req.Description)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": kb.ID.String(), "name": kb.Name.String(),
		"description": kb.Description, "status": kb.Status,
	})
}

func (h *Handler) ListKnowledgeBases(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	items, err := h.listKB.Handle(c.Request.Context(), wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetKnowledgeBase(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	kbID, err := kernel.ParseKnowledgeBaseID(c.Param("kbId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid knowledge base id", http.StatusBadRequest)
		return
	}
	item, err := h.getKB.Handle(c.Request.Context(), wsID, memberID, kbID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) UpdateKnowledgeBase(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	kbID, err := kernel.ParseKnowledgeBaseID(c.Param("kbId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid knowledge base id", http.StatusBadRequest)
		return
	}
	var req updateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	kb, err := h.updateKB.Handle(c.Request.Context(), wsID, memberID, kbID, req.Name, req.Description)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": kb.ID.String(), "name": kb.Name.String(),
		"description": kb.Description, "status": kb.Status,
	})
}

func (h *Handler) ArchiveKnowledgeBase(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	kbID, err := kernel.ParseKnowledgeBaseID(c.Param("kbId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid knowledge base id", http.StatusBadRequest)
		return
	}
	if err := h.archiveKB.Handle(c.Request.Context(), wsID, memberID, kbID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *Handler) CreateDocument(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	kbID, err := kernel.ParseKnowledgeBaseID(c.Param("kbId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid knowledge base id", http.StatusBadRequest)
		return
	}
	var req createDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	doc, err := h.createDoc.Handle(c.Request.Context(), wsID, memberID, kbID, req.Title, req.Content)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": doc.ID.String(), "title": doc.Title.String(),
		"content": doc.Content, "status": doc.Status,
	})
}

func (h *Handler) ListDocuments(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	kbID, err := kernel.ParseKnowledgeBaseID(c.Param("kbId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid knowledge base id", http.StatusBadRequest)
		return
	}
	items, err := h.listDocs.Handle(c.Request.Context(), wsID, memberID, kbID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetDocument(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	docID, err := kernel.ParseDocumentID(c.Param("docId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid document id", http.StatusBadRequest)
		return
	}
	item, err := h.getDoc.Handle(c.Request.Context(), wsID, memberID, docID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) UpdateDocument(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	docID, err := kernel.ParseDocumentID(c.Param("docId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid document id", http.StatusBadRequest)
		return
	}
	var req updateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	doc, err := h.updateDoc.Handle(c.Request.Context(), wsID, memberID, docID, req.Title, req.Content)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": doc.ID.String(), "title": doc.Title.String(),
		"content": doc.Content, "status": doc.Status,
	})
}

func (h *Handler) ArchiveDocument(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	docID, err := kernel.ParseDocumentID(c.Param("docId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid document id", http.StatusBadRequest)
		return
	}
	if err := h.archiveDoc.Handle(c.Request.Context(), wsID, memberID, docID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *Handler) SearchDocuments(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	kbID, err := kernel.ParseKnowledgeBaseID(c.Param("kbId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid knowledge base id", http.StatusBadRequest)
		return
	}
	var req searchDocumentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	items, err := h.searchDocs.Handle(c.Request.Context(), wsID, memberID, kbID, req.Query, req.Limit)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}
