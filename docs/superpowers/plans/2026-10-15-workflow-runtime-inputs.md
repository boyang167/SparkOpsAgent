# 工作流运行输入契约与发布参数 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 工作流在启动前就能声明并校验自己需要的运行参数；发布计划在规划阶段统一填写这些参数（计划级继承 + 发布项覆盖），缺参时拦截「完成规划」。

**Architecture:** product_workflow 引擎新增占位符扫描（`RuntimeInputs` / `UnresolvedPlaceholders`），运行时统一替换 `{{.key}}` 与 `{{key}}`，`PrepareRun` 拦截残留占位符。通过 `appctx.WorkflowRuntimeInputReader` 端口把清单提供给 product 模块：发布计划新增 `parameters`，执行时按所需键合并进运行变量，完成规划前校验齐全。前端读取 `runtime_inputs` 渲染「发布参数」区块、发布项「运行参数」字段，并在运行弹窗、工作流编辑器中提示未声明占位符。

**Tech Stack:** Go 1.26（gin、gorm、testify）、Postgres jsonb、Next.js 15 / React 19、zod、vitest。

规范：`docs/superpowers/specs/2026-10-15-workflow-runtime-inputs-design.md`

## Global Constraints

- 占位符键规则：`[A-Za-z][A-Za-z0-9_]*`；语法 `{{.KEY}}` 与 `{{key}}`，花括号内不允许空格。
- 原样内容字段键名：`content`、`yaml`、`manifest`、`template`，其中只识别 / 替换 `{{.KEY}}`。
- 发布项参数保留键：`branch`、`environment_id`，以及所有 `_` 前缀键；同名输入只能在计划级填写。
- 敏感值掩码常量：`******`（前后端一致）。
- 缺参错误码：`RELEASE_PLAN_MISSING_INPUTS`，HTTP 422；工作流缺失沿用 `RELEASE_PLAN_INVALID_WORKFLOW_REF`。
- 迁移文件序号：`000068_release_plan_parameters`。
- 文案：「发布参数」「运行参数」「继承：x」「未填写」「单独填写」「恢复继承」「声明为变量」。
- 后端改动后执行 `.\scripts\dev-restart.ps1 -Backend`。
- 后端测试命令在 `backend/` 下执行；前端测试在 `fronted/` 下执行 `npx vitest run <file>`。
- 本机 git 2.31 不支持环境注入的 `--trailer`，`git commit` 失败时由用户手动提交，不要改 git 配置。
- 前端遵循 `.cursor/skills/apple-restrained-ui` 与 `.cursor/skills/fronted`：近无色、留白分层、警示色只用于缺参。

---

### Task 1: 引擎占位符扫描与运行输入清单

**Files:**
- Create: `backend/internal/modules/product_workflow/domain/engine/runtime_inputs.go`
- Test: `backend/internal/modules/product_workflow/domain/engine/runtime_inputs_test.go`

**Interfaces:**
- Consumes: `ApplyRunInput(stages, input)`、`cloneStages(stages)`、`systemVariables(wf, runID, git)`（均在 engine 包内）。
- Produces:
  - `var placeholderPattern *regexp.Regexp`（分组 1 为可选 `.`，分组 2 为 key），Task 2 复用。
  - `func isRawContentKey(key string) bool`，Task 2 复用。
  - `type RuntimeInputRef struct{ StageIndex int; StageName, JobName, JobType, Field string }`（json：`stage_index`、`stage_name`、`job_name`、`job_type`、`field`）。
  - `type RuntimeInput struct{ Key, Label string; Declared, Secret bool; Refs []RuntimeInputRef }`（json：`key`、`label`、`declared`、`secret,omitempty`、`refs,omitempty`）。
  - `func RuntimeInputs(wf *model.ProductWorkflow, input map[string]any) []RuntimeInput`
  - `func UnresolvedPlaceholders(stages []model.WorkflowStage) []string`

- [ ] **Step 1: 写失败测试**

```go
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
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/product_workflow/domain/engine/ -run "TestRuntimeInputs|TestUnresolvedPlaceholders" -v`
Expected: 编译失败，`undefined: RuntimeInputs`、`undefined: RuntimeInputRef`、`undefined: UnresolvedPlaceholders`。

- [ ] **Step 3: 实现 `runtime_inputs.go`**

```go
package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
)

// placeholderPattern matches {{.KEY}} (run variables) and {{key}} (template leftovers).
var placeholderPattern = regexp.MustCompile(`\{\{(\.?)([A-Za-z][A-Za-z0-9_]*)\}\}`)

// rawContentKeys hold verbatim file bodies that may carry their own {{name}} text;
// only {{.KEY}} is recognised inside them.
var rawContentKeys = map[string]struct{}{
	"content":  {},
	"yaml":     {},
	"manifest": {},
	"template": {},
}

func isRawContentKey(key string) bool {
	_, ok := rawContentKeys[key]
	return ok
}

type RuntimeInputRef struct {
	StageIndex int    `json:"stage_index"`
	StageName  string `json:"stage_name"`
	JobName    string `json:"job_name"`
	JobType    string `json:"job_type"`
	Field      string `json:"field"`
}

type RuntimeInput struct {
	Key      string            `json:"key"`
	Label    string            `json:"label"`
	Declared bool              `json:"declared"`
	Secret   bool              `json:"secret,omitempty"`
	Refs     []RuntimeInputRef `json:"refs,omitempty"`
}

// RuntimeInputs lists the values a run must provide once input overrides are applied.
func RuntimeInputs(wf *model.ProductWorkflow, input map[string]any) []RuntimeInput {
	out := []RuntimeInput{}
	if wf == nil {
		return out
	}
	declared := map[string]model.WorkflowVariable{}
	for _, item := range wf.Variables {
		if key := strings.TrimSpace(item.Key); key != "" {
			declared[key] = item
		}
	}
	system := systemVariables(nil, "", nil)
	index := map[string]int{}
	seenRef := map[string]struct{}{}
	add := func(key string, ref *RuntimeInputRef) {
		pos, ok := index[key]
		if !ok {
			item := RuntimeInput{Key: key, Label: key}
			if variable, isDeclared := declared[key]; isDeclared {
				item.Declared = true
				item.Secret = variable.Secret
				if desc := strings.TrimSpace(variable.Description); desc != "" {
					item.Label = desc
				}
			}
			out = append(out, item)
			pos = len(out) - 1
			index[key] = pos
		}
		if ref == nil {
			return
		}
		refKey := fmt.Sprintf("%s|%d|%s|%s", key, ref.StageIndex, ref.JobName, ref.Field)
		if _, dup := seenRef[refKey]; dup {
			return
		}
		seenRef[refKey] = struct{}{}
		out[pos].Refs = append(out[pos].Refs, *ref)
	}
	for _, item := range wf.Variables {
		if key := strings.TrimSpace(item.Key); key != "" && item.RuntimeInput {
			add(key, nil)
		}
	}
	stages := ApplyRunInput(cloneStages(wf.Stages), input)
	scanStagePlaceholders(stages, func(key string, ref RuntimeInputRef) {
		if _, ok := system[key]; ok {
			return
		}
		if variable, ok := declared[key]; ok && !variable.RuntimeInput {
			return
		}
		add(key, &ref)
	})
	return out
}

// UnresolvedPlaceholders lists placeholder keys still present in stage specs, in first-seen order.
func UnresolvedPlaceholders(stages []model.WorkflowStage) []string {
	keys := []string{}
	seen := map[string]struct{}{}
	scanStagePlaceholders(stages, func(key string, _ RuntimeInputRef) {
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	})
	return keys
}

func scanStagePlaceholders(stages []model.WorkflowStage, visit func(key string, ref RuntimeInputRef)) {
	for si, stage := range stages {
		for _, job := range stage.Jobs {
			spec := genericSpec(job.Spec)
			for _, field := range sortedKeys(spec) {
				ref := RuntimeInputRef{
					StageIndex: si, StageName: stage.Name, JobName: job.Name, JobType: job.Type, Field: field,
				}
				scanPlaceholderValue(spec[field], !isRawContentKey(field), func(key string) { visit(key, ref) })
			}
		}
	}
}

func scanPlaceholderValue(value any, bare bool, visit func(key string)) {
	switch typed := value.(type) {
	case string:
		for _, match := range placeholderPattern.FindAllStringSubmatch(typed, -1) {
			if match[1] == "" && !bare {
				continue
			}
			visit(match[2])
		}
	case []any:
		for _, item := range typed {
			scanPlaceholderValue(item, bare, visit)
		}
	case map[string]any:
		for _, key := range sortedKeys(typed) {
			scanPlaceholderValue(typed[key], bare && !isRawContentKey(key), visit)
		}
	}
}

// genericSpec normalises typed slices (e.g. []map[string]any) into plain JSON values.
func genericSpec(spec map[string]any) map[string]any {
	raw, err := json.Marshal(spec)
	if err != nil {
		return spec
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return spec
	}
	return out
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/product_workflow/domain/engine/ -run "TestRuntimeInputs|TestUnresolvedPlaceholders" -v`
Expected: 4 个测试 PASS。若 `sortedKeys` 与包内已有函数重名导致编译失败，改名为 `sortedSpecKeys` 并同步替换。

- [ ] **Step 5: 提交**

```bash
git add backend/internal/modules/product_workflow/domain/engine/runtime_inputs.go backend/internal/modules/product_workflow/domain/engine/runtime_inputs_test.go
git commit -m "feat(workflow): scan runtime inputs from stage placeholders"
```

---

### Task 2: 统一替换两种占位符，PrepareRun 拦截残留

**Files:**
- Modify: `backend/internal/modules/product_workflow/domain/engine/interpolate.go`
- Modify: `backend/internal/modules/product_workflow/domain/engine/prepare_run.go:20-40`
- Test: `backend/internal/modules/product_workflow/domain/engine/prepare_run_test.go`

**Interfaces:**
- Consumes: Task 1 的 `placeholderPattern`、`isRawContentKey`、`UnresolvedPlaceholders`。
- Produces: `Interpolate(value, vars)` 签名不变；`PrepareRun` 签名不变，残留占位符时返回 `PRODUCT_WORKFLOW_VARIABLE_REQUIRED` 错误。

- [ ] **Step 1: 在 `prepare_run_test.go` 末尾追加失败测试**

```go
func versionWorkflow() *model.ProductWorkflow {
	return &model.ProductWorkflow{Stages: []model.WorkflowStage{
		{Name: "构建", Jobs: []model.WorkflowJob{{Name: "build-1", Type: model.JobTypeSparkopsBuild}}},
		{Name: "版本", Jobs: []model.WorkflowJob{{
			Name: "archive", Type: model.JobTypeSparkopsCreateVersion,
			Spec: map[string]any{"version_tag": "{{version}}"},
		}}},
	}}
}

func TestInterpolateReplacesBothSyntaxes(t *testing.T) {
	got := Interpolate("{{.TAG}}/{{version}}/{{missing}}", map[string]string{"TAG": "v1", "version": "1.2.0"})
	require.Equal(t, "v1/1.2.0/{{missing}}", got)
}

func TestInterpolateStagesKeepsBarePlaceholdersInRawContent(t *testing.T) {
	stages := []model.WorkflowStage{{
		Name: "部署",
		Jobs: []model.WorkflowJob{{
			Name: "deploy", Type: model.JobTypeSparkopsDeploy,
			Spec: map[string]any{
				"config_resources": []any{map[string]any{"yaml": "name: {{name}}\ntag: {{.TAG}}"}},
			},
		}},
	}}
	out := InterpolateStages(stages, map[string]string{"name": "x", "TAG": "v1"})
	row := out[0].Jobs[0].Spec["config_resources"].([]any)[0].(map[string]any)
	require.Equal(t, "name: {{name}}\ntag: v1", row["yaml"])
}

func TestPrepareRunReplacesTemplatePlaceholderFromVariables(t *testing.T) {
	stages, _, _, err := PrepareRun(versionWorkflow(), "run-1", "manual", map[string]any{
		"variables": map[string]any{"version": "1.2.0"},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "1.2.0", stages[1].Jobs[0].Spec["version_tag"])
}

func TestPrepareRunRejectsUnresolvedPlaceholders(t *testing.T) {
	_, _, _, err := PrepareRun(versionWorkflow(), "run-1", "manual", nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "version")
}

func TestPrepareRunAcceptsVersionTagOverride(t *testing.T) {
	stages, _, _, err := PrepareRun(versionWorkflow(), "run-1", "manual", map[string]any{
		"version_tag_overrides": map[string]any{"archive": "v1.0.0"},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "v1.0.0", stages[1].Jobs[0].Spec["version_tag"])
}

func TestPrepareRunIgnoresPlaceholdersInSkippedStages(t *testing.T) {
	stages, _, _, err := PrepareRun(versionWorkflow(), "run-1", "manual", map[string]any{
		"stage_indices": []any{float64(0)},
	}, nil)
	require.NoError(t, err)
	require.Len(t, stages, 1)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/product_workflow/domain/engine/ -run "TestInterpolate|TestPrepareRun" -v`
