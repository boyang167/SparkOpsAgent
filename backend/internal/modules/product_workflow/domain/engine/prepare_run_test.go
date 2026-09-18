package engine

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestInterpolateLeavesUnknownKeys(t *testing.T) {
	got := Interpolate("image:{{.TAG}}/{{.MISSING}}", map[string]string{"TAG": "v1"})
	require.Equal(t, "image:v1/{{.MISSING}}", got)
}

func TestInterpolateStagesSkipsConfigYAML(t *testing.T) {
	stages := []model.WorkflowStage{{
		Name: "部署",
		Jobs: []model.WorkflowJob{{
			Name: "deploy",
			Type: model.JobTypeSparkopsDeploy,
			Spec: map[string]any{
				"branch": "{{.BRANCH}}",
				"config_resources": []any{
					map[string]any{"yaml": "kind: ConfigMap\nvalue: {{.BRANCH}}"},
				},
			},
		}},
	}}
	out := InterpolateStages(stages, map[string]string{"BRANCH": "main"})
	require.Equal(t, "main", out[0].Jobs[0].Spec["branch"])
	resources := out[0].Jobs[0].Spec["config_resources"].([]any)
	row := resources[0].(map[string]any)
	require.Contains(t, row["yaml"], "value: main")
	require.NotContains(t, row["yaml"], "{{.BRANCH}}")
}

func TestPrepareRunGitOverridesOnlyMatchingProject(t *testing.T) {
	wf := &model.ProductWorkflow{
		Name: "更新开发环境", Key: "dev", ProductID: kernel.NewProductID(),
		Variables: []model.WorkflowVariable{{Key: "IMAGE_TAG", Value: "latest"}},
		Stages: []model.WorkflowStage{{
			Name: "构建",
			Jobs: []model.WorkflowJob{{
				Name: "build-1", Type: model.JobTypeSparkopsBuild,
				Spec: map[string]any{
					"project_builds": []any{
						map[string]any{"project_id": "p1", "branch": "{{.GIT_BRANCH}}"},
						map[string]any{"project_id": "p2", "branch": "main"},
					},
				},
			}},
		}},
	}
	stages, stored, trigger, err := PrepareRun(wf, "run-1", "git", nil, &GitRunContext{
		ProjectID: "p1", Event: "push", Ref: "feature/login", RefType: "branch", Commit: "abc",
	})
	require.NoError(t, err)
	require.Equal(t, "git", trigger["source"])
	vars := stored["variables"].(map[string]string)
	require.Equal(t, "feature/login", vars["GIT_BRANCH"])
	require.Equal(t, "latest", vars["IMAGE_TAG"])
	builds := stages[0].Jobs[0].Spec["project_builds"].([]any)
	require.Equal(t, "feature/login", builds[0].(map[string]any)["branch"])
	require.Equal(t, "branch", builds[0].(map[string]any)["git_ref_type"])
	require.Equal(t, "main", builds[1].(map[string]any)["branch"])
}

func TestPrepareRunRequiresRuntimeVariable(t *testing.T) {
	wf := &model.ProductWorkflow{
		Variables: []model.WorkflowVariable{{Key: "ENV", RuntimeInput: true}},
		Stages:    []model.WorkflowStage{{Name: "阶段", Jobs: []model.WorkflowJob{}}},
	}
	_, _, _, err := PrepareRun(wf, "run-1", "cron", nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ENV")
}

func TestGitURLAndRefFilter(t *testing.T) {
	require.True(t, GitURLMatches("git@gitlab.example.com:group/orders.git", "https://gitlab.example.com/group/orders.git", "group/orders"))
	require.False(t, GitURLMatches("https://gitlab.example.com/group/other.git", "", "group/orders"))
	require.True(t, MatchRefFilter("main, release/*", "release/1.2"))
	require.False(t, MatchRefFilter("main", "develop"))
}

func TestParseGitLabPush(t *testing.T) {
	body := []byte(`{"object_kind":"push","ref":"refs/heads/main","checkout_sha":"abc","project":{"path_with_namespace":"group/orders","git_http_url":"https://gitlab.example.com/group/orders.git"}}`)
	hook, skip, err := ParseGitLabHook(body)
	require.NoError(t, err)
	require.False(t, skip)
	require.Equal(t, "push", hook.Event)
	require.Equal(t, "main", hook.Ref)
	require.Equal(t, "branch", hook.RefType)
	require.Equal(t, "abc", hook.Commit)
}
