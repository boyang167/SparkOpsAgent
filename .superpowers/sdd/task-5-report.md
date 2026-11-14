# Task 5 Report

## 状态

完成统一 Namespace、与我共享、有效权限查询 API 及三轮审查修复；旧 `/api/v1/workspaces/:wsId` 路由未改，未实施 Task 6。

## 最终实现

- Organization root 仅在 root 有 `READ` 时公开；隐藏 root 时 `root_node_id=null`，省略 `parent_id` 的 children 查询由服务端从 root 开始过滤。
- children 在授权后补页，cursor 只取最后一个已返回的可见节点。
- SharedWithMe 先以 typed principals 做稳定、有界 DB 候选查询，再按 64 节点 chunk 调用 Task 4 批量 Authorizer；每次请求最多扫描 256 个 raw candidates。
- continuation cursor 为版本化 base64 opaque token；预算分页回归覆盖续页无遗漏、无重复。
- 批量 Authorizer 每个 namespace/chunk 各批量加载一次节点祖先和相关 typed-principal grants，在内存复用 Deny、继承、owner/admin 语义；300 节点回归验证仓储调用固定为 1+1，而非 node×action×depth。
- Allow/Deny 决策来源均进入 `source_grant_ids`；祖先 Deny 压过子节点 Allow。
- Organization namespace 发现同样受 256 扫描预算约束；无法判定时返回 domain budget error/HTTP 503。入口最多 32 个，并返回 `entry_nodes_truncated`；capabilities 仅合并实际返回入口的有效 actions。
- 数据库和未知错误映射 500，仅明确 not-found/forbidden 映射 404/403。
- OpenAPI 记录 opaque cursor、truncation 字段和 budget-exceeded 错误；Task 5 未增加 storage mutation 契约，也未以测试 ignore 掩盖差异。

## TDD

### RED

- 缺少 `AuthorizeMany`/批量仓储接口时查询测试编译失败。
- 300 个候选会逐节点逐 action 授权，无法满足固定 chunk 调用上限。
- SharedWithMe 超过 256 候选时没有 opaque continuation。
- Organization namespace 在扫描预算耗尽时静默遗漏，且没有 503 映射。
- namespace entry 数量和 capabilities 没有固定上限语义。

### GREEN

- query、authorization、HTTP 专项测试通过。
- 完整 `go test ./internal/modules/filesystem/... -count=1` 通过。
- OpenAPI 解析、治理 header、stable error、local refs 等专项测试通过。
- `git diff --check` 通过。

## Concerns

- 无。

## Pre-existing baseline contract repair

- 在指定 Base commit `b094ee35e5648d2c70d94d96d621e057f777ebca` 中，运行时已注册 storage provider/pool PATCH/DELETE 四条路由，但同 commit 的 OpenAPI fragment 与根文档缺少对应 path。
- 复审前单独补齐了这四条既有路由、path parameters 及与当前 handler 一致的请求/响应 schema，并保留 exact router/spec 校验；这属于既有基线契约修复，不归因于 Task 5 统一读取 API 功能。
- RED：`go test ./api/openapi -count=1` 仅因四条路由缺失失败。
- GREEN：完整 OpenAPI、完整 filesystem 与 `git diff --check` 均通过。