Expected: `TestInterpolateReplacesBothSyntaxes`、`TestPrepareRunReplacesTemplatePlaceholderFromVariables`、`TestPrepareRunRejectsUnresolvedPlaceholders` FAIL；其余通过。

- [ ] **Step 3: 改写 `interpolate.go` 的替换逻辑**

删除 `interpolatePattern`，把 `Interpolate`、`interpolateValue` 和 `InterpolateStages` 中的调用改为：

```go
func Interpolate(value string, vars map[string]string) string {
	return interpolate(value, vars, true)
}

// interpolate replaces {{.key}} always and {{key}} only when bare is true.
func interpolate(value string, vars map[string]string, bare bool) string {
	if value == "" || !strings.Contains(value, "{{") {
		return value
	}
	return placeholderPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := placeholderPattern.FindStringSubmatch(match)
		if len(parts) != 3 || (parts[1] == "" && !bare) {
			return match
		}
		if replacement, ok := vars[parts[2]]; ok {
			return replacement
		}
		return match
	})
}
```

`InterpolateStages` 中循环改为 `walked = interpolateValue(walked, vars, true)`，并替换 `interpolateValue`：

```go
func interpolateValue(value any, vars map[string]string, bare bool) any {
	switch typed := value.(type) {
	case string:
		return interpolate(typed, vars, bare)
	case []any:
		for i, item := range typed {
			typed[i] = interpolateValue(item, vars, bare)
		}
		return typed
	case map[string]any:
		for key, item := range typed {
			typed[key] = interpolateValue(item, vars, bare && !isRawContentKey(key))
		}
		return typed
	default:
		return value
	}
}
```

`regexp` 导入若已无引用则删除。

- [ ] **Step 4: 在 `PrepareRun` 中加入拦截**

在 `stage_indices` 过滤块之后、`stored := map[string]any{}` 之前插入：

```go
	if missing := UnresolvedPlaceholders(stages); len(missing) > 0 {
		return nil, nil, nil, model.FormatMissingVariables(missing)
	}
```

- [ ] **Step 5: 运行整个引擎包测试**

Run: `go test ./internal/modules/product_workflow/... -count=1`
Expected: 全部 PASS（含原有 `TestInterpolateLeavesUnknownKeys`、`TestInterpolateStagesSkipsConfigYAML`、`run_input_projects_test.go`、`create_version_executor_test.go`）。若有既有测试的工作流里写了未声明占位符而期望运行成功，为该测试的 `input` 补上对应 `variables`，不要放宽拦截。

- [ ] **Step 6: 提交**

```bash
git add backend/internal/modules/product_workflow/domain/engine/interpolate.go backend/internal/modules/product_workflow/domain/engine/prepare_run.go backend/internal/modules/product_workflow/domain/engine/prepare_run_test.go
git commit -m "feat(workflow): resolve template placeholders at run time and block unresolved ones"
```

---

### Task 3: 工作流接口返回 runtime_inputs，并提供跨模块端口

**Files:**
- Modify: `backend/internal/modules/product_workflow/application/query/workflow_queries.go:115-121`
- Modify: `backend/internal/modules/product_workflow/interface/http/workflow_handler.go:145-163`
- Modify: `backend/internal/platform/appctx/ports.go`
- Modify: `backend/internal/platform/appctx/container.go:53`
- Modify: `backend/internal/modules/product_workflow/module.go`

**Interfaces:**
- Consumes: Task 1 的 `engine.RuntimeInputs`、`engine.RuntimeInput`、`engine.RuntimeInputRef`。
- Produces（appctx）：

```go
var ErrWorkflowNotFound = errors.New("workflow not found")

type WorkflowRuntimeInputRef struct {
	StageIndex int    `json:"stage_index"`
	StageName  string `json:"stage_name"`
	JobName    string `json:"job_name"`
	JobType    string `json:"job_type"`
	Field      string `json:"field"`
}

type WorkflowRuntimeInput struct {
	Key      string                    `json:"key"`
	Label    string                    `json:"label"`
	Declared bool                      `json:"declared"`
	Secret   bool                      `json:"secret,omitempty"`
	Refs     []WorkflowRuntimeInputRef `json:"refs,omitempty"`
}

type WorkflowRuntimeInputReader interface {
	RuntimeInputs(ctx context.Context, productID kernel.ProductID, workflowID kernel.ProductWorkflowID, input map[string]any) ([]WorkflowRuntimeInput, error)
}
```

`Container.WorkflowRuntimeInputs WorkflowRuntimeInputReader`。

- [ ] **Step 1: 在 `ports.go` 末尾加入上面的类型与接口**（导入 `errors`；`context`、`kernel` 已导入则复用）。

- [ ] **Step 2: 在 `container.go` 的 `WorkflowRunTrigger` 下一行加字段**

```go
	WorkflowRuntimeInputs      WorkflowRuntimeInputReader
```

- [ ] **Step 3: 工作流详情响应附带清单**

`workflow_queries.go` 的 `GetProductWorkflowHandler.Handle` 返回 map 中追加一项，并导入 `.../product_workflow/domain/engine`：

```go
		"runtime_inputs": engine.RuntimeInputs(wf, nil),
```

`workflow_handler.go` 的 `workflowPayload` 返回的 `gin.H` 追加同一项（同样导入 engine）：

```go
		"runtime_inputs": engine.RuntimeInputs(wf, nil),
```

- [ ] **Step 4: 在 `module.go` 实现并注册端口**

在 `c.WorkflowRunTrigger = ...` 下一行加：

```go
	c.WorkflowRuntimeInputs = &workflowRuntimeInputPort{workflows: workflowRepo}
```

文件末尾新增（导入 `errors`、`pwfdomain ".../product_workflow/domain"`、`".../product_workflow/domain/repository"`）：

```go
type workflowRuntimeInputPort struct {
	workflows repository.ProductWorkflowRepository
}

func (p *workflowRuntimeInputPort) RuntimeInputs(
	ctx context.Context,
	productID kernel.ProductID,
	workflowID kernel.ProductWorkflowID,
	input map[string]any,
) ([]appctx.WorkflowRuntimeInput, error) {
	wf, err := p.workflows.FindWorkflowByID(ctx, workflowID)
	if errors.Is(err, pwfdomain.ErrProductWorkflowNotFound) {
		return nil, appctx.ErrWorkflowNotFound
	}
	if err != nil {
		return nil, err
	}
	if wf.ProductID != productID {
		return nil, appctx.ErrWorkflowNotFound
	}
	items := engine.RuntimeInputs(wf, input)
	out := make([]appctx.WorkflowRuntimeInput, 0, len(items))
	for _, item := range items {
		refs := make([]appctx.WorkflowRuntimeInputRef, 0, len(item.Refs))
		for _, ref := range item.Refs {
			refs = append(refs, appctx.WorkflowRuntimeInputRef(ref))
		}
		out = append(out, appctx.WorkflowRuntimeInput{
			Key: item.Key, Label: item.Label, Declared: item.Declared, Secret: item.Secret, Refs: refs,
		})
	}
	return out, nil
}
```

先确认 `persistence.NewProductWorkflowRepository` 的返回类型是 `repository.ProductWorkflowRepository`，以及 `FindWorkflowByID` 未找到时返回 `pwfdomain.ErrProductWorkflowNotFound`（读 `workflow_repo.go:107` 附近）；若返回 `gorm.ErrRecordNotFound`，把 `errors.Is` 的目标改为它。

- [ ] **Step 5: 编译与回归**

Run: `go build ./... && go test ./internal/modules/product_workflow/... -count=1`
Expected: 编译通过，测试 PASS。

- [ ] **Step 6: 提交**

```bash
git add backend/internal/platform/appctx backend/internal/modules/product_workflow
git commit -m "feat(workflow): expose runtime inputs in workflow API and appctx port"
```

---

### Task 4: 发布计划领域模型——参数、合并、缺参、启动失败、编辑权限、掩码

**Files:**
- Modify: `backend/internal/modules/product/domain/model/release_plan.go`（`ReleasePlan` 结构、`ToRunInput`、新增方法）
- Create: `backend/internal/modules/product/domain/model/release_plan_inputs.go`
- Modify: `backend/internal/modules/product/domain/model/release_plan_version.go`
- Modify: `backend/internal/modules/product/domain/model/release_plan_template.go`
- Modify: `backend/internal/modules/product/domain/model/release_plan_version_diff.go`
- Modify: `backend/internal/modules/product/domain/errors.go`
- Test: `backend/internal/modules/product/domain/model/release_plan_inputs_test.go`
- Modify test: `backend/internal/modules/product/domain/model/release_plan_test.go:55`

**Interfaces:**
- Produces:
  - `ReleasePlan.Parameters map[string]string`
  - `const MaskedParameterValue = "******"`
  - `func IsJobInputOverrideKey(key string) bool`
  - `func NormalizeParameters(params map[string]string) map[string]string`
  - `func RestoreMaskedParameters(next, prev map[string]string) map[string]string`
  - `func MaskParameters(params map[string]string, secret map[string]struct{}) map[string]string`
  - `func (spec WorkflowRefSpec) ResolveInput(planParams map[string]string, key string) string`
  - `func (spec WorkflowRefSpec) MissingInputs(planParams map[string]string, required []string) []string`
  - `func (spec WorkflowRefSpec) ToRunInput(planParams map[string]string, required []string) map[string]any`（签名变更）
  - `func (p *ReleasePlan) FailWorkflowJobStart(jobID kernel.ReleasePlanJobID, errMsg string) error`
  - `func (p *ReleasePlan) EnsureJobEditable(jobID kernel.ReleasePlanJobID, workflowRefOnly bool) error`
  - `ReleasePlanVersionSnapshot.Parameters`、`ReleasePlanTemplateSnapshot.Parameters`（json `parameters,omitempty`）
  - `domain.NewReleasePlanMissingInputsError(problems []string) error`（code `RELEASE_PLAN_MISSING_INPUTS`）
  - `domain.NewReleasePlanWorkflowMissingError(jobName string) error`（code `RELEASE_PLAN_INVALID_WORKFLOW_REF`）

- [ ] **Step 1: 写失败测试 `release_plan_inputs_test.go`**

