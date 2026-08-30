package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
)

type listRepository interface {
	ListPlugins(ctx context.Context) ([]*model.PluginRecord, error)
}

type getRepository interface {
	FindPlugin(ctx context.Context, id string) (*model.PluginRecord, error)
}

type ListHandler struct {
	repo listRepository
}

func NewListHandler(repo listRepository) *ListHandler {
	return &ListHandler{repo: repo}
}

func (h *ListHandler) Handle(ctx context.Context) ([]*model.PluginRecord, error) {
	return h.repo.ListPlugins(ctx)
}

type GetHandler struct {
	repo getRepository
}

func NewGetHandler(repo getRepository) *GetHandler {
	return &GetHandler{repo: repo}
}

func (h *GetHandler) Handle(ctx context.Context, id string) (*model.PluginRecord, error) {
	return h.repo.FindPlugin(ctx, id)
}
