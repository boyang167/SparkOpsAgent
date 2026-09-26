package application

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SprintProcessPort struct {
	getOrSeed *command.GetOrSeedSprintProcessHandler
	processes repository.ProcessRepository
}

func NewSprintProcessPort(getOrSeed *command.GetOrSeedSprintProcessHandler, processes repository.ProcessRepository) *SprintProcessPort {
	return &SprintProcessPort{getOrSeed: getOrSeed, processes: processes}
}

func (p *SprintProcessPort) GetOrSeedDefault(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID) (*model.SprintProcess, error) {
	return p.getOrSeed.Handle(ctx, productID, actor)
}

func (p *SprintProcessPort) FindByID(ctx context.Context, productID kernel.ProductID, processID kernel.SprintProcessID) (*model.SprintProcess, error) {
	return p.processes.FindByID(ctx, productID, processID)
}
