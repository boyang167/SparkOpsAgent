package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func ensureMember(ctx context.Context, checker port.WorkspaceChecker, memberID kernel.MemberID, wsID kernel.WorkspaceID) error {
	ok, err := checker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrWorkspaceAccessDenied
	}
	return nil
}

type CreateKnowledgeBaseHandler struct {
	repo       repository.KnowledgeBaseRepository
	wsChecker  port.WorkspaceChecker
}

func NewCreateKnowledgeBaseHandler(repo repository.KnowledgeBaseRepository, wsChecker port.WorkspaceChecker) *CreateKnowledgeBaseHandler {
	return &CreateKnowledgeBaseHandler{repo: repo, wsChecker: wsChecker}
}

func (h *CreateKnowledgeBaseHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, rawName, description string) (*model.KnowledgeBase, error) {
	if err := ensureMember(ctx, h.wsChecker, memberID, wsID); err != nil {
		return nil, err
	}
	name, err := model.NewKnowledgeBaseName(rawName)
	if err != nil {
		return nil, err
	}
	if existing, _ := h.repo.FindKnowledgeBaseByWorkspaceAndName(ctx, wsID, name); existing != nil {
		return nil, domain.ErrKnowledgeBaseNameExists
	}
	kb := model.NewKnowledgeBase(wsID, name, description)
	if err := h.repo.SaveKnowledgeBase(ctx, kb); err != nil {
		return nil, err
	}
	return kb, nil
}

type UpdateKnowledgeBaseHandler struct {
	repo       repository.KnowledgeBaseRepository
	wsChecker  port.WorkspaceChecker
}

func NewUpdateKnowledgeBaseHandler(repo repository.KnowledgeBaseRepository, wsChecker port.WorkspaceChecker) *UpdateKnowledgeBaseHandler {
	return &UpdateKnowledgeBaseHandler{repo: repo, wsChecker: wsChecker}
}

func (h *UpdateKnowledgeBaseHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, kbID kernel.KnowledgeBaseID, rawName, description string) (*model.KnowledgeBase, error) {
	if err := ensureMember(ctx, h.wsChecker, memberID, wsID); err != nil {
		return nil, err
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
	name, err := model.NewKnowledgeBaseName(rawName)
	if err != nil {
		return nil, err
	}
	if name.String() != kb.Name.String() {
		if existing, _ := h.repo.FindKnowledgeBaseByWorkspaceAndName(ctx, wsID, name); existing != nil {
			return nil, domain.ErrKnowledgeBaseNameExists
		}
	}
	if err := kb.Update(name, description); err != nil {
		return nil, err
	}
	if err := h.repo.UpdateKnowledgeBase(ctx, kb); err != nil {
		return nil, err
	}
	return kb, nil
}

type ArchiveKnowledgeBaseHandler struct {
	repo       repository.KnowledgeBaseRepository
	wsChecker  port.WorkspaceChecker
}

func NewArchiveKnowledgeBaseHandler(repo repository.KnowledgeBaseRepository, wsChecker port.WorkspaceChecker) *ArchiveKnowledgeBaseHandler {
	return &ArchiveKnowledgeBaseHandler{repo: repo, wsChecker: wsChecker}
}

func (h *ArchiveKnowledgeBaseHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, kbID kernel.KnowledgeBaseID) error {
	if err := ensureMember(ctx, h.wsChecker, memberID, wsID); err != nil {
		return err
	}
	kb, err := h.repo.FindKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return err
	}
	if kb.WorkspaceID != wsID {
		return domain.ErrKnowledgeBaseNotFound
	}
	if err := kb.Archive(); err != nil {
		return err
	}
	return h.repo.UpdateKnowledgeBase(ctx, kb)
}

type CreateDocumentHandler struct {
	repo       repository.KnowledgeBaseRepository
	wsChecker  port.WorkspaceChecker
}

func NewCreateDocumentHandler(repo repository.KnowledgeBaseRepository, wsChecker port.WorkspaceChecker) *CreateDocumentHandler {
	return &CreateDocumentHandler{repo: repo, wsChecker: wsChecker}
}

func (h *CreateDocumentHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, kbID kernel.KnowledgeBaseID, rawTitle, content string) (*model.Document, error) {
	if err := ensureMember(ctx, h.wsChecker, memberID, wsID); err != nil {
		return nil, err
	}
	kb, err := h.repo.FindKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if kb.WorkspaceID != wsID || kb.Status == model.KnowledgeBaseStatusArchived {
		return nil, domain.ErrKnowledgeBaseNotFound
	}
	title, err := model.NewDocumentTitle(rawTitle)
	if err != nil {
		return nil, err
	}
	if existing, _ := h.repo.FindDocumentByKBAndTitle(ctx, kbID, title); existing != nil {
		return nil, domain.ErrDocumentTitleExists
	}
	doc := model.NewDocument(kbID, title, content)
	if err := h.repo.SaveDocument(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

type UpdateDocumentHandler struct {
	repo       repository.KnowledgeBaseRepository
	wsChecker  port.WorkspaceChecker
}

func NewUpdateDocumentHandler(repo repository.KnowledgeBaseRepository, wsChecker port.WorkspaceChecker) *UpdateDocumentHandler {
	return &UpdateDocumentHandler{repo: repo, wsChecker: wsChecker}
}

func (h *UpdateDocumentHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, docID kernel.DocumentID, rawTitle, content string) (*model.Document, error) {
	if err := ensureMember(ctx, h.wsChecker, memberID, wsID); err != nil {
		return nil, err
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
	title, err := model.NewDocumentTitle(rawTitle)
	if err != nil {
		return nil, err
	}
	if title.String() != doc.Title.String() {
		if existing, _ := h.repo.FindDocumentByKBAndTitle(ctx, doc.KnowledgeBaseID, title); existing != nil {
			return nil, domain.ErrDocumentTitleExists
		}
	}
	if err := doc.Update(title, content); err != nil {
		return nil, err
	}
	if err := h.repo.UpdateDocument(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

type ArchiveDocumentHandler struct {
	repo       repository.KnowledgeBaseRepository
	wsChecker  port.WorkspaceChecker
}

func NewArchiveDocumentHandler(repo repository.KnowledgeBaseRepository, wsChecker port.WorkspaceChecker) *ArchiveDocumentHandler {
	return &ArchiveDocumentHandler{repo: repo, wsChecker: wsChecker}
}

func (h *ArchiveDocumentHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, docID kernel.DocumentID) error {
	if err := ensureMember(ctx, h.wsChecker, memberID, wsID); err != nil {
		return err
	}
	doc, err := h.repo.FindDocumentByID(ctx, docID)
	if err != nil {
		return err
	}
	kb, err := h.repo.FindKnowledgeBaseByID(ctx, doc.KnowledgeBaseID)
	if err != nil {
		return err
	}
	if kb.WorkspaceID != wsID {
		return domain.ErrDocumentNotFound
	}
	if err := doc.Archive(); err != nil {
		return err
	}
	return h.repo.UpdateDocument(ctx, doc)
}
