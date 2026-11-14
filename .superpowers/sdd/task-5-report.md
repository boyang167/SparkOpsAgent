# Task 5 Report

## 状态

完成统一只读文件治理 API 及审查修复：

- `GET /api/v1/filesystem/namespaces`
- `GET /api/v1/filesystem/namespaces/:namespaceId/children`
- `GET /api/v1/filesystem/shared-with-me`
- `GET /api/v1/filesystem/nodes/:nodeId/effective-access`

旧 `/api/v1/workspaces/:wsId` 路由未变；未实施 Task 6。

## 本轮修复

- Organization root 仅在 root 有 `READ` 时返回；隐藏 root 时 `root_node_id=null`，capabilities 仅合并可见子树入口的有效 actions。
- children 在省略 `parent_id` 时内部使用 namespace root，不要求客户端获知 root；返回第一层节点时清除隐藏 root 的 `parent_id`。
- children 先授权后分页，通过 bounded raw pages 补页并用最后一个已返回的可见节点作为 cursor；hidden ID 不进入响应 cursor。
- SharedWithMe 保留 typed-principal、组织范围、稳定 cursor 的 bounded DB 候选查询，最终可见性/actions 全部调用 Task 4 Authorizer。
- Task 4 Decision 的 `source_grant_ids` 覆盖影响决策的 Allow/Deny grant；祖先 Deny 可压过子节点 Allow。
- Governance 仓储/domain 错误仅明确 not-found/forbidden 映射为 404/403，其余数据库和未知错误映射为 500。
- 移除了 Task 5 中 storage provider/pool PATCH/DELETE 的 OpenAPI 文档扩展，仅保留统一只读 API 契约。

## TDD

### RED

- SharedWithMe 构造器缺少 Authorizer，查询测试编译失败。
- unauthorized parent 测试证明旧测试未实际调用 Authorizer。
- 授权后分页返回 3 项而非 requested limit 2。
- repository failure 被错误映射为 404。
- 隐藏 root 的 children 请求返回 404，并在修复后继续暴露 `parent_id`。
- root 无 READ 但有 ADMIN 时，namespace capabilities 错误包含 root ADMIN。

### GREEN

- `go test ./internal/modules/filesystem/application/query -count=1`
- `go test ./internal/modules/filesystem/application/authorization -count=1`
- `go test ./internal/modules/filesystem/interface/http -count=1`
- `go test ./internal/modules/filesystem/... -count=1`
- `go test ./api/openapi -count=1`
- `git diff --check`

## Concerns

- OpenAPI 路由一致性测试显式忽略仓库既有但未写入 fragment 的 storage provider/pool PATCH/DELETE 路由；本任务按审查要求没有补写这些非 Task 5 契约。
