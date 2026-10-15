package engine

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/stretchr/testify/require"
)

func inputKeys(items []RuntimeInput) []string {
	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, item.Key)
	}
	return keys
}

func TestRuntimeInputsCollectsDeclaredAndUndeclared(t *testing.T) {
	wf := &model.ProductWorkflow{
		Variables: []model.WorkflowVariable{
			{Key: "ENV", RuntimeInput: true, Description: "环境"},
			{Key: "IMAGE_TAG", Value: "latest"},
			{Key: "EMPTY_FIXED"},
		},
		Stages: []model.WorkflowStage{
			{
				Name: "构建",
				Jobs: []model.WorkflowJob{{
					Name: "build", Type: model.JobTypeSparkopsBuild,
					Spec: map[string]any{
						"branch": "{{build_branch}}",
						"tag":    "{{.IMAGE_TAG}}-{{.RUN_ID}}-{{.EMPTY_FIXED}}",
					},
				}},
			},
			{
				Name: "版本",
				Jobs: []model.WorkflowJob{{
					Name: "archive", Type: model.JobTypeSparkopsCreateVersion,
					Spec: map[string]any{
						"version_tag": "{{version}}",
						"note":        "{{.version}} {{version}}",
					},
				}},
			},
		},
	}

	inputs := RuntimeInputs(wf, nil)

	require.Equal(t, []string{"ENV", "build_branch", "version"}, inputKeys(inputs))
	require.True(t, inputs[0].Declared)
	require.Equal(t, "环境", inputs[0].Label)
	require.Empty(t, inputs[0].Refs)
	require.False(t, inputs[1].Declared)
	require.Equal(t, "build_branch", inputs[1].Label)
	require.Equal(t, []RuntimeInputRef{{
		StageIndex: 0, StageName: "构建", JobName: "build", JobType: model.JobTypeSparkopsBuild, Field: "branch",
	}}, inputs[1].Refs)
	require.Len(t, inputs[2].Refs, 2)
	require.Equal(t, "note", inputs[2].Refs[0].Field)
	require.Equal(t, "version_tag", inputs[2].Refs[1].Field)
}

func TestRuntimeInputsSkipsBarePlaceholdersInRawContent(t *testing.T) {
	wf := &model.ProductWorkflow{Stages: []model.WorkflowStage{{
		Name: "部署",
		Jobs: []model.WorkflowJob{{
			Name: "deploy", Type: model.JobTypeSparkopsDeploy,
			Spec: map[string]any{
				"config_resources": []any{map[string]any{"yaml": "name: {{name}}\ntag: {{.TAG}}"}},
			},
		}},
	}}}

	inputs := RuntimeInputs(wf, nil)

	require.Equal(t, []string{"TAG"}, inputKeys(inputs))
	require.Equal(t, "config_resources", inputs[0].Refs[0].Field)
}

func TestRuntimeInputsHonoursRunInputOverrides(t *testing.T) {
	wf := &model.ProductWorkflow{Stages: []model.WorkflowStage{{
		Name: "构建",
		Jobs: []model.WorkflowJob{{
			Name: "build", Type: model.JobTypeSparkopsBuild,
			Spec: map[string]any{"branch": "{{build_branch}}"},
		}},
	}}}

	require.Equal(t, []string{"build_branch"}, inputKeys(RuntimeInputs(wf, nil)))
	require.Empty(t, RuntimeInputs(wf, map[string]any{"branch": "main"}))
}

func TestUnresolvedPlaceholdersListsEachKeyOnce(t *testing.T) {
	stages := []model.WorkflowStage{{
		Name: "版本",
		Jobs: []model.WorkflowJob{{
			Name: "archive", Type: model.JobTypeSparkopsCreateVersion,
			Spec: map[string]any{
				"version_tag": "{{version}}",
				"note":        "{{.version}} {{ticket}}",
				"manifest":    "{{raw}}",
			},
		}},
	}}

	require.Equal(t, []string{"version", "ticket"}, UnresolvedPlaceholders(stages))
}
