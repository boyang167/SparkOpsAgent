# Task 8 报告：发布端事务迁移

日期：2026-10-17

基线：`366707fb8addd257a0d770ff0b29d47b7773354b`

## 范围

本任务只迁移现有 10 条事件发布链，使业务写入与 `ReliableBus.Publish` 共用
`postgres.Transactor.WithinTransaction` / `postgres.RequireTx` 上下文。未实现 Task 9
的 Inbox 订阅副作用迁移，也未实现 Task 10 的 Operation Outbox 外部操作迁移。

## 迁移清单

1. `identity.MemberRegistered`
   - `MemberRepository.Save` 与 Outbox 插入同事务。
   - 发布失败返回错误并回滚 member。
2. `integration.WorkspaceCreated`
   - workspace、workspace owner、organization owner 与 Outbox 同事务。
   - 不再忽略 `OrganizationRepository.AddMember` 错误。
3. `workspace_app.ProjectCreated`
   - project、project lead 与 Outbox 同事务。
   - Casbin 同步保持在事务提交之后，不进入数据库事务。
4. `integration.SecretUpdated`
   - secret update 与 Outbox 同事务；无凭据变更时不产生事件。
5. `agent.AgentPublished`
   - agent/current version 状态与 Outbox 同事务。
   - `AgentRepository.Save` 复用上下文事务，避免内层新开事务。
6. `delivery.BuildCompleted`
   - terminal build update 与 Outbox 同事务。
   - 删除旧的三次进程内 Publish 重试；持久发布重试由 Dispatcher 负责。
7. `delivery.DeployCompleted`
   - terminal callback 下沉到 `DeploymentRunRepository.Update` 的事务回调。
   - Outbox 失败时 deployment run update 和 deployment event 一并回滚。
8. `workflow.WorkflowRunCompleted`
   - run save、terminal update 与 Outbox 同事务。
9. `product_workflow.WorkflowRunCompleted`
   - terminal `UpdateRun` 与 Outbox 同事务。
   - notification / Git report 保持提交后执行，不放进事务。
10. `agent_runtime.RunCompleted`
    - completed run、assistant message 与 Outbox 同事务。
    - LLM 流式网络调用仍在事务外。

## 仓储事务适配

仅将上述发布链实际参与的读写切换为 `postgres.DB(ctx, fallback)`：

- identity member；
- tenant workspace / membership / organization membership / usage；
- workspace project / project member / secret；
- agent aggregate / versions；
- delivery build / deployment run；
- workflow run；
- product-workflow run / workflow lookup；
- agent-runtime run / message / conversation timestamp。

原本需要多表原子性的 Agent 和 Deployment 仓储改用可嵌套复用的
`postgres.Transactor`，避免 GORM 内层另开事务。

## Event codec 注册

owner module 在暴露 handler / route 前注册 codec，并传播重复或非法注册错误：

- identity；
- tenant (`WorkspaceCreated`)；
- workspace_app (`ProjectCreated`, `SecretUpdated`)；
- agent；
- delivery；
- workflow；
- product_workflow；
- agent_runtime。

共享 integration 注册拆分为按 owner 注册，同时保留组合 `Register` 入口。

## RED / GREEN

### RED

- `TestRegisterMember...` 首次运行因构造器尚未注入 `postgres.Transactor` 编译失败，
  证明旧 handler 无法建立所需事务边界。
- 基线代码盘点确认 10 条链均为“先写业务、后事务外 Publish”，其中多处使用
  `_ = bus.Publish`，Outbox 失败不能回滚先前业务写入。

### GREEN

真实 SQLite + 生产仓储 / 真实 Outbox 覆盖：

- member + `identity.MemberRegistered`：失败双回滚、成功双提交并校验事件类型；
- workspace + 两类 membership + `integration.WorkspaceCreated`：失败四项均不存在，
  成功各写一条；
- project + lead membership + `workspace_app.ProjectCreated`：失败三项均不存在，
  成功各写一条；