```go
package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func executingWorkflowPlan(t *testing.T) *model.ReleasePlan {
	t.Helper()
	now := time.Now().UTC()
	plan := model.NewReleasePlan(
		kernel.NewProductID(), "v2.4 发布", "", kernel.NewMemberID(), kernel.NewMemberID(),
		now.Add(-time.Hour), now.Add(time.Hour), model.ApprovalConfig{},
	)
	ref, _ := json.Marshal(model.WorkflowRefSpec{Mode: "reference", WorkflowID: kernel.NewProductWorkflowID().String()})
	plan.Jobs = []model.ReleasePlanJob{{
		ID: kernel.NewReleasePlanJobID(), Name: "dev", Type: model.ReleasePlanJobTypeWorkflow,
		Status: model.ReleasePlanJobStatusTodo, WorkflowRef: ref,
	}}
	require.NoError(t, plan.FinishPlanning())
	require.Equal(t, model.ReleasePlanStatusExecuting, plan.Status)
	return plan
}

func TestToRunInputMergesRequiredPlanParameters(t *testing.T) {
	spec := model.WorkflowRefSpec{Parameters: map[string]string{
		"branch": "main", "release_note": "hotfix", "ticket": "OPS-9",
	}}
	plan := map[string]string{"version": "1.2.0", "ticket": "OPS-1", "unused": "x"}

	input := spec.ToRunInput(plan, []string{"version", "ticket"})

	require.Equal(t, "main", input["branch"])
	require.Equal(t, map[string]any{"release_note": "hotfix", "ticket": "OPS-9", "version": "1.2.0"}, input["variables"])
}

func TestToRunInputWithoutJobParametersStillUsesPlan(t *testing.T) {
	input := model.WorkflowRefSpec{}.ToRunInput(map[string]string{"version": "1.2.0"}, []string{"version"})
	require.Equal(t, map[string]any{"version": "1.2.0"}, input["variables"])
}

func TestMissingInputsIgnoresBlankValuesAndReservedJobKeys(t *testing.T) {
	spec := model.WorkflowRefSpec{Parameters: map[string]string{"branch": "main", "env": " "}}
	missing := spec.MissingInputs(map[string]string{"env": ""}, []string{"version", "env", "branch"})
	require.Equal(t, []string{"version", "env", "branch"}, missing)

	spec.Parameters["version"] = "1.0.0"
	require.Equal(t, []string{"env"}, spec.MissingInputs(map[string]string{"branch": "main"}, []string{"version", "env", "branch"}))
}

func TestRestoreMaskedParametersKeepsStoredSecrets(t *testing.T) {
	next := map[string]string{"token": model.MaskedParameterValue, "version": "1.2.0", "ghost": model.MaskedParameterValue}
	prev := map[string]string{"token": "s3cret"}
	require.Equal(t, map[string]string{"token": "s3cret", "version": "1.2.0"}, model.RestoreMaskedParameters(next, prev))
}

func TestMaskParametersHidesOnlySecretKeys(t *testing.T) {
	masked := model.MaskParameters(map[string]string{"token": "s3cret", "version": "1.2.0", "empty": ""},
		map[string]struct{}{"token": {}, "empty": {}})
	require.Equal(t, map[string]string{"token": model.MaskedParameterValue, "version": "1.2.0", "empty": ""}, masked)
}

func TestNormalizeParametersTrimsAndDropsBlanks(t *testing.T) {
	require.Equal(t, map[string]string{"version": "1.2.0"},
		model.NormalizeParameters(map[string]string{" version ": " 1.2.0 ", "": "x", "note": "  "}))
}

func TestFailWorkflowJobStartThenRetry(t *testing.T) {
	plan := executingWorkflowPlan(t)
	jobID := plan.Jobs[0].ID

	require.NoError(t, plan.FailWorkflowJobStart(jobID, "缺少运行时必填变量: version"))
	require.Equal(t, model.ReleasePlanJobStatusFailed, plan.Jobs[0].Status)
	var ref model.ExecutionRef
	require.NoError(t, json.Unmarshal(plan.Jobs[0].ExecutionRef, &ref))
	require.Equal(t, "缺少运行时必填变量: version", ref.ErrorMessage)

	require.NoError(t, plan.RetryWorkflowJob(jobID))
	require.Equal(t, model.ReleasePlanJobStatusTodo, plan.Jobs[0].Status)
}

func TestEnsureJobEditable(t *testing.T) {
	plan := executingWorkflowPlan(t)
	jobID := plan.Jobs[0].ID

	require.Error(t, plan.EnsureJobEditable(jobID, true))
	require.NoError(t, plan.FailWorkflowJobStart(jobID, "boom"))
	require.NoError(t, plan.EnsureJobEditable(jobID, true))
	require.Error(t, plan.EnsureJobEditable(jobID, false))
	require.Error(t, plan.EnsureJobEditable(kernel.NewReleasePlanJobID(), true))
}

func TestVersionDiffReportsParameterPresenceOnly(t *testing.T) {
	before := model.ReleasePlanVersionSnapshot{Name: "p", Parameters: map[string]string{"version": "1.0.0", "token": "a"}}
	after := model.ReleasePlanVersionSnapshot{Name: "p", Parameters: map[string]string{"version": "1.1.0", "ticket": "OPS-1"}}

	resp := model.DiffReleasePlanVersionSnapshots("plan-1", 2, 1, before, after)

	raw, _ := json.Marshal(resp)
	require.NotContains(t, string(raw), "1.1.0")
	require.NotContains(t, string(raw), "OPS-1")
	labels := []string{}
	for _, group := range resp.Groups {
		for _, change := range group.Changes {
			labels = append(labels, change.Label)
		}
	}
	require.Equal(t, []string{"发布参数 · ticket", "发布参数 · token", "发布参数 · version"}, labels)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/product/domain/model/ -count=1`
Expected: 编译失败（`MaskedParameterValue`、`FailWorkflowJobStart`、`EnsureJobEditable`、`Parameters` 字段等未定义；`ToRunInput` 参数个数不符）。

- [ ] **Step 3: 新建 `release_plan_inputs.go`**

```go
package model

import (
	"strings"
)

const MaskedParameterValue = "******"

var reservedJobParameterKeys = map[string]struct{}{"branch": {}, "environment_id": {}}

// IsJobInputOverrideKey reports whether a job parameter key carries a run variable override.
func IsJobInputOverrideKey(key string) bool {
	if key == "" || strings.HasPrefix(key, "_") {
		return false
	}
	_, reserved := reservedJobParameterKeys[key]
	return !reserved
}

func NormalizeParameters(params map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range params {
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

// RestoreMaskedParameters keeps the stored value wherever the client echoed the mask back.
func RestoreMaskedParameters(next, prev map[string]string) map[string]string {
	out := make(map[string]string, len(next))
	for key, value := range next {
		if value == MaskedParameterValue {
			old, ok := prev[key]
			if !ok {
				continue
			}
			value = old
		}
		out[key] = value
	}
	return out
}

func MaskParameters(params map[string]string, secret map[string]struct{}) map[string]string {
	out := make(map[string]string, len(params))
	for key, value := range params {
		if _, ok := secret[key]; ok && value != "" {
			value = MaskedParameterValue
		}
		out[key] = value
	}
	return out
}

// ResolveInput returns the value a job runs with: its own override first, then the plan parameter.
func (spec WorkflowRefSpec) ResolveInput(planParams map[string]string, key string) string {
	if IsJobInputOverrideKey(key) {
		if value := strings.TrimSpace(spec.Parameters[key]); value != "" {
			return value
		}
	}
	return strings.TrimSpace(planParams[key])
}

func (spec WorkflowRefSpec) MissingInputs(planParams map[string]string, required []string) []string {
	missing := []string{}
	for _, key := range required {
		if spec.ResolveInput(planParams, key) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}
```

- [ ] **Step 4: 修改 `release_plan.go`**

在 `ReleasePlan` 结构体 `HookState` 下一行加：

```go
	Parameters          map[string]string
```

把 `ToRunInput` 整体替换为：

```go
// ToRunInput builds the workflow run input; required lists the inputs the workflow needs,
// which are filled from planParams when the job does not override them.
func (spec WorkflowRefSpec) ToRunInput(planParams map[string]string, required []string) map[string]any {
	input := map[string]any{}
	params := spec.Parameters
	if branch := strings.TrimSpace(params["branch"]); branch != "" {
		input["branch"] = branch
	}
	if raw := strings.TrimSpace(params["_environment_overrides"]); raw != "" {
		var overrides map[string]string
		if err := json.Unmarshal([]byte(raw), &overrides); err == nil && len(overrides) > 0 {
			envMap := make(map[string]any, len(overrides))
			for jobName, envID := range overrides {
				if strings.TrimSpace(envID) != "" {
					envMap[jobName] = envID
				}
			}
			if len(envMap) > 0 {
				input["environment_overrides"] = envMap
			}
		}
	}
	if raw := strings.TrimSpace(params["_project_ids"]); raw != "" {
		var projectIDs []string
		if err := json.Unmarshal([]byte(raw), &projectIDs); err == nil {
			items := make([]any, 0, len(projectIDs))
			for _, projectID := range projectIDs {
				if id := strings.TrimSpace(projectID); id != "" {
					items = append(items, id)
				}
			}
			input["project_ids"] = items
		}
	}
	if overrides := parseStringMapParam(params["_branch_overrides"]); len(overrides) > 0 {
		input["branch_overrides"] = overrides
	}
	if overrides := parseStringMapParam(params["_ref_type_overrides"]); len(overrides) > 0 {
		input["ref_type_overrides"] = overrides
	}
	variables := map[string]any{}
	for key, value := range params {
		if !IsJobInputOverrideKey(key) {
			continue
		}
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			variables[key] = trimmed
		}
	}
	for _, key := range required {
		if _, ok := variables[key]; ok {
			continue
		}
		if value := strings.TrimSpace(planParams[key]); value != "" {
			variables[key] = value
		}
	}
	if len(variables) > 0 {
		input["variables"] = variables
	}
	return input
}
```

在 `RetryWorkflowJob` 之后新增：

```go
// FailWorkflowJobStart records that the current workflow job could not start, so it can be fixed and retried.
func (p *ReleasePlan) FailWorkflowJobStart(jobID kernel.ReleasePlanJobID, errMsg string) error {
	idx := p.jobIndexByID(jobID)
	if idx < 0 {
		return domain.ErrReleasePlanJobNotFound
	}
	if err := p.ensureCurrentJob(jobID); err != nil {
		return err
	}
	job := &p.Jobs[idx]
	if job.Type != ReleasePlanJobTypeWorkflow || job.Status != ReleasePlanJobStatusTodo {
		return domain.ErrInvalidReleasePlanJobTransition
	}
	ref, _ := json.Marshal(ExecutionRef{ErrorMessage: errMsg})
	job.ExecutionRef = ref
	job.Status = ReleasePlanJobStatusFailed
	now := time.Now().UTC()
	job.UpdatedAt = now
	p.UpdatedAt = now
	return nil
}

// EnsureJobEditable allows any edit while planning; once executing, only a failed
// workflow job's workflow reference (its run inputs) may change.
func (p *ReleasePlan) EnsureJobEditable(jobID kernel.ReleasePlanJobID, workflowRefOnly bool) error {
	idx := p.jobIndexByID(jobID)
	if idx < 0 {
		return domain.ErrReleasePlanJobNotFound
	}
	switch p.Status {
	case ReleasePlanStatusPlanning:
		return nil
	case ReleasePlanStatusExecuting:
		job := p.Jobs[idx]
		if workflowRefOnly && job.Type == ReleasePlanJobTypeWorkflow && job.Status == ReleasePlanJobStatusFailed {
			return nil
		}
	}
	return domain.ErrInvalidReleasePlanTransition
}
```

- [ ] **Step 5: 快照携带参数**

`release_plan_version.go`：`ReleasePlanVersionSnapshot` 在 `ApprovalConfig` 下加 `Parameters map[string]string \`json:"parameters,omitempty"\``；`VersionSnapshotFromPlan` 构造时加 `Parameters: maps.Clone(plan.Parameters),`（导入 `maps`）。

`release_plan_template.go`：`ReleasePlanTemplateSnapshot` 同样加 `Parameters` 字段；`SnapshotFromPlan` 加 `Parameters: maps.Clone(plan.Parameters),`。

- [ ] **Step 6: 版本差异只报告参数是否填写**

`release_plan_version_diff.go` 的 `diffReleasePlanMetadata` 在 `return changes` 前插入（导入 `sort`）：

```go
	for _, key := range unionParameterKeys(before.Parameters, after.Parameters) {
		b, a := before.Parameters[key], after.Parameters[key]
		if b == a {
			continue
		}
		changes = append(changes, ReleasePlanVersionDiffChange{
			ChangeType: "field_changed", Path: "parameters." + key, Label: "发布参数 · " + key,
			Before: parameterPresence(b, ""), After: parameterPresence(a, b),
		})
	}
```

