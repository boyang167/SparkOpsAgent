package subscriber

import (
	"context"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	productmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type OnBuildCompleted struct {
	repo repository.ProductRepository
}

func NewOnBuildCompleted(repo repository.ProductRepository) *OnBuildCompleted {
	return &OnBuildCompleted{repo: repo}
}

func (s *OnBuildCompleted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(deliveryevent.BuildCompleted)
	if !ok {
		return nil
	}
	if ev.Status != string(model.BuildStatusSuccess) {
		return nil
	}

	req, err := s.repo.FindRequirementByBuildID(ctx, ev.BuildID)
	if err != nil {
		if err == domain.ErrProductRequirementNotFound {
			return nil
		}
		return err
	}
	if req.Status != productmodel.ProductRequirementStatusRunning {
		return nil
	}

	req.Status = productmodel.ProductRequirementStatusCompleted
	return s.repo.UpdateRequirement(ctx, req)
}
