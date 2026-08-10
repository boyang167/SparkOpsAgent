package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type KnowledgeBaseView struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

type DocumentView struct {
	ID              string `json:"id"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
	Title           string `json:"title"`
	Content         string `json:"content"`
	Status          string `json:"status"`
	CreatedAt       string `json:"created_at"`
}

type ListKnowledgeBasesHandler struct {
	repo      repository.KnowledgeBaseRepository
	wsChecker port.WorkspaceChecker
}

func NewListKnowledgeBasesHandler(repo repository.KnowledgeBaseRepository, wsChecker port.WorkspaceChecker) *ListKnowledgeBasesHandler {
	return &ListKnowledgeBasesHandler{repo: repo, wsChecker: wsChecker}
}

func (h *ListKnowledgeBasesHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) ([]KnowledgeBaseView, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	items, err := h.repo.ListKnowledgeBasesByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	out := make([]KnowledgeBaseView, 0, len(items))
	for _, kb := range items {
		out = append(out, toKBView(kb))
	}
	return out, nil
}

type GetKnowledgeBaseHandler struct {
	repo      repository.KnowledgeBaseRepository
	wsChecker port.WorkspaceChecker
}

func NewGetKnowledgeBaseHandler(repo repository.KnowledgeBaseRepository, wsChecker port.WorkspaceChecker) *GetKnowledgeBaseHandler {
	return &GetKnowledgeBaseHandler{repo: repo, wsChecker: wsChecker}
}

func (h *GetKnowledgeBaseHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, kbID kernel.KnowledgeBaseID) (*KnowledgeBaseView, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	kb, err := h.repo.FindKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if kb.WorkspaceID != wsID {
		return nil, domain.ErrKnowledgeBaseNotFound
	}
	if kb.Status == model.KnowledgeBaseStatusArchived {
		return nil, domain.ErrKnowledgeBaseNotFound
	}
	view := toKBView(kb)
	return &view, nil
}

type ListDocumentsHandler struct {
	repo      repository.KnowledgeBaseRepository
	wsChecker port.WorkspaceChecker
}

func NewListDocumentsHandler(repo repository.KnowledgeBaseRepository, wsChecker port.WorkspaceChecker) *ListDocumentsHandler {
	return &ListDocumentsHandler{repo: repo, wsChecker: wsChecker}
}

func (h *ListDocumentsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, kbID kernel.KnowledgeBaseID) ([]DocumentView, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	kb, err := h.repo.FindKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if kb.WorkspaceID != wsID || kb.Status == model.KnowledgeBaseStatusArchived {
		return nil, domain.ErrKnowledgeBaseNotFound
	}
	docs, err := h.repo.ListDocumentsByKnowledgeBase(ctx, kbID)
	if err != nil {
		return nil, err
	}
	out := make([]DocumentView, 0, len(docs))
	for _, doc := range docs {
		out = append(out, toDocView(doc))
	}
	return out, nil
}

type GetDocumentHandler struct {
	repo      repository.KnowledgeBaseRepository
	wsChecker port.WorkspaceChecker
}

func NewGetDocumentHandler(repo repository.KnowledgeBaseRepository, wsChecker port.WorkspaceChecker) *GetDocumentHandler {
	return &GetDocumentHandler{repo: repo, wsChecker: wsChecker}
}

func (h *GetDocumentHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, docID kernel.DocumentID) (*DocumentView, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	doc, err := h.repo.FindDocumentByID(ctx, docID)
	if err != nil {
		return nil, err
	}
	kb, err := h.repo.FindKnowledgeBaseByID(ctx, doc.KnowledgeBaseID)
	if err != nil {
		return nil, err
	}
	if kb.WorkspaceID != wsID {
		return nil, domain.ErrDocumentNotFound
	}
	if doc.Status == model.DocumentStatusArchived {
		return nil, domain.ErrDocumentNotFound
	}
	view := toDocView(doc)
	return &view, nil
}

type SearchDocumentsHandler struct {
	repo      repository.KnowledgeBaseRepository
	wsChecker port.WorkspaceChecker
}

func NewSearchDocumentsHandler(repo repository.KnowledgeBaseRepository, wsChecker port.WorkspaceChecker) *SearchDocumentsHandler {
	return &SearchDocumentsHandler{repo: repo, wsChecker: wsChecker}
}

func (h *SearchDocumentsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, kbID kernel.KnowledgeBaseID, query string, limit int) ([]DocumentView, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	kb, err := h.repo.FindKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if kb.WorkspaceID != wsID || kb.Status == model.KnowledgeBaseStatusArchived {
		return nil, domain.ErrKnowledgeBaseNotFound
	}
	docs, err := h.repo.SearchDocumentsByKnowledgeBase(ctx, kbID, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]DocumentView, 0, len(docs))
	for _, doc := range docs {
		out = append(out, toDocView(doc))
	}
	return out, nil
}

func toKBView(kb *model.KnowledgeBase) KnowledgeBaseView {
	return KnowledgeBaseView{
		ID: kb.ID.String(), WorkspaceID: kb.WorkspaceID.String(),
		Name: kb.Name.String(), Description: kb.Description,
		Status: string(kb.Status),
		CreatedAt: kb.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func toDocView(doc *model.Document) DocumentView {
	return DocumentView{
		ID: doc.ID.String(), KnowledgeBaseID: doc.KnowledgeBaseID.String(),
		Title: doc.Title.String(), Content: doc.Content,
		Status: string(doc.Status),
		CreatedAt: doc.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