文件末尾新增：

```go
func unionParameterKeys(a, b map[string]string) []string {
	seen := map[string]struct{}{}
	keys := []string{}
	for _, m := range []map[string]string{a, b} {
		for key := range m {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// parameterPresence describes a parameter value without exposing it in version history.
func parameterPresence(value, previous string) string {
	switch {
	case value == "":
		return "未填写"
	case previous != "":
		return "已修改"
	default:
		return "已填写"
	}
}
```

- [ ] **Step 7: 领域错误**

`product/domain/errors.go` 末尾新增（导入 `fmt`、`strings`）：

```go
func NewReleasePlanMissingInputsError(problems []string) error {
	return kernel.NewDomainError("RELEASE_PLAN_MISSING_INPUTS", "发布项缺少运行参数："+strings.Join(problems, "；"))
}

func NewReleasePlanWorkflowMissingError(jobName string) error {
	return kernel.NewDomainError("RELEASE_PLAN_INVALID_WORKFLOW_REF", fmt.Sprintf("发布项「%s」关联的工作流不存在", jobName))
}
```

- [ ] **Step 8: 更新既有测试调用**

`release_plan_test.go:55` 改为 `input := spec.ToRunInput(nil, nil)`。

- [ ] **Step 9: 运行测试**

Run: `go test ./internal/modules/product/domain/... -count=1`
Expected: PASS。`go build ./...` 会在 `release_plan_executor.go` 报 `ToRunInput` 参数不足，由 Task 6 修复；本任务只要求 domain 包测试通过。

- [ ] **Step 10: 提交**

```bash
git add backend/internal/modules/product/domain
git commit -m "feat(release-plan): plan parameters, input merge and start-failure in domain model"
```

---

### Task 5: 持久化与复制 / 模板携带参数

**Files:**
- Create: `backend/migrations/000068_release_plan_parameters.up.sql`
- Create: `backend/migrations/000068_release_plan_parameters.down.sql`
- Modify: `backend/internal/modules/product/infrastructure/persistence/release_plan_repo.go`（`releasePlanModel`、`UpdatePlan`、`toReleasePlanModel`、`rowToReleasePlan`）
- Modify: `backend/internal/modules/product/application/command/release_plan_commands_ext.go`（复制、从模板创建）

**Interfaces:**
- Consumes: Task 4 的 `ReleasePlan.Parameters`、`ReleasePlanTemplateSnapshot.Parameters`。

- [ ] **Step 1: 迁移文件**

`000068_release_plan_parameters.up.sql`：

```sql
ALTER TABLE product.release_plans
  ADD COLUMN IF NOT EXISTS parameters JSONB NOT NULL DEFAULT '{}';
```

`000068_release_plan_parameters.down.sql`：

```sql
ALTER TABLE product.release_plans
  DROP COLUMN IF EXISTS parameters;
```

- [ ] **Step 2: 仓储映射**

`releasePlanModel` 在 `HookState` 下加：

```go
	Parameters          string  `gorm:"type:jsonb;not null;default:'{}'"`
```

`UpdatePlan` 的 `updates` map 加 `"parameters": row.Parameters,`。

`toReleasePlanModel` 在 marshal `hookSettings` 后加：

```go
	params := plan.Parameters
	if params == nil {
		params = map[string]string{}
	}
	parameters, err := json.Marshal(params)
	if err != nil {
		return releasePlanModel{}, err
	}
```

并在 `row := releasePlanModel{...}` 中加 `Parameters: string(parameters),`。

`rowToReleasePlan` 构造 `plan` 之后加：

```go
	if row.Parameters != "" {
		_ = json.Unmarshal([]byte(row.Parameters), &plan.Parameters)
	}
```

- [ ] **Step 3: 复制与模板创建携带参数**

`CopyReleasePlanHandler.Handle` 在 `clone.ScheduleExecuteTime = src.ScheduleExecuteTime` 后加 `clone.Parameters = maps.Clone(src.Parameters)`。

`CreateReleasePlanFromTemplateHandler.Handle` 在 `plan.SprintID = input.SprintID` 后加 `plan.Parameters = maps.Clone(tpl.Snapshot.Parameters)`。两处导入 `maps`。

- [ ] **Step 4: 编译检查**

Run: `go vet ./internal/modules/product/infrastructure/... ./internal/modules/product/domain/...`
Expected: 无错误。（`application/service` 的编译错误留给 Task 6。）

- [ ] **Step 5: 提交**

```bash
git add backend/migrations/000068_release_plan_parameters.up.sql backend/migrations/000068_release_plan_parameters.down.sql backend/internal/modules/product/infrastructure backend/internal/modules/product/application/command/release_plan_commands_ext.go
git commit -m "feat(release-plan): persist plan parameters"
```

---

### Task 6: 应用层——完成规划校验、执行合并、失败记录、更新与查询

**Files:**
- Create: `backend/internal/modules/product/application/service/release_plan_inputs.go`
- Test: `backend/internal/modules/product/application/service/release_plan_inputs_test.go`
- Modify: `backend/internal/modules/product/application/service/release_plan_executor.go`
- Modify: `backend/internal/modules/product/application/command/release_plan_commands.go`（`UpdateReleasePlanInput`、`UpdateReleasePlanHandler`、`FinishReleasePlanPlanningHandler`、`UpdateReleasePlanJobHandler`）
- Modify: `backend/internal/modules/product/application/query/release_plan_queries.go`
- Modify: `backend/internal/modules/product/application/query/release_plan_queries_ext.go`
- Modify: `backend/internal/modules/product/interface/http/release_plan_handler.go:511-546`
- Modify: `backend/internal/platform/httpx/errors.go`
- Modify: `backend/internal/modules/product/module.go`

**Interfaces:**
- Consumes: Task 3 的 `appctx.WorkflowRuntimeInputReader`、`appctx.ErrWorkflowNotFound`；Task 4 的模型方法与错误。
- Produces:
  - `func ReleasePlanJobInputs(ctx context.Context, plan *model.ReleasePlan, job *model.ReleasePlanJob, reader appctx.WorkflowRuntimeInputReader) (model.WorkflowRefSpec, []appctx.WorkflowRuntimeInput, error)`
  - `func RuntimeInputKeys(items []appctx.WorkflowRuntimeInput) []string`
  - `func CheckReleasePlanInputs(ctx context.Context, plan *model.ReleasePlan, reader appctx.WorkflowRuntimeInputReader) error`
  - `(*ReleasePlanExecutor).UseRuntimeInputs(reader)`、`(*FinishReleasePlanPlanningHandler).UseRuntimeInputs(reader)`、`(*GetReleasePlanHandler).UseRuntimeInputs(reader)`、`(*GetReleasePlanHandlerWithSprint).UseRuntimeInputs(reader)`
  - 详情响应：计划 `parameters`（掩码后），每个工作流发布项 `runtime_inputs`。

- [ ] **Step 1: 写失败测试 `release_plan_inputs_test.go`**

```go
package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type fakeInputReader struct {
	byWorkflow map[string][]appctx.WorkflowRuntimeInput
	err        error
}

func (f fakeInputReader) RuntimeInputs(_ context.Context, _ kernel.ProductID, id kernel.ProductWorkflowID, _ map[string]any) ([]appctx.WorkflowRuntimeInput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byWorkflow[id.String()], nil
}

func planningPlanWithJob(t *testing.T, name string, workflowID kernel.ProductWorkflowID, params map[string]string) *model.ReleasePlan {
	t.Helper()
	now := time.Now().UTC()
	plan := model.NewReleasePlan(kernel.NewProductID(), "p", "", kernel.NewMemberID(), kernel.NewMemberID(), now, now.Add(time.Hour), model.ApprovalConfig{})
	ref, _ := json.Marshal(model.WorkflowRefSpec{Mode: "reference", WorkflowID: workflowID.String(), Parameters: params})
	plan.Jobs = []model.ReleasePlanJob{{
		ID: kernel.NewReleasePlanJobID(), Name: name, Type: model.ReleasePlanJobTypeWorkflow,
		Status: model.ReleasePlanJobStatusTodo, WorkflowRef: ref,
	}}
	return plan
}

func TestCheckReleasePlanInputsReportsMissingKeys(t *testing.T) {
	wfID := kernel.NewProductWorkflowID()
	reader := fakeInputReader{byWorkflow: map[string][]appctx.WorkflowRuntimeInput{
		wfID.String(): {{Key: "version"}, {Key: "ticket"}},
	}}
	plan := planningPlanWithJob(t, "dev", wfID, map[string]string{"ticket": "OPS-1"})

	err := service.CheckReleasePlanInputs(context.Background(), plan, reader)

	require.Error(t, err)
	de, ok := kernel.IsDomainError(err)
	require.True(t, ok)
	require.Equal(t, "RELEASE_PLAN_MISSING_INPUTS", de.Code())
	require.Contains(t, err.Error(), "「dev」缺少 version")
	require.NotContains(t, err.Error(), "ticket")
}

func TestCheckReleasePlanInputsPassesWithPlanParameters(t *testing.T) {
	wfID := kernel.NewProductWorkflowID()
	reader := fakeInputReader{byWorkflow: map[string][]appctx.WorkflowRuntimeInput{wfID.String(): {{Key: "version"}}}}
	plan := planningPlanWithJob(t, "dev", wfID, nil)
	plan.Parameters = map[string]string{"version": "1.2.0"}

	require.NoError(t, service.CheckReleasePlanInputs(context.Background(), plan, reader))
}

func TestCheckReleasePlanInputsSkipsSkippedJobs(t *testing.T) {
	wfID := kernel.NewProductWorkflowID()
	reader := fakeInputReader{byWorkflow: map[string][]appctx.WorkflowRuntimeInput{wfID.String(): {{Key: "version"}}}}
	plan := planningPlanWithJob(t, "dev", wfID, nil)
	plan.Jobs[0].Status = model.ReleasePlanJobStatusSkipped

	require.NoError(t, service.CheckReleasePlanInputs(context.Background(), plan, reader))
}

func TestCheckReleasePlanInputsNamesMissingWorkflow(t *testing.T) {
	plan := planningPlanWithJob(t, "dev", kernel.NewProductWorkflowID(), nil)

	err := service.CheckReleasePlanInputs(context.Background(), plan, fakeInputReader{err: appctx.ErrWorkflowNotFound})

	require.EqualError(t, err, "发布项「dev」关联的工作流不存在")
}

func TestCheckReleasePlanInputsWithoutReaderIsNoop(t *testing.T) {
	plan := planningPlanWithJob(t, "dev", kernel.NewProductWorkflowID(), nil)
	require.NoError(t, service.CheckReleasePlanInputs(context.Background(), plan, nil))
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/product/application/service/ -run TestCheckReleasePlanInputs -count=1`
Expected: 编译失败，`undefined: service.CheckReleasePlanInputs`（以及 executor 中 `ToRunInput` 参数不足）。

- [ ] **Step 3: 新建 `release_plan_inputs.go`**

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

// ReleasePlanJobInputs resolves the runtime inputs a workflow job still needs after its own overrides apply.
func ReleasePlanJobInputs(
	ctx context.Context,
	plan *model.ReleasePlan,
	job *model.ReleasePlanJob,
	reader appctx.WorkflowRuntimeInputReader,
) (model.WorkflowRefSpec, []appctx.WorkflowRuntimeInput, error) {
	spec, err := plan.ParseWorkflowRef(job)
	if err != nil {
		return spec, nil, err
	}
	if reader == nil {
		return spec, nil, nil
	}
	workflowID, err := kernel.ParseProductWorkflowID(spec.WorkflowID)
	if err != nil {
		return spec, nil, domain.ErrInvalidWorkflowRef
	}
	inputs, err := reader.RuntimeInputs(ctx, plan.ProductID, workflowID, spec.ToRunInput(nil, nil))
	return spec, inputs, err
}

func RuntimeInputKeys(items []appctx.WorkflowRuntimeInput) []string {
	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, item.Key)
	}
	return keys
}