- secret update + `integration.SecretUpdated`：失败保留旧密文且无 Outbox，成功同时更新；
- agent publish + `agent.AgentPublished`：失败保持 draft 且无 Outbox，成功变为
  published 且有一条 Outbox；
- build terminal + `delivery.BuildCompleted`：失败回滚 terminal update，成功提交
  terminal update 与一条 Outbox；
- deployment terminal + `delivery.DeployCompleted`：callback 失败回滚 run update、
  deployment event 与 Outbox；
- workflow run + `workflow.WorkflowRunCompleted`：失败不留 run / Outbox，成功各一条；
- product workflow terminal + `product_workflow.WorkflowRunCompleted`：失败保持 pending，
  成功提交 success 与 Outbox；
- completed run + assistant message + `agent_runtime.RunCompleted`：失败保持 running 且
  不留 assistant / Outbox，成功三项同时提交。

## 验证

- `go test ./internal/modules/identity/... ./internal/modules/tenant/... ./internal/modules/workspace_app/... ./internal/modules/agent/... -count=1`
- `go test ./internal/modules/workflow/... ./internal/modules/agent_runtime/... ./internal/modules/product_workflow/... -count=1`
- `go test ./internal/modules/delivery/... -count=1`
- `go test ./internal/modules/... -count=1 -timeout 300s`
- `go test ./internal/platform/messaging/... ./internal/platform/postgres/... -count=1 -timeout 300s`
- `go build -o .tmp/task8-server.exe ./cmd/server`
- `rg "_ = .*\\.Publish\\(" backend/internal`：无匹配。

额外执行 `go test ./... -count=1 -timeout 600s` 时，Task 8 相关包通过，但全量命令
被工作区既有 Kubernetes plugin 改动的两个失败阻断：

- bundled descriptor 缺少 `backend/internal/plugins/kubernetes/diagnosis/agent.go`；
- stream recording 的 stdout/stderr 合并结果与既有断言不一致。

这两个失败位于用户明确排除的 plugin 工作区，未在 Task 8 中修改。

## 提交边界

只暂存本报告、上述 publisher / owner module / 参与仓储、必要构造器调用点和测试。
不暂存或覆盖 plugin、frontend、progress、Task 5 报告、日志及生成物。

## 已知边界

- `inmemory` 仍同步执行订阅者；事务包装正确，但其订阅副作用可靠化属于 Task 9/10。
- PostgreSQL 与外部系统不形成分布式事务；本任务没有把外部操作提前迁入
  Operation Outbox。
- product-workflow 的通知和 Git 状态报告保持提交后语义，避免事务内网络 I/O。
- workspace / project / secret 测试使用 SQLite 上的事务感知 GORM repository adapter
  隔离无关 schema；其余路径直接使用生产 persistence repository。

## 审查修复补充

- Agent `CreateAgent` 只在存在领域事件时调用 `Publish`；无事件且未配置 Bus 时仍可在事务内成功保存。
- 保留既有构造约束：存在待发布事件时必须配置 Bus；测试覆盖 nil Bus 的该约束。既有真实 SQLite 测试继续证明发布失败会回滚 Agent 状态。
- Deployment 成功链测试注册真实 delivery codec，证明 terminal run、deployment event 与恰好一条 `delivery.DeployCompleted` Outbox 同时提交，并校验 aggregate 与关键 payload；原失败回滚测试保留。

### 本轮 RED / GREEN

- RED：`TestCreateAgentWithNoDomainEventsDoesNotRequireBus` 在无事件时因 nil Bus 解引用 panic。
- GREEN：增加空事件短路后，Agent command 测试通过。
- RED：Deployment 成功链在未注册 codec 时返回 `event descriptor not registered for "delivery.DeployCompleted"`。
- GREEN：通过 `deliveryevent.Register` 注册真实 codec 后，成功提交测试及原失败回滚测试通过。
