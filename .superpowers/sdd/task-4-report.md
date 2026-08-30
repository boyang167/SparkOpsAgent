## Status

Task 4 implemented with RED -> GREEN. Commit hash is generated after this report is staged; final task response records the actual commit.

## Files

- `backend/migrations/000033_plugin_framework.up.sql`
- `backend/migrations/000033_plugin_framework.down.sql`
- `backend/internal/shared/kernel/ids.go`
- `backend/internal/modules/plugin/domain/model/plugin.go`
- `backend/internal/modules/plugin/domain/model/plugin_test.go`
- `backend/internal/modules/plugin/domain/repository/repository.go`
- `backend/internal/modules/plugin/domain/errors.go`
- `backend/internal/modules/plugin/infrastructure/persistence/store.go`
- `backend/internal/modules/plugin/infrastructure/persistence/store_test.go`
- `backend/internal/bootstrap/migrate.go`
- `.superpowers/sdd/task-4-report.md`

## RED

Command:

```text
go test ./internal/modules/plugin/domain/model/ -count=1
```

Output:

```text
github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model: no non-test Go files in A:\21-Agent\SparkOpsAgent\backend\internal\modules\plugin\domain\model
FAIL    github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model [build failed]
FAIL
```

Command:

```text
go test ./internal/modules/plugin/infrastructure/persistence/ -count=1
```

Output:

```text
FAIL    github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/infrastructure/persistence [setup failed]
FAIL
# github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/infrastructure/persistence
internal\modules\plugin\infrastructure\persistence\store_test.go:11:2: no required module provides package github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain; to add it:
        go get github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain
```

## GREEN

Command:

```text
go test ./internal/modules/plugin/domain/model/ -count=1
```

Output:

```text
ok      github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model  2.832s
```

Command:

```text
go test ./internal/modules/plugin/... -count=1
```

Output:

```text
?       github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain        [no test files]
ok      github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest       2.980s
ok      github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model  3.056s
?       github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/repository     [no test files]
ok      github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/infrastructure/persistence     3.334s
```

Command:

```text
go test -v ./internal/modules/plugin/infrastructure/persistence/ -count=1
```

Output:

```text
=== RUN   TestAutoMigrateCreatesPluginSchemaTables
    store_test.go:35: DATABASE_DSN is not set
--- SKIP: TestAutoMigrateCreatesPluginSchemaTables (0.00s)
=== RUN   TestStorePersistsPluginsAndWorkspaceBindings
    store_test.go:45: DATABASE_DSN is not set
--- SKIP: TestStorePersistsPluginsAndWorkspaceBindings (0.00s)
PASS
ok      github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/infrastructure/persistence     3.102s
```

Command:

```text
go test ./internal/bootstrap -count=1
```

Output:

```text
ok      github.com/sparkops/sparkops-agent/backend/internal/bootstrap     3.183s
```

## Self-Review

- Plugin lifecycle follows the brief transitions and adds the user-decided uninstall reservations: `uninstalling`, `uninstall_merged`, `MarkUninstalling(prURL)`, `MarkUninstallMerged()`, and `DeletePlugin(ctx,id)`.
- SQL migration uses the required `plugin.plugins` and `plugin.workspace_bindings` schema/table names and fields.
- Repository interface exposes only Task 4 persistence operations; no Task 7 command or HTTP surface was added.
- Changes were scoped to the Task 4 files plus the explicitly allowed `ids.go` and `migrate.go`.

## Concerns

- PostgreSQL repository integration tests were skipped because `DATABASE_DSN` is not set in this environment; they compile and are ready to run against a PostgreSQL DSN.
- `backend/internal/shared/kernel/ids.go` is not gofmt-clean in the pre-existing file style; I kept its diff minimal instead of reformatting unrelated lines.
# Task 4 Report

## Status

PASS — Task 4 已完成，未执行 `git add`、`git commit` 或 `git restore`。

实现范围仅包括 Jenkins JobSpec/BuildRequest port、Folder/Job create-or-update、稳定凭据 ID、Jenkinsfile XML upsert、部署专用 TriggerDeployment 参数，以及普通 Build/rollback 兼容适配。未实现 K8s、BuildCompleted orchestration 或前端。

## Files

新增：

- `backend/internal/modules/delivery/application/command/trigger_build_test.go`
- `backend/internal/modules/delivery/deployrun/application/service/prepare.go`
- `backend/internal/modules/delivery/deployrun/application/service/prepare_test.go`
- `backend/internal/modules/delivery/infrastructure/jenkins/executor_test.go`

修改：

- `backend/internal/modules/delivery/domain/port/ports.go`
- `backend/internal/modules/delivery/application/command/trigger_build.go`
- `backend/internal/modules/delivery/application/command/rollback_build.go`
- `backend/internal/modules/delivery/infrastructure/jenkins/executor.go`
- `backend/internal/modules/delivery/infrastructure/jenkins/mock_executor.go`