// CheckReleasePlanInputs fails when any pending workflow job would start without a required input.
func CheckReleasePlanInputs(ctx context.Context, plan *model.ReleasePlan, reader appctx.WorkflowRuntimeInputReader) error {
	if reader == nil {
		return nil
	}
	problems := []string{}
	for i := range plan.Jobs {
		job := &plan.Jobs[i]
		if job.Type != model.ReleasePlanJobTypeWorkflow || job.Status == model.ReleasePlanJobStatusSkipped {
			continue
		}
		spec, inputs, err := ReleasePlanJobInputs(ctx, plan, job, reader)
		if errors.Is(err, appctx.ErrWorkflowNotFound) {
			return domain.NewReleasePlanWorkflowMissingError(job.Name)
		}
		if err != nil {
			return err
		}
		if missing := spec.MissingInputs(plan.Parameters, RuntimeInputKeys(inputs)); len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("「%s」缺少 %s", job.Name, strings.Join(missing, "、")))
		}
	}
	if len(problems) > 0 {
		return domain.NewReleasePlanMissingInputsError(problems)
	}
	return nil
}
```

- [ ] **Step 4: 执行器按所需键合并，启动失败记为发布项失败**

`ReleasePlanExecutor` 结构体加字段 `inputs appctx.WorkflowRuntimeInputReader`，并新增：

```go
func (e *ReleasePlanExecutor) UseRuntimeInputs(reader appctx.WorkflowRuntimeInputReader) {
	e.inputs = reader
}

func (e *ReleasePlanExecutor) failJobStart(ctx context.Context, plan *model.ReleasePlan, job *model.ReleasePlanJob, actor kernel.MemberID, msg string) error {
	if err := plan.FailWorkflowJobStart(job.ID, msg); err != nil {
		return err
	}
	if err := e.plans.UpdateJob(ctx, job); err != nil {
		return err
	}
	if err := e.plans.UpdatePlan(ctx, plan); err != nil {
		return err
	}
	return e.audit(ctx, plan.ID, actor, "job_failed", map[string]any{"job_id": job.ID.String(), "error_message": msg})
}
```

`Advance` 中从 `spec, err := plan.ParseWorkflowRef(job)` 到 `if err != nil { return err }`（启动运行之后那段）替换为（导入 `errors`）：

```go
	spec, inputs, err := ReleasePlanJobInputs(ctx, plan, job, e.inputs)
	if errors.Is(err, appctx.ErrWorkflowNotFound) {
		return e.failJobStart(ctx, plan, job, actor, domain.NewReleasePlanWorkflowMissingError(job.Name).Error())
	}
	if err != nil {
		return err
	}
	workflowID, err := kernel.ParseProductWorkflowID(spec.WorkflowID)
	if err != nil {
		return domain.ErrInvalidWorkflowRef
	}
	runInput := spec.ToRunInput(plan.Parameters, RuntimeInputKeys(inputs))
	var runID kernel.ProductWorkflowRunID
	if len(runInput) == 0 {
		runID, err = e.trigger.StartForProduct(ctx, plan.ProductID, workflowID)
	} else {
		runID, err = e.trigger.StartForProductWithInput(ctx, plan.ProductID, workflowID, actor, runInput)
	}
	if err != nil {
		if _, ok := kernel.IsDomainError(err); ok {
			return e.failJobStart(ctx, plan, job, actor, err.Error())
		}
		return err
	}
```

之后的 `MarkWorkflowJobRunning` 等保持不变。

- [ ] **Step 5: 完成规划前校验**

`FinishReleasePlanPlanningHandler` 加字段 `inputs appctx.WorkflowRuntimeInputReader` 与方法：

```go
func (h *FinishReleasePlanPlanningHandler) UseRuntimeInputs(reader appctx.WorkflowRuntimeInputReader) {
	h.inputs = reader
}
```

`Handle` 中在 `if plan.Status != model.ReleasePlanStatusPlanning {...}` 之后、`MaterializeWorkflowSnapshots` 之前插入：

```go
	if err := service.CheckReleasePlanInputs(ctx, plan, h.inputs); err != nil {
		return nil, err
	}
```

文件导入 `.../platform/appctx`。

- [ ] **Step 6: 更新计划参数**

`UpdateReleasePlanInput` 加字段 `Parameters *map[string]string`。`UpdateReleasePlanHandler.Handle` 在 `HookSettings` 赋值后加：

```go
	if input.Parameters != nil {
		plan.Parameters = model.NormalizeParameters(model.RestoreMaskedParameters(*input.Parameters, plan.Parameters))
	}
```

`release_plan_handler.go` 的 `updateReleasePlanRequest` 加 `Parameters *map[string]string \`json:"parameters"\``，构造 `command.UpdateReleasePlanInput` 时加 `Parameters: req.Parameters,`。

- [ ] **Step 7: 发布项更新——执行中允许修改失败项运行参数，并还原掩码**

`UpdateReleasePlanJobHandler.Handle` 删除 `if plan.Status != model.ReleasePlanStatusPlanning { return nil, domain.ErrInvalidReleasePlanTransition }`，在找到 `job` 且判空之后插入：

```go
	workflowRefOnly := input.HasWorkflow && input.Name == nil && input.OwnerID == nil &&
		input.TextContent == nil && input.TextFormat == nil && input.SortOrder == nil
	if err := plan.EnsureJobEditable(jobID, workflowRefOnly); err != nil {
		return nil, err
	}
```

把 `if input.HasWorkflow { job.WorkflowRef = input.WorkflowRef }` 改为：

```go
	if input.HasWorkflow {
		ref, err := mergeWorkflowRefUpdate(plan, job, input.WorkflowRef)
		if err != nil {
			return nil, err
		}
		job.WorkflowRef = ref
	}
```

文件末尾新增：

```go
// mergeWorkflowRefUpdate restores masked secret parameters and, outside planning, keeps the job on its workflow.
func mergeWorkflowRefUpdate(plan *model.ReleasePlan, job *model.ReleasePlanJob, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return raw, nil
	}
	var next model.WorkflowRefSpec
	if err := json.Unmarshal(raw, &next); err != nil {
		return nil, domain.ErrInvalidWorkflowRef
	}
	var prev model.WorkflowRefSpec
	if len(job.WorkflowRef) > 0 {
		_ = json.Unmarshal(job.WorkflowRef, &prev)
	}
	if plan.Status != model.ReleasePlanStatusPlanning && next.WorkflowID != prev.WorkflowID {
		return nil, domain.ErrInvalidReleasePlanTransition
	}
	next.Parameters = model.RestoreMaskedParameters(next.Parameters, prev.Parameters)
	return json.Marshal(next)
}
```

- [ ] **Step 8: 详情查询附带 runtime_inputs 并掩码**

`release_plan_queries.go`：

`GetReleasePlanHandler` 加字段 `inputs appctx.WorkflowRuntimeInputReader` 与 `UseRuntimeInputs` 方法（同 Step 5 写法）。`Handle` 中 `out := releasePlanDetail(plan)` 之后加：

```go
	h.attachRuntimeInputs(ctx, plan, out)
```

`releasePlanDetail` 在 `out["jobs"] = jobs` 后加：

```go
	out["parameters"] = plan.Parameters
	if plan.Parameters == nil {
		out["parameters"] = map[string]string{}
	}
```

新增（导入 `.../application/service`、`.../platform/appctx`）：

```go
func (h *GetReleasePlanHandler) attachRuntimeInputs(ctx context.Context, plan *model.ReleasePlan, out map[string]any) {
	if h.inputs == nil {
		return
	}
	jobs, _ := out["jobs"].([]map[string]any)
	secret := map[string]struct{}{}
	for i := range plan.Jobs {
		job := &plan.Jobs[i]
		if job.Type != model.ReleasePlanJobTypeWorkflow || i >= len(jobs) {
			continue
		}
		_, inputs, err := service.ReleasePlanJobInputs(ctx, plan, job, h.inputs)
		if err != nil {
			continue
		}
		jobs[i]["runtime_inputs"] = inputs
		jobSecret := map[string]struct{}{}
		for _, input := range inputs {
			if input.Secret {
				secret[input.Key] = struct{}{}
				jobSecret[input.Key] = struct{}{}
			}
		}
		maskJobParameters(jobs[i], jobSecret)
	}
	out["parameters"] = model.MaskParameters(plan.Parameters, secret)
}

func maskJobParameters(item map[string]any, secret map[string]struct{}) {
	ref, ok := item["workflow_ref"].(map[string]any)
	if !ok {
		return
	}
	params, ok := ref["parameters"].(map[string]any)
	if !ok {
		return
	}
	for key := range secret {
		if value, ok := params[key].(string); ok && value != "" {
			params[key] = model.MaskedParameterValue
		}
	}
}
```

`releasePlanDetail` 生成的 `jobs` 与 `plan.Jobs` 顺序一致（逐个 append），依赖此对齐；如果实现时发现顺序不一致，改为按 `item["id"]` 匹配。

`release_plan_queries_ext.go` 新增：

```go
func (h *GetReleasePlanHandlerWithSprint) UseRuntimeInputs(reader appctx.WorkflowRuntimeInputReader) {
	h.base.UseRuntimeInputs(reader)
}
```

先确认 query 包导入 service 包不会产生循环依赖（`grep -r "application/query" backend/internal/modules/product/application/service` 应无结果）。如有循环，把 `ReleasePlanJobInputs` 和 `RuntimeInputKeys` 移到新包 `product/application/inputs`，service 与 query 都从那里导入。

- [ ] **Step 9: 422 映射**

`httpx/errors.go` 的 `switch de.Code()` 中，在 `case "PRODUCT_WORKFLOW_VARIABLE_REQUIRED":` 之前加：

```go
		case "RELEASE_PLAN_MISSING_INPUTS":
			status = http.StatusUnprocessableEntity
```

- [ ] **Step 10: 模块装配**

`product/module.go`：在 `releasePlanExecutor := ...` 后加 `releasePlanExecutor.UseRuntimeInputs(c.WorkflowRuntimeInputs)`。把 `NewReleasePlanHandlers(...)` 参数里内联构造的两个处理器提到前面并注入：

```go
	finishPlanning := command.NewFinishReleasePlanPlanningHandler(repo, releasePlanRepo, releasePlanExecutor, releasePlanNotifier, releasePlanHookNotifier, workflowSnapshotReader)
	finishPlanning.UseRuntimeInputs(c.WorkflowRuntimeInputs)
	getReleasePlan := query.NewGetReleasePlanHandlerWithSprint(repo, releasePlanRepo, sprintRepo)
	getReleasePlan.UseRuntimeInputs(c.WorkflowRuntimeInputs)
```

并在参数列表原位置替换为 `finishPlanning`、`getReleasePlan`。

- [ ] **Step 11: 运行测试与编译**

Run: `go build ./... && go test ./internal/modules/product/... ./internal/modules/product_workflow/... ./internal/platform/... -count=1`
Expected: 编译通过，全部 PASS（含 Task 6 的 5 个新测试）。

- [ ] **Step 12: 重启后端并冒烟**

Run（仓库根目录）：`.\scripts\dev-restart.ps1 -Backend`
Expected: 后端在 :8080 启动；日志中 product 模块迁移无报错；`product.release_plans` 已有 `parameters` 列（GORM AutoMigrate 会补列）。

- [ ] **Step 13: 提交**

```bash
git add backend/internal/modules/product backend/internal/platform/httpx/errors.go
git commit -m "feat(release-plan): validate runtime inputs at finish planning and merge plan parameters"
```

---

### Task 7: 前端契约类型

**Files:**
- Modify: `fronted/packages/contracts/src/product/product-workflow.ts:64-81`
- Modify: `fronted/packages/contracts/src/product/release-plan.ts`

**Interfaces:**
- Produces: `WorkflowRuntimeInputRefSchema` / `WorkflowRuntimeInputRef`、`WorkflowRuntimeInputSchema` / `WorkflowRuntimeInput`；`ProductWorkflowVM.runtime_inputs?`、`ReleasePlanJobVM.runtime_inputs?`、`ReleasePlanVM.parameters?`、`UpdateReleasePlanCommand.parameters?`。

