package model

import (
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkItemStageMove struct {
	WorkItemID kernel.WorkItemID
	NewStageID string
}

// PlanSprintStageUpdate validates a stage layout change and plans work item migrations.
func PlanSprintStageUpdate(
	current []SprintStage,
	next []SprintStage,
	items []*WorkItem,
	migrations map[string]string,
) ([]SprintStage, []WorkItemStageMove, error) {
	normalized, err := NormalizeSprintStages(next)
	if err != nil {
		return nil, nil, err
	}
	newIDs := map[string]struct{}{}
	for _, stage := range normalized {
		newIDs[stage.ID] = struct{}{}
	}
	removed := map[string]struct{}{}
	for _, stage := range current {
		if _, ok := newIDs[stage.ID]; !ok {
			removed[stage.ID] = struct{}{}
		}
	}
	moves := make([]WorkItemStageMove, 0)
	for _, item := range items {
		if _, ok := removed[item.StageID]; !ok {
			continue
		}
		target := strings.TrimSpace(migrations[item.StageID])
		if target == "" {
			return nil, nil, domain.ErrStageHasWorkItems
		}
		if _, ok := newIDs[target]; !ok {
			return nil, nil, domain.ErrStageHasWorkItems
		}
		moves = append(moves, WorkItemStageMove{WorkItemID: item.ID, NewStageID: target})
	}
	return normalized, moves, nil
}