其中 `rollback_build.go` 仅为适配新的 `BuildRequest` 接口，保持原普通 rollback 行为；未改动其他 Tasks 1-3 文件。

## RED / GREEN

RED：

- Jenkins 定向测试因 `renderJobXML`、`JenkinsJobSpec`、`BuildRequest`、`buildParams` 未定义而失败。
- Prepare 测试因 `NewPrepareService`、JobSpec/credential port 未定义而失败。
- Trigger 测试因 `TriggerDeployment`、BuildRequest 未定义而失败。
- 部署无后缀测试先得到 `orders-build`，按预期失败后修正为部署默认 `orders`。
- 真实 adapter HTTP 契约测试暴露 gojenkins `GetFolder` 在非 JSON 404 body 上先解码失败；改为通过实际 `Requester.Get` 签名读取资源 HTTP 状态，再调用已验证的 folder/job API。

GREEN：

- Task 4 定向测试：PASS。
- `go test ./internal/modules/delivery/...`：PASS。
- `go test ./...`：PASS。
- `go vet ./...`：PASS。
- `go run ./scripts/check-deps.go`：PASS（`domain dependency check passed`）。
- Cursor lint：delivery 目录无诊断。

## Self-review

- Job XML 使用 `encoding/xml.EscapeText` 转义 Jenkinsfile，并通过反序列化验证脚本内容；参数定义包含 `BRANCH`、`CodeVersion`、`IMAGE_TAG`、`IMAGE_REPOSITORY`。
- 部署参数携带 branch/code version、image tag、image repository；普通 Build 仍只发送兼容的 `BRANCH`，job 保持 `<project>-build`。
- 部署 Folder 为 namespace、Job 为 project name；部署默认不附加 `jobSuffix`。
- 凭据 ID 固定为 `sparkops-{workspaceID}-{secretID}`；支持 username/password、secret text、SSH private key，错误只包含 credential ID/kind，不包含 secret。
- Mock executor 记录最后 JobSpec/BuildRequest，并保留可预测流式输出。
- 已按 gojenkins v1.2.0 实际源码/签名验证 `CreateFolder`、`CreateJobInFolder`、`GetJob(parentIDs...)`、`Job.UpdateConfig`、`CredentialsManager.List/Add/Update`，未假设不存在的 API。

## Concerns

- `go test -race` 在当前 Windows Go/MinGW 环境因 `cc1.exe: 64-bit mode not compiled in` 无法启动；普通测试、完整测试和 vet 均通过。
- 按 Task 4 边界仅提供 `PrepareService` 与 `TriggerDeployment`；未接入后续 BuildCompleted/deployment consumer orchestration。

---

## Review Fix (Critical / Important)

### Status

PASS — 审查提出的 4 组 Critical/Important 已按 TDD 修复；仍未执行 `git add`、`git commit` 或 `git restore`。

### Additional files

新增：

- `backend/internal/modules/delivery/deployrun/infrastructure/acl/pipeline_renderer.go`
- `backend/internal/modules/delivery/deployrun/infrastructure/acl/pipeline_renderer_test.go`

追加修改：

- `backend/internal/modules/delivery/deployrun/domain/errors.go`
- `backend/internal/modules/delivery/deployrun/domain/port/project_deployment_reader.go`
- `backend/internal/modules/delivery/deployrun/application/service/prepare.go`
- `backend/internal/modules/delivery/deployrun/application/service/prepare_test.go`
- `backend/internal/modules/delivery/domain/errors.go`
- `backend/internal/modules/delivery/domain/port/ports.go`
- `backend/internal/modules/delivery/application/command/trigger_build.go`
- `backend/internal/modules/delivery/application/command/trigger_build_test.go`
- `backend/internal/modules/delivery/infrastructure/jenkins/executor.go`
- `backend/internal/modules/delivery/infrastructure/jenkins/executor_test.go`
- `backend/internal/modules/workspace_app/application/query/generate_project_templates.go`
- `backend/internal/modules/workspace_app/application/query/generate_project_templates_test.go`
- `backend/internal/modules/workspace_app/application/query/template_render_context.go`

### RED / GREEN evidence

RED：

- Prepare 渲染测试因 `PipelineRenderer`、`PipelineRenderRequest`、新 `PrepareInput` 和空流水线业务错误未定义而编译失败。
- workspace_app 生产模板测试因 `RenderDeploymentPipelineArtifact` / `DeploymentPipelineRenderInput` 未定义而编译失败。
- 生产 renderer adapter 测试因 `NewPipelineRenderer` / `NewWorkspacePipelineRenderer` 未定义而编译失败。
- 部署请求强校验测试因 Namespace/ProjectName 字段和业务错误未定义而编译失败。
- 四参数测试得到仅 `BRANCH`，缺少 `CodeVersion`，按预期失败。
- Jenkins 并发冲突测试在 Folder create 409 返回 `create jenkins folder: 409`，按预期失败。