- [ ] **Step 1: `product-workflow.ts` 在 `ProductWorkflowVMSchema` 之前加入**

```ts
export const WorkflowRuntimeInputRefSchema = z.object({
  stage_index: z.number().int(),
  stage_name: z.string(),
  job_name: z.string(),
  job_type: z.string(),
  field: z.string(),
});

export type WorkflowRuntimeInputRef = z.infer<typeof WorkflowRuntimeInputRefSchema>;

export const WorkflowRuntimeInputSchema = z.object({
  key: z.string(),
  label: z.string(),
  declared: z.boolean(),
  secret: z.boolean().optional(),
  refs: z.array(WorkflowRuntimeInputRefSchema).optional(),
});

export type WorkflowRuntimeInput = z.infer<typeof WorkflowRuntimeInputSchema>;
```

`ProductWorkflowVMSchema` 在 `advanced` 后加 `runtime_inputs: z.array(WorkflowRuntimeInputSchema).optional(),`。

- [ ] **Step 2: `release-plan.ts`**

顶部加 `import { WorkflowRuntimeInputSchema } from "./product-workflow";`。`ReleasePlanJobVMSchema` 加 `runtime_inputs: z.array(WorkflowRuntimeInputSchema).optional(),`；`ReleasePlanVMSchema` 在 `hook_state` 后加 `parameters: z.record(z.string()).optional(),`；`UpdateReleasePlanCommandSchema` 加 `parameters: z.record(z.string()).optional(),`。

- [ ] **Step 3: 类型检查**

Run（`fronted/`）：`pnpm --filter @sparkops/web typecheck`
Expected: 只出现修改前已有的错误（`StartWorkflowRunModal.tsx`、`ReleasePlanLinkSprintModal.tsx` 等）；没有新增错误。先在改动前跑一次并记下错误数作对照。

- [ ] **Step 4: 提交**

```bash
git add fronted/packages/contracts/src/product/product-workflow.ts fronted/packages/contracts/src/product/release-plan.ts
git commit -m "feat(contracts): runtime inputs and release plan parameters"
```

---

### Task 8: 前端纯函数——继承、缺参、运行弹窗过滤、参数合并

**Files:**
- Create: `fronted/apps/web/src/modules/product/components/release-plan-runtime-inputs.ts`
- Test: `fronted/apps/web/src/modules/product/components/release-plan-runtime-inputs.test.ts`
- Modify: `fronted/apps/web/src/modules/product/components/release-plan-workflow-params.ts:260-305`
- Modify test: `fronted/apps/web/src/modules/product/components/release-plan-workflow-params.test.ts`

**Interfaces:**
- Consumes: Task 7 的类型。
- Produces:
  - `MASKED_PARAMETER_VALUE = "******"`
  - `canOverrideInJob(key: string): boolean`
  - `formatInputRef(ref: WorkflowRuntimeInputRef): string` → `阶段3 · archive`
  - `interface PlanInputRow { key: string; label: string; secret: boolean; usedBy: string[] }`，`planInputRows(jobs: ReleasePlanJobVM[]): PlanInputRow[]`
  - `type JobInputSource = "job" | "plan" | "missing"`；`interface JobInputState { key; label; secret: boolean; source: JobInputSource; value: string; overridable: boolean }`；`jobInputStates(job, planParams): JobInputState[]`
  - `interface JobMissingInputs { jobId: string; jobName: string; keys: string[] }`；`planMissingInputs(plan): JobMissingInputs[]`；`formatMissingInputs(items): string`
  - `runModalExtraInputs(inputs?: WorkflowRuntimeInput[]): WorkflowRuntimeInput[]`
  - `WorkflowJobParamsPatch.variables?: Record<string, string | null>`

- [ ] **Step 1: 写失败测试 `release-plan-runtime-inputs.test.ts`**

```ts
import { describe, expect, it } from "vitest";
import type { ReleasePlanJobVM } from "@sparkops/contracts/product";
import {
  canOverrideInJob,
  formatMissingInputs,
  jobInputStates,
  planInputRows,
  planMissingInputs,
  runModalExtraInputs,
} from "./release-plan-runtime-inputs";

const ref = (stage_index: number, job_name: string, field = "version_tag") => ({
  stage_index,
  stage_name: `阶段${stage_index + 1}`,
  job_name,
  job_type: "sparkops-create-version",
  field,
});

function workflowJob(partial: Partial<ReleasePlanJobVM>): ReleasePlanJobVM {
  return { id: "j1", name: "dev", type: "workflow", status: "todo", ...partial };
}

describe("release plan runtime inputs", () => {
  it("treats reserved and underscore keys as plan-only", () => {
    expect(canOverrideInJob("version")).toBe(true);
    expect(canOverrideInJob("branch")).toBe(false);
    expect(canOverrideInJob("environment_id")).toBe(false);
    expect(canOverrideInJob("_project_ids")).toBe(false);
  });

  it("unions inputs across workflow jobs with usage labels", () => {
    const rows = planInputRows([
      workflowJob({ id: "a", name: "dev", runtime_inputs: [{ key: "version", label: "version", declared: false, refs: [ref(2, "archive")] }] }),
      workflowJob({ id: "b", name: "prod", runtime_inputs: [{ key: "version", label: "版本", declared: true, secret: true }] }),
      workflowJob({ id: "c", name: "skip", status: "skipped", runtime_inputs: [{ key: "ticket", label: "ticket", declared: false }] }),
    ]);
    expect(rows).toEqual([
      { key: "version", label: "version", secret: true, usedBy: ["dev · 阶段3 · archive", "prod · 运行时变量"] },
    ]);
  });

  it("resolves job override, plan inheritance and missing values", () => {
    const job = workflowJob({
      workflow_ref: { workflow_id: "wf", parameters: { version: " 2.0.0 ", branch: "main" } },
      runtime_inputs: [
        { key: "version", label: "version", declared: false },
        { key: "ticket", label: "工单", declared: true },
        { key: "branch", label: "branch", declared: false },
        { key: "env", label: "env", declared: false },
      ],
    });
    const states = jobInputStates(job, { ticket: "OPS-1", env: " " });
    expect(states.map((s) => [s.key, s.source, s.value, s.overridable])).toEqual([
      ["version", "job", "2.0.0", true],
      ["ticket", "plan", "OPS-1", true],
      ["branch", "missing", "", false],
      ["env", "missing", "", true],
    ]);
  });

  it("lists missing inputs per pending workflow job", () => {
    const plan = {
      parameters: { version: "1.0.0" },
      jobs: [
        workflowJob({ id: "a", name: "dev", runtime_inputs: [{ key: "version", label: "v", declared: false }] }),
        workflowJob({ id: "b", name: "prod", runtime_inputs: [{ key: "ticket", label: "t", declared: false }, { key: "env", label: "e", declared: false }] }),
        { id: "c", name: "检查", type: "text" as const, status: "todo" },
      ],
    };
    const missing = planMissingInputs(plan);
    expect(missing).toEqual([{ jobId: "b", jobName: "prod", keys: ["ticket", "env"] }]);
    expect(formatMissingInputs(missing)).toBe("请先填写运行参数：「prod」ticket、env");
  });

  it("keeps only undeclared inputs that the version field does not cover", () => {
    const extra = runModalExtraInputs([
      { key: "ENV", label: "ENV", declared: true },
      { key: "version", label: "version", declared: false, refs: [ref(2, "archive")] },
      { key: "ticket", label: "ticket", declared: false, refs: [ref(2, "archive"), ref(0, "build", "branch")] },
      { key: "note", label: "note", declared: false },
    ]);
    expect(extra.map((item) => item.key)).toEqual(["ticket", "note"]);
  });
});
```

- [ ] **Step 2: 在 `release-plan-workflow-params.test.ts` 追加失败测试**

```ts
describe("mergeWorkflowJobParameters variables", () => {
  it("writes and clears per-job input overrides", () => {
    const merged = mergeWorkflowJobParameters(
      { branch: "main", version: "1.0.0" },
      null,
      { variables: { version: " 2.0.0 ", ticket: "OPS-1" } },
    );
    expect(merged).toEqual({ branch: "main", version: "2.0.0", ticket: "OPS-1" });
    expect(mergeWorkflowJobParameters(merged, null, { variables: { ticket: null } })).toEqual({
      branch: "main",
      version: "2.0.0",
    });
  });
});
```

- [ ] **Step 3: 运行测试确认失败**

Run（`fronted/`）：`npx vitest run apps/web/src/modules/product/components/release-plan-runtime-inputs.test.ts apps/web/src/modules/product/components/release-plan-workflow-params.test.ts`
Expected: 新文件因模块不存在而失败；`variables` 用例因输出缺少 `ticket` 而失败。

- [ ] **Step 4: 实现 `release-plan-runtime-inputs.ts`**

```ts
import type {
  ReleasePlanJobVM,
  ReleasePlanVM,
  WorkflowRuntimeInput,
  WorkflowRuntimeInputRef,
} from "@sparkops/contracts/product";

export const MASKED_PARAMETER_VALUE = "******";

const RESERVED_JOB_KEYS = new Set(["branch", "environment_id"]);

export function canOverrideInJob(key: string): boolean {
  return Boolean(key) && !key.startsWith("_") && !RESERVED_JOB_KEYS.has(key);
}

export function formatInputRef(ref: WorkflowRuntimeInputRef): string {
  return `阶段${ref.stage_index + 1} · ${ref.job_name}`;
}

function pendingWorkflowJobs(jobs: ReleasePlanJobVM[] | undefined): ReleasePlanJobVM[] {
  return (jobs ?? []).filter((job) => job.type === "workflow" && job.status !== "skipped");
}

export interface PlanInputRow {
  key: string;
  label: string;
  secret: boolean;
  usedBy: string[];
}

export function planInputRows(jobs: ReleasePlanJobVM[]): PlanInputRow[] {
  const rows = new Map<string, PlanInputRow>();
  for (const job of pendingWorkflowJobs(jobs)) {
    for (const input of job.runtime_inputs ?? []) {
      const row = rows.get(input.key) ?? { key: input.key, label: input.label || input.key, secret: false, usedBy: [] };
      row.secret = row.secret || Boolean(input.secret);
      const places = input.refs?.length ? input.refs.map(formatInputRef) : ["运行时变量"];
      for (const place of places) {
        const label = `${job.name} · ${place}`;
        if (!row.usedBy.includes(label)) row.usedBy.push(label);
      }
      rows.set(input.key, row);
    }
  }
  return [...rows.values()];
}

export type JobInputSource = "job" | "plan" | "missing";

export interface JobInputState {
  key: string;
  label: string;
  secret: boolean;
  source: JobInputSource;
  value: string;
  overridable: boolean;
}

export function jobInputStates(
  job: ReleasePlanJobVM,
  planParams: Record<string, string> | undefined,
): JobInputState[] {
  const params = job.workflow_ref?.parameters ?? {};
  return (job.runtime_inputs ?? []).map((input) => {
    const overridable = canOverrideInJob(input.key);
    const own = overridable ? (params[input.key] ?? "").trim() : "";
    const inherited = (planParams?.[input.key] ?? "").trim();
    const source: JobInputSource = own ? "job" : inherited ? "plan" : "missing";
    return {
      key: input.key,
      label: input.label || input.key,
      secret: Boolean(input.secret),
      source,
      value: own || inherited,
      overridable,
    };
  });
}

export interface JobMissingInputs {
  jobId: string;
  jobName: string;
  keys: string[];
}

export function planMissingInputs(plan: Pick<ReleasePlanVM, "jobs" | "parameters">): JobMissingInputs[] {
  const out: JobMissingInputs[] = [];
  for (const job of pendingWorkflowJobs(plan.jobs)) {
    const keys = jobInputStates(job, plan.parameters)
      .filter((state) => state.source === "missing")
      .map((state) => state.key);
    if (keys.length > 0) out.push({ jobId: job.id, jobName: job.name, keys });
  }
  return out;
}

export function formatMissingInputs(items: JobMissingInputs[]): string {
  return `请先填写运行参数：${items.map((item) => `「${item.jobName}」${item.keys.join("、")}`).join("；")}`;
}

/** 运行弹窗额外需要填写的输入：未声明，且不只出现在版本号字段里（版本号有专门输入框）。 */
export function runModalExtraInputs(inputs: WorkflowRuntimeInput[] | undefined): WorkflowRuntimeInput[] {
  return (inputs ?? []).filter(
    (input) =>
      !input.declared && !(input.refs?.length && input.refs.every((item) => item.field === "version_tag")),
  );
}
```

