package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type stubVersionCreator struct{ calls int }

func (s *stubVersionCreator) CreateFromWorkflow(context.Context, appctx.CreateVersionRequest) (kernel.ProjectVersionID, error) {
	s.calls++
	return kernel.ProjectVersionID{}, nil
}

func TestWorkflowStagesForRunAppliesPersistedVersionOverride(t *testing.T) {
	wf := &model.ProductWorkflow{Stages: []model.WorkflowStage{
		{Name: "build", Jobs: []model.WorkflowJob{{Name: "build", Type: model.JobTypeSparkopsBuild, Spec: map[string]any{}}}},
		{Name: "阶段3", Jobs: []model.WorkflowJob{{
			Name: "archive",
			Type: model.JobTypeSparkopsCreateVersion,
			Spec: map[string]any{"version_tag": "{{version}}", "source": "fromjob", "job_name": "build"},
		}}},
	}}
	run := &model.ProductWorkflowRun{
		Input:     []byte(`{"version_tag_overrides":{"archive":"v1.0.0"},"variables":{}}`),
		StageRuns: []model.StageRun{{}, {}},
	}

	stages := WorkflowStagesForRun(wf, run)

	if got := stages[1].Jobs[0].Spec["version_tag"]; got != "v1.0.0" {
		t.Fatalf("version_tag = %v, want v1.0.0", got)
	}
	if got := wf.Stages[1].Jobs[0].Spec["version_tag"]; got != "{{version}}" {
		t.Fatalf("workflow definition mutated: %v", got)
	}
}

func TestCreateVersionRejectsUnresolvedPlaceholder(t *testing.T) {
	creator := &stubVersionCreator{}
	executor := NewCreateVersionExecutor(creator)
	run := &model.ProductWorkflowRun{StageRuns: []model.StageRun{{Jobs: []model.JobRun{{Name: "archive"}}}}}
	job := model.WorkflowJob{
		Name: "archive",
		Type: model.JobTypeSparkopsCreateVersion,
		Spec: map[string]any{"version_tag": "{{version}}", "source": "fromjob", "job_name": "build"},
	}

	err := executor.RunJob(context.Background(), &model.ProductWorkflow{}, run, 0, 0, job)

	if err == nil {
		t.Fatal("expected error for unresolved placeholder")
	}
	jobRun := run.StageRuns[0].Jobs[0]
	if jobRun.Status != model.JobRunStatusFailed || !strings.Contains(jobRun.ErrorMessage, "占位符") {
		t.Fatalf("unexpected job run: %+v", jobRun)
	}
	if creator.calls != 0 {
		t.Fatalf("version creator should not be called, got %d calls", creator.calls)
	}
}