GREEN：

- Task 4 定向四包测试：PASS。
- `go test ./internal/modules/delivery/...`：PASS。
- `go test ./internal/modules/workspace_app/...`：PASS。
- `go test ./...`：PASS。
- `go vet ./...`：PASS。
- `go run ./scripts/check-deps.go`：PASS。
- Cursor lint：delivery 与受影响 workspace_app query 无诊断。

### Self-review

- Prepare 不再读取/透传 `ProjectDeploymentContext.Jenkinsfile`；通过 deployrun 窄 `PipelineRenderer` port 调用 workspace_app 的 `GenerateProjectTemplatesHandler` 渲染边界，并传入本次 git ref、image tag/repository 和稳定 Git/Harbor credential ID。
- 渲染结果 trim 后为空返回 `ErrDeploymentPipelineEmpty`，且不会调用 Jenkins upsert。
- `BuildRequest` 明确携带 Namespace/ProjectName；TriggerDeployment 在保存和启动 goroutine 前 trim 并校验所有部署坐标、四个参数、Folder/Namespace 与 Job/ProjectName 一致性，同时校验请求 ProjectName 与实际项目名一致。
- 保存的 `Build.JenkinsJob` 使用与 executor `GetJob(job, folder)` 相同的 `folder/job` 路径；普通 Build 仍使用无 Folder 的 `<project>-build` 和仅 `BRANCH` 参数。
- 部署请求由 Namespace/ProjectName 明确标识，executor 对部署无条件生成 `BRANCH`、`CodeVersion`、`IMAGE_TAG`、`IMAGE_REPOSITORY`；普通 Build 参数保持兼容。
- Folder、Job、credential create 遇到 409/already-exists 时均重新读取；Folder 继续，Job 按目标 XML UpdateConfig，credential 按目标值 Update。非冲突创建错误仍直接返回。

### Concerns

- `PrepareService` 与生产 `NewWorkspacePipelineRenderer` 已具备组合边界，但按 Task 4 范围未接入 Task 5/6 consumer orchestration。
- `go test -race` 的本机 MinGW 64 位限制仍存在；本轮未重复运行 race，标准全量测试与 vet 均通过。

---

## Review Fix 2 (Authoritative Namespace)

### Status

PASS — TriggerDeployment 现已在持久化 Build 和启动 goroutine 前，将请求 Namespace/Folder 与 `workspace_app.projects.namespace` 权威值比对；未执行 git add/commit/restore。

### Files

- 修改 `backend/internal/modules/delivery/domain/port/ports.go`
- 修改 `backend/internal/modules/delivery/application/command/trigger_build.go`
- 修改 `backend/internal/modules/delivery/application/command/trigger_build_test.go`
- 修改 `backend/internal/modules/delivery/infrastructure/acl/checkers.go`
- 新增 `backend/internal/modules/delivery/infrastructure/acl/checkers_test.go`

### RED / GREEN

RED：

- `TestTriggerDeploymentRejectsNamespaceDifferentFromPersistedProjectBeforeSave` 使用彼此相等的 `Namespace=Folder=client-team`，而 fake 权威 namespace 为 `team-a`；修复前错误为 nil，测试按预期失败。
- `TestProjectNamespaceReadsPersistedWorkspaceProjectNamespace` 因生产 ACL 尚无 `ProjectNamespace` 方法而编译失败。

GREEN：

- 上述两项定向测试：PASS。
- Task 4 相关五包定向测试：PASS。
- `go test ./internal/modules/delivery/...`：PASS。
- `go test ./internal/modules/workspace_app/...`：PASS。
- `go test ./...`：PASS。
- `go vet ./...`：PASS。
- `go run ./scripts/check-deps.go`：PASS。
- Cursor lint：相关文件无诊断。

### Self-review

- `ProjectChecker` 仅新增最窄 `ProjectNamespace(ctx, projectID) (string, error)`。
- workspace_app ACL 只查询持久化项目的 `namespace` 列。
- TriggerDeployment 先完成请求内部一致性检查，再通过现有项目门禁取得实际项目名，并读取权威 namespace；任一不匹配均返回 `ErrInvalidDeploymentBuildRequest`。
- namespace 不匹配测试同时断言未 Save Build，并等待确认 executor 未被调用；正确 namespace 路径继续通过并保持 `folder/job` 持久化一致。

### Concerns

- 无新增功能性 concern；既有本机 MinGW race 检测限制不变。