- [ ] **Step 5: 扩展 `mergeWorkflowJobParameters`**

`WorkflowJobParamsPatch` 加：

```ts
  /** 发布项单独填写的运行参数；值为 null 或空表示恢复继承 */
  variables?: Record<string, string | null>;
```

`mergeWorkflowJobParameters` 在 `return merged;` 之前加：

```ts
  if (patch.variables) {
    for (const [key, value] of Object.entries(patch.variables)) {
      const trimmed = value?.trim() ?? "";
      if (trimmed) merged[key] = trimmed;
      else delete merged[key];
    }
  }
```

- [ ] **Step 6: 运行测试确认通过**

Run（`fronted/`）：`npx vitest run apps/web/src/modules/product/components/release-plan-runtime-inputs.test.ts apps/web/src/modules/product/components/release-plan-workflow-params.test.ts`
Expected: 全部 PASS。

- [ ] **Step 7: 提交**

```bash
git add fronted/apps/web/src/modules/product/components/release-plan-runtime-inputs.ts fronted/apps/web/src/modules/product/components/release-plan-runtime-inputs.test.ts fronted/apps/web/src/modules/product/components/release-plan-workflow-params.ts fronted/apps/web/src/modules/product/components/release-plan-workflow-params.test.ts
git commit -m "feat(release-plan): runtime input helpers for plan parameters"
```

---

### Task 9: 发布计划界面——发布参数区块、运行参数字段、完成规划预检

**Files:**
- Create: `fronted/apps/web/src/modules/product/components/ReleasePlanParametersSection.tsx`
- Create: `fronted/apps/web/src/modules/product/components/ReleasePlanJobInputs.tsx`
- Modify: `fronted/apps/web/src/modules/product/components/ReleasePlanJobEditor.tsx`
- Modify: `fronted/apps/web/src/modules/product/components/ReleasePlansPanel.tsx`
- Modify: `fronted/apps/web/app/globals.css`（`.release-plan-job-field-hint` 附近追加）

**Interfaces:**
- Consumes: Task 8 的 `planInputRows`、`jobInputStates`、`planMissingInputs`、`formatMissingInputs`、`JobInputState`、`WorkflowJobParamsPatch.variables`。
- Produces:
  - `ReleasePlanParametersSection({ plan, editable, busy, onChange(parameters) })`
  - `ReleasePlanJobInputs({ states, editable, busy, onChange(key, value | null) })`
  - `ReleasePlanJobEditorProps.focusJobIds?: string[]`

- [ ] **Step 1: 发布参数区块 `ReleasePlanParametersSection.tsx`**

```tsx
"use client";

import { useMemo } from "react";
import type { ReleasePlanVM } from "@sparkops/contracts/product";
import { planInputRows, planMissingInputs } from "./release-plan-runtime-inputs";

interface ReleasePlanParametersSectionProps {
  plan: ReleasePlanVM;
  editable: boolean;
  busy?: boolean;
  onChange: (parameters: Record<string, string>) => void | Promise<void>;
}

export function ReleasePlanParametersSection({ plan, editable, busy, onChange }: ReleasePlanParametersSectionProps) {
  const rows = useMemo(() => planInputRows(plan.jobs ?? []), [plan.jobs]);
  const missingKeys = useMemo(
    () => new Set(planMissingInputs(plan).flatMap((item) => item.keys)),
    [plan],
  );
  if (rows.length === 0) return null;
  const params = plan.parameters ?? {};

  function commit(key: string, raw: string) {
    const value = raw.trim();
    if (value === (params[key] ?? "")) return;
    const next = { ...params };
    if (value) next[key] = value;
    else delete next[key];
    void onChange(next);
  }

  return (
    <section className="release-plan-params" aria-label="发布参数">
      <header className="release-plan-params-head">
        <h3 className="release-plan-params-title">发布参数</h3>
        <p className="release-plan-params-desc">所有工作流发布项共用；单个发布项可在「运行参数」中单独填写。</p>
      </header>
      <ul className="release-plan-params-list">
        {rows.map((row) => {
          const value = params[row.key] ?? "";
          const missing = !value && missingKeys.has(row.key);
          return (
            <li key={row.key} className={`release-plan-param-row${missing ? " is-missing" : ""}`}>
              <div className="release-plan-param-meta">
                <span className="release-plan-param-key">{row.key}</span>
                {row.label !== row.key ? <span className="release-plan-param-label">{row.label}</span> : null}
                <span className="release-plan-param-usage" title={row.usedBy.join("\n")}>
                  {row.usedBy.join("、")}
                </span>
              </div>
              {editable ? (
                <input
                  key={`${row.key}-${value}`}
                  className="input text-sm release-plan-param-input"
                  type={row.secret ? "password" : "text"}
                  defaultValue={value}
                  disabled={busy}
                  placeholder={missing ? "必填" : "未填写"}
                  aria-label={`${row.key} 取值`}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") e.currentTarget.blur();
                  }}
                  onBlur={(e) => commit(row.key, e.target.value)}
                />
              ) : (
                <span className="release-plan-param-value">
                  {value ? (row.secret ? "••••••" : value) : "未填写"}
                </span>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
```

- [ ] **Step 2: 发布项运行参数 `ReleasePlanJobInputs.tsx`**

```tsx
"use client";

import { useState } from "react";
import type { JobInputState } from "./release-plan-runtime-inputs";

interface ReleasePlanJobInputsProps {
  states: JobInputState[];
  editable: boolean;
  busy?: boolean;
  onChange: (key: string, value: string | null) => void;
}

function display(state: JobInputState): string {
  const shown = state.secret ? "••••••" : state.value;
  if (state.source === "plan") return `继承：${shown}`;
  if (state.source === "missing") return "未填写";
  return shown;
}

export function ReleasePlanJobInputs({ states, editable, busy, onChange }: ReleasePlanJobInputsProps) {
  const [editing, setEditing] = useState<Set<string>>(() => new Set());

  function stopEditing(key: string) {
    setEditing((prev) => {
      const next = new Set(prev);
      next.delete(key);
      return next;
    });
  }

  return (
    <ul className="release-plan-job-inputs">
      {states.map((state) => {
        const ownInput = editable && state.overridable && (state.source === "job" || editing.has(state.key));
        const original = state.source === "job" ? state.value : "";
        return (
          <li
            key={state.key}
            className={`release-plan-job-input${state.source === "missing" && !ownInput ? " is-missing" : ""}`}
          >
            <span className="release-plan-job-input-key" title={state.label}>
              {state.key}
            </span>
            {ownInput ? (
              <input
                key={`${state.key}-${state.source}-${state.value}`}
                autoFocus={state.source !== "job"}
                className="input text-xs release-plan-job-field-control"
                type={state.secret ? "password" : "text"}
                defaultValue={original}
                disabled={busy}
                placeholder={state.source === "plan" ? display(state) : "填写本项取值"}
                aria-label={`${state.key} 本项取值`}
                onKeyDown={(e) => {
                  if (e.key === "Enter") e.currentTarget.blur();
                  if (e.key === "Escape") {
                    e.currentTarget.value = original;
                    e.currentTarget.blur();
                  }
                }}
                onBlur={(e) => {
                  const value = e.target.value.trim();
                  stopEditing(state.key);
                  if (value === original) return;
                  onChange(state.key, value || null);
                }}
              />
            ) : (
              <span className="release-plan-job-input-value">{display(state)}</span>
            )}
            {editable && state.overridable && state.source === "job" ? (
              <button
                type="button"
                className="release-plan-job-input-action"
                disabled={busy}
                onClick={() => onChange(state.key, null)}
              >
                恢复继承
              </button>
            ) : null}
            {editable && state.overridable && state.source !== "job" && !ownInput ? (
              <button
                type="button"
                className="release-plan-job-input-action"
                disabled={busy}
                onClick={() => setEditing((prev) => new Set(prev).add(state.key))}
              >
                单独填写
              </button>
            ) : null}
          </li>
        );
      })}
    </ul>
  );
}
```

- [ ] **Step 3: 发布项编辑器接入运行参数并支持聚焦缺参项**

`ReleasePlanJobEditor.tsx`：

1. 导入 `import { ReleasePlanJobInputs } from "./ReleasePlanJobInputs";` 与 `import { jobInputStates } from "./release-plan-runtime-inputs";`。
2. `ReleasePlanJobEditorProps` 加：

```ts
  /** 需要展开并滚动到的发布项（如完成规划预检发现缺参） */
  focusJobIds?: string[];
```

并在组件参数解构中加入 `focusJobIds`。
3. 在 `expandedIds` 声明之后加：

```ts
  useEffect(() => {
    if (!focusJobIds?.length) return;
    setExpandedIds((prev) => new Set([...prev, ...focusJobIds]));
    window.requestAnimationFrame(() => {
      document
        .getElementById(`release-plan-job-${focusJobIds[0]}`)
        ?.scrollIntoView({ block: "center", behavior: "smooth" });
    });
  }, [focusJobIds]);
```

4. 在 `jobs.map` 回调中 `const jobHasBuild = ...` 之后加：

```ts
            const jobInputs = jobInputStates(job, plan.parameters);
            const canEditInputs =
              canEditJob || (plan.status === "executing" && job.type === "workflow" && job.status === "failed");
```

5. 在「执行项目」`JobField` 结束（`) : null}`）之后、外层 `</>` 之前插入：

```tsx
                        {jobInputs.length > 0 ? (
                          <JobField label="运行参数" alignTop>
                            <ReleasePlanJobInputs
                              states={jobInputs}
                              editable={canEditInputs}
                              busy={busy}
                              onChange={(key, value) =>
                                void onWorkflowParamsChange(job.id, workflowId, { variables: { [key]: value } })
                              }
                            />
                          </JobField>
                        ) : null}
```

6. 收起状态摘要（`release-plan-job-summary-meta` 的数组）末尾加一项，缺参时提示：

```ts
                                jobInputs.some((state) => state.source === "missing") ? "缺少运行参数" : null,
```

- [ ] **Step 4: `ReleasePlansPanel.tsx` 接入**

1. 导入 `ReleasePlanParametersSection`，以及 `formatMissingInputs`、`planMissingInputs`；`WorkflowJobParamsPatch` 从 `./release-plan-workflow-params` 导入（若未导入）。
2. 新增状态 `const [missingFocus, setMissingFocus] = useState<string[]>([]);`。
3. `onWorkflowParamsChange` 的 `patch` 参数类型改为 `WorkflowJobParamsPatch`，`workflow_ref.mode` 改为沿用原值：

```ts
        workflow_ref: {
          mode: job?.workflow_ref?.mode ?? "reference",
          workflow_id: workflowId,
          parameters,
        },
```

4. `onFinishPlanning` 在 `if (!selected) return;` 之后加预检：

```ts
    const missing = planMissingInputs(selected);
    if (missing.length > 0) {
      setError(formatMissingInputs(missing));
      setDetailTab("jobs");
      setMissingFocus(missing.map((item) => item.jobId));
      return;
    }
```

5. 新增：

```ts
  async function onPlanParametersChange(parameters: Record<string, string>) {
    if (!selected) return;
    setBusy(true);
    try {
      await product.updateReleasePlan(workspaceId, productId, selected.id, { parameters });
      await refreshDetail();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存发布参数失败");
    } finally {
      setBusy(false);
    }
  }
```

6. `detailTab === "jobs"` 分支包成片段，在 `ReleasePlanJobEditor` 前渲染区块，并给编辑器传 `focusJobIds`：

