package model

import "testing"

func TestApplyStageWorkflowBindings(t *testing.T) {
	current := []SprintStage{
		{ID: "s1", Key: "dev", Name: "开发", SortOrder: 0},
		{ID: "s2", Key: "test", Name: "测试", SortOrder: 1},
	}

	t.Run("only workflows changed", func(t *testing.T) {
		next := []SprintStage{
			{ID: "s1", Key: "dev", Name: "开发", SortOrder: 0, Workflows: []StageWorkflowBinding{{WorkflowID: "wf-1", TriggerMode: "manual"}}},
			{ID: "s2", Key: "test", Name: "测试", SortOrder: 1},
		}
		got, ok := ApplyStageWorkflowBindings(current, next)
		if !ok {
			t.Fatal("expected ok")
		}
		if len(got[0].Workflows) != 1 || got[0].Workflows[0].WorkflowID != "wf-1" {
			t.Fatalf("binding not applied: %+v", got[0].Workflows)
		}
		if len(current[0].Workflows) != 0 {
			t.Fatal("current stages mutated")
		}
	})

	t.Run("structure changed", func(t *testing.T) {
		renamed := []SprintStage{
			{ID: "s1", Key: "dev", Name: "研发", SortOrder: 0},
			{ID: "s2", Key: "test", Name: "测试", SortOrder: 1},
		}
		if _, ok := ApplyStageWorkflowBindings(current, renamed); ok {
			t.Fatal("rename should be rejected")
		}
		if _, ok := ApplyStageWorkflowBindings(current, current[:1]); ok {
			t.Fatal("removing a stage should be rejected")
		}
	})
}
