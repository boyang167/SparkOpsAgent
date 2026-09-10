package command

import (
	"context"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WebhookInput struct {
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	Token   string   `json:"token"`
	Events  []string `json:"events"`
	Enabled *bool    `json:"enabled"`
}

type CreateWebhookHandler struct {
	repo    *persistence.WebhookRepository
	checker port.ProjectChecker
}

func NewCreateWebhookHandler(repo *persistence.WebhookRepository, checker port.ProjectChecker) *CreateWebhookHandler {
	return &CreateWebhookHandler{repo: repo, checker: checker}
}

func (h *CreateWebhookHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input WebhookInput,
) (*persistence.WebhookRecord, error) {
	if err := authorizeProject(h.checker, ctx, wsID, projectID); err != nil {
		return nil, err
	}
	hook, err := newWebhookRecord(kernel.NewWebhookID(), projectID, input, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, hook); err != nil {
		return nil, err
	}
	return hook, nil
}

type UpdateWebhookHandler struct {
	repo    *persistence.WebhookRepository
	checker port.ProjectChecker
}

func NewUpdateWebhookHandler(repo *persistence.WebhookRepository, checker port.ProjectChecker) *UpdateWebhookHandler {
	return &UpdateWebhookHandler{repo: repo, checker: checker}
}

func (h *UpdateWebhookHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	webhookID kernel.WebhookID,
	input WebhookInput,
) (*persistence.WebhookRecord, error) {
	if err := authorizeProject(h.checker, ctx, wsID, projectID); err != nil {
		return nil, err
	}
	existing, err := h.repo.Get(ctx, webhookID)
	if err != nil {
		return nil, domain.ErrWebhookNotFound
	}
	if existing.ProjectID != projectID {
		return nil, domain.ErrWebhookNotFound
	}
	hook, err := newWebhookRecord(existing.ID, projectID, input, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	hook.CreatedAt = existing.CreatedAt
	if err := h.repo.Save(ctx, hook); err != nil {
		return nil, err
	}
	return hook, nil
}

type DeleteWebhookHandler struct {
	repo    *persistence.WebhookRepository
	checker port.ProjectChecker
}

func NewDeleteWebhookHandler(repo *persistence.WebhookRepository, checker port.ProjectChecker) *DeleteWebhookHandler {
	return &DeleteWebhookHandler{repo: repo, checker: checker}
}

func (h *DeleteWebhookHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	webhookID kernel.WebhookID,
) error {
	if err := authorizeProject(h.checker, ctx, wsID, projectID); err != nil {
		return err
	}
	existing, err := h.repo.Get(ctx, webhookID)
	if err != nil {
		return domain.ErrWebhookNotFound
	}
	if existing.ProjectID != projectID {
		return domain.ErrWebhookNotFound
	}
	return h.repo.Delete(ctx, webhookID)
}

func newWebhookRecord(
	id kernel.WebhookID,
	projectID kernel.ProjectID,
	input WebhookInput,
	now time.Time,
) (*persistence.WebhookRecord, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 255 {
		return nil, domain.ErrInvalidWebhook
	}
	hookURL, err := ValidateWebhookURL(input.URL)
	if err != nil {
		return nil, err
	}
	events, err := NormalizeWebhookEvents(input.Events)
	if err != nil {
		return nil, err
	}
	enabled := false
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	return &persistence.WebhookRecord{
		ID: id, ProjectID: projectID, Name: name, URL: hookURL,
		Token: strings.TrimSpace(input.Token), Events: events, Enabled: enabled,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func authorizeProject(
	checker port.ProjectChecker,
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) error {
	inWs, err := checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return err
	}
	if !inWs {
		return domain.ErrProjectAccessDenied
	}
	return nil
}