```tsx
            {detailTab === "jobs" ? (
              <>
                <ReleasePlanParametersSection
                  plan={selected}
                  editable={selected.status === "planning"}
                  busy={busy}
                  onChange={onPlanParametersChange}
                />
                <ReleasePlanJobEditor
                  {/* 原有 props 不变 */}
                  focusJobIds={missingFocus}
                />
              </>
            ) : detailTab === "audit" ? (
```

（上面注释仅示意：保留原有全部 props，只新增 `focusJobIds`。）

- [ ] **Step 5: 样式（追加在 `.release-plan-job-field-hint` 规则之后）**

```css
.release-plan-params {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-bottom: 1.25rem;
  padding: 1rem 1.125rem;
  border-radius: var(--radius-md);
  background: rgba(0, 0, 0, 0.02);
}

.release-plan-params-head {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
}

.release-plan-params-title {
  margin: 0;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text);
}

.release-plan-params-desc {
  margin: 0;
  font-size: 0.75rem;
  color: var(--muted);
}

.release-plan-params-list {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
}

.release-plan-param-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(12rem, 18rem);
  align-items: center;
  gap: 1rem;
  padding: 0.5rem 0;
}

.release-plan-param-row + .release-plan-param-row {
  border-top: 1px solid var(--border);
}

.release-plan-param-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.25rem 0.5rem;
  min-width: 0;
}

.release-plan-param-key,
.release-plan-job-input-key {
  font-family: var(--font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 0.8125rem;
  color: var(--text);
}

.release-plan-param-label {
  font-size: 0.8125rem;
  color: var(--text-secondary);
}

.release-plan-param-usage {
  flex-basis: 100%;
  overflow: hidden;
  font-size: 0.75rem;
  color: var(--muted);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.release-plan-param-value {
  font-size: 0.8125rem;
  color: var(--text);
}

.release-plan-param-row.is-missing .release-plan-param-key,
.release-plan-param-row.is-missing .release-plan-param-value {
  color: var(--warning);
}

.release-plan-param-row.is-missing .release-plan-param-input {
  border-color: var(--warning-border);
}

.release-plan-job-inputs {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.release-plan-job-input {
  display: grid;
  grid-template-columns: minmax(6rem, 10rem) minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
}

.release-plan-job-input-value {
  overflow: hidden;
  font-size: 0.8125rem;
  color: var(--text-secondary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.release-plan-job-input.is-missing .release-plan-job-input-value {
  color: var(--warning);
}

.release-plan-job-input-action {
  padding: 0;
  border: none;
  background: none;
  font-size: 0.75rem;
  color: var(--muted);
  cursor: pointer;
  transition: color var(--motion-fast, 120ms) var(--motion-ease, ease);
}

.release-plan-job-input-action:hover:not(:disabled) {
  color: var(--text);
}
```

使用前确认 `--radius-md`、`--text-secondary`、`--motion-ease` 在 `globals.css` 的 `:root` 中存在；不存在的变量直接用回退值，不要新增全局 token。

- [ ] **Step 6: 类型检查**

Run（`fronted/`）：`pnpm --filter @sparkops/web typecheck`
Expected: 与 Task 7 记录的基线相比没有新增错误。

- [ ] **Step 7: 手动验证（`.\scripts\dev-restart.ps1 -Frontend` 后在浏览器操作）**

1. 打开含 `dev` 工作流（`version_tag` 为 `{{version}}`）的规划中计划：「发布项」Tab 顶部出现「发布参数」，`version` 行警示色，引用显示「dev · 阶段3 · archive」。
2. 不填直接点「完成规划」：出现「请先填写运行参数：「dev」version」，dev 卡片展开并滚动到可见，「运行参数」显示「未填写」。
3. 在发布参数填 `1.2.0` 后失焦：卡片显示「继承：1.2.0」；点「单独填写」输入 `1.2.1` 后显示 `1.2.1` 和「恢复继承」；点「恢复继承」回到继承。
4. 点「完成规划」成功；执行到创建版本阶段，版本号为 `1.2.0` / `1.2.1`。

- [ ] **Step 8: 提交**

```bash
git add fronted/apps/web/src/modules/product/components/ReleasePlanParametersSection.tsx fronted/apps/web/src/modules/product/components/ReleasePlanJobInputs.tsx fronted/apps/web/src/modules/product/components/ReleasePlanJobEditor.tsx fronted/apps/web/src/modules/product/components/ReleasePlansPanel.tsx fronted/apps/web/app/globals.css
git commit -m "feat(release-plan): plan parameters and per-job run inputs UI"
```

---

### Task 10: 运行弹窗补充未声明输入，工作流编辑器提示

**Files:**
- Modify: `fronted/apps/web/src/modules/product/components/StartWorkflowRunModal.tsx:73-76`
- Modify: `fronted/apps/web/src/modules/product/components/WorkflowSidePanels.tsx:31-120`
- Modify: `fronted/apps/web/src/modules/product/components/ProductWorkflowEditor.tsx`（props 与 642 行）
- Modify: `fronted/apps/web/src/modules/product/components/ProductWorkflowEditorPage.tsx`
- Modify: `fronted/apps/web/app/globals.css`

**Interfaces:**
- Consumes: Task 8 的 `runModalExtraInputs`、`formatInputRef`；Task 7 的 `WorkflowRuntimeInput`。
- Produces: `WorkflowVariablesPanel` 新增可选 props `undeclared?: WorkflowRuntimeInput[]`、`onDeclare?: (key: string) => void`；`ProductWorkflowEditorProps.undeclaredInputs?: WorkflowRuntimeInput[]`。

- [ ] **Step 1: 运行弹窗把未声明输入当作必填变量**

`StartWorkflowRunModal.tsx` 导入 `runModalExtraInputs`，把 `runtimeVariables` 改为：

```ts
  const runtimeVariables = useMemo(() => {
    const declared = (workflow?.variables ?? []).filter((item) => item.runtime_input && item.key.trim());
    const extra = runModalExtraInputs(workflow?.runtime_inputs)
      .filter((input) => !declared.some((item) => item.key === input.key))
      .map((input) => ({
        id: `runtime-input-${input.key}`,
        key: input.key,
        value: "",
        description: input.label,
        runtime_input: true,
        secret: false,
        vault_path: "",
      }));
    return [...declared, ...extra];
  }, [workflow]);
```

其余渲染、校验、提交逻辑不变（它们只读 `key`、`value`、`description`、`secret`）。

- [ ] **Step 2: 变量面板显示未声明占位符**

`WorkflowSidePanels.tsx`：导入 `WorkflowRuntimeInput` 类型与 `formatInputRef`；`WorkflowVariablesPanel` 签名改为：

```tsx
export function WorkflowVariablesPanel({
  variables,
  onChange,
  undeclared = [],
  onDeclare,
}: {
  variables: WorkflowVariableVM[];
  onChange: (variables: WorkflowVariableVM[]) => void;
  undeclared?: WorkflowRuntimeInput[];
  onDeclare?: (key: string) => void;
}) {
```

在说明 `<p>` 之后插入：

```tsx
      {undeclared.length > 0 ? (
        <div className="pwf-undeclared">
          <p className="text-sm font-medium">未声明的占位符</p>
          <p className="text-xs text-muted">以下占位符没有声明为变量，运行时将作为必填参数。</p>
          {undeclared.map((item) => (
            <div key={item.key} className="pwf-undeclared-row">
              <span className="pwf-undeclared-key">{item.key}</span>
              <span className="pwf-undeclared-refs">{(item.refs ?? []).map(formatInputRef).join("、")}</span>
              {onDeclare ? (
                <button type="button" className="btn btn-ghost text-sm" onClick={() => onDeclare(item.key)}>
                  声明为变量
                </button>
              ) : null}
            </div>
          ))}
        </div>
      ) : null}
```

- [ ] **Step 3: 编辑器与页面传值**

`ProductWorkflowEditor.tsx`：导入 `WorkflowRuntimeInput` 类型与 `emptyVariable`（`./workflow-settings`）；props 加 `undeclaredInputs?: WorkflowRuntimeInput[];` 并解构；642 行改为：

```tsx
        {sidePanel === "variables" ? (
          <WorkflowVariablesPanel
            variables={variables}
            onChange={onVariablesChange}
            undeclared={undeclaredInputs}
            onDeclare={(key) => onVariablesChange([...variables, { ...emptyVariable(), key, runtime_input: true }])}
          />
        ) : null}
```

`ProductWorkflowEditorPage.tsx`：

```ts
  const [runtimeInputs, setRuntimeInputs] = useState<WorkflowRuntimeInput[]>([]);
```

`load` 中 `setVariables(...)` 后加 `setRuntimeInputs(wf.runtime_inputs ?? []);`；`handleSave` 中 `setVariables(...)` 后加 `setRuntimeInputs(saved.runtime_inputs ?? runtimeInputs);`；渲染 `ProductWorkflowEditor` 时加：

```tsx
      undeclaredInputs={runtimeInputs.filter(
        (input) => !input.declared && !variables.some((item) => item.key.trim() === input.key),
      )}
```

- [ ] **Step 4: 样式（追加在 `.pwf-sys-vars` 规则附近）**

```css
.pwf-undeclared {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.75rem;
  border-radius: var(--radius-md);
  background: var(--warning-bg);
}

.pwf-undeclared-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
}

.pwf-undeclared-key {
  font-family: var(--font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 0.8125rem;
  color: var(--warning);
}

.pwf-undeclared-refs {
  overflow: hidden;
  font-size: 0.75rem;
  color: var(--muted);
  text-overflow: ellipsis;
  white-space: nowrap;
}
```

- [ ] **Step 5: 类型检查与手动验证**

Run（`fronted/`）：`pnpm --filter @sparkops/web typecheck`，与基线比较无新增错误。

手动：
1. 在 `dev` 工作流编辑器打开「变量」面板，看到「未声明的占位符 · version · 阶段3 · archive」；点「声明为变量」后列表项消失，变量列表新增 `version`（运行时填写）；保存后刷新仍不再提示。
2. 给某个工作流的构建任务分支写 `{{ticket}}`，打开运行弹窗，「运行时变量」出现 `ticket` 且必填；`{{version}}` 只出现在版本号字段时不重复要求。

- [ ] **Step 6: 提交**

```bash
git add fronted/apps/web/src/modules/product/components/StartWorkflowRunModal.tsx fronted/apps/web/src/modules/product/components/WorkflowSidePanels.tsx fronted/apps/web/src/modules/product/components/ProductWorkflowEditor.tsx fronted/apps/web/src/modules/product/components/ProductWorkflowEditorPage.tsx fronted/apps/web/app/globals.css
git commit -m "feat(workflow): surface undeclared placeholders in run modal and editor"
```

---

### Task 11: 端到端回归

**Files:** 无新增。

- [ ] **Step 1: 后端全量相关测试**

Run（`backend/`）：`go build ./... && go test ./internal/modules/product/... ./internal/modules/product_workflow/... ./internal/platform/... -count=1`
Expected: 全部 PASS。

- [ ] **Step 2: 前端相关测试**

Run（`fronted/`）：`npx vitest run apps/web/src/modules/product/components`
Expected: 全部 PASS。

- [ ] **Step 3: 重启并复现原始故障场景**

Run：`.\scripts\dev-restart.ps1`
操作：对 `deploy1`（工作流 `dev`）重新打开规划（或复制计划），在「发布参数」填写 `version`，完成规划、执行。
Expected: 第三阶段「创建版本 · archive」使用填写的版本号成功；不再出现「版本号 "{{version}}" 包含未替换的占位符」。

- [ ] **Step 4: 执行中补参重试**

操作：完成规划后，在工作流编辑器给 `dev` 新增一个未声明占位符（如部署任务描述写 `{{ticket}}`）并保存，再点执行。
Expected: 发布项立即变为失败，错误显示「缺少运行时必填变量: ticket」，没有创建运行记录；发布项「运行参数」出现 `ticket` 可「单独填写」；填写后点「重试」成功启动。
