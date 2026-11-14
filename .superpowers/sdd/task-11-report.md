# Task 11 复审修复报告

## 范围

- 基线：`185d2d3339537cc8f266527a461117e39a54f0aa`
- 修复起点：`80607d861f3fec59babc2b449b69a3baf55ffddd`
- 仅处理 Task 11 的 shared reshare 服务端防御、治理分页、OpenAPI 契约、授权测试桩和本报告清理。

## TDD 记录

- RED：OpenAPI 契约测试确认 `ArtifactPage` 错误 required `truncated`，治理页缺少 `next_cursor` required 和 cursor query。
- RED：治理 limit 上限测试确认请求 `limit=999` 被错误拒绝，而非钳制为 200。
- RED：前端 contracts 测试确认治理 limit 未钳制到 200。
- GREEN：上述聚焦测试通过；既有 reshare proof、跨节点 cursor、分页去重、BFF 转发、Drawer stale 防护测试通过。

## 修复结果

1. Shared reshare proof
   - shared preview/submit 使用 `reshare` 与 `source_grant_ids` 明示 proof。
   - `SharedWithMePanel` 仅提交当前投影中 `ALLOW + SHARE` 的 source grant IDs。
   - 后端对每个 proof grant 权威校验：目标节点或可继承祖先、同组织、`ALLOW`、`SHARE`、未撤销、未过期，并命中当前 actor 解析出的 typed principal。
   - owner、source namespace workspace admin、organization admin 保持独立 SHARE authority，不要求 proof。
   - preview 与 submit 共用同一验证；submit 在事务内再次验证，避免校验与写入窗口。

2. 治理 cursor pagination
   - grants 使用稳定 grant ID 顺序；share requests 使用稳定 `(created_at, id)` 逆序。
   - 默认 50，最大 200，超限钳制到 200；数据库按 cursor 与 `limit + 1` 查询，不再全量读取。
   - AES-GCM cursor 绑定 organization、node 和 resource kind；篡改或跨节点/跨 kind 重放返回 400。
   - upstream、BFF、web-sdk、contracts、Next routes 完整转发 `limit/cursor`。
   - Drawer 支持加载更多、去重、取消旧请求并忽略 node/scope 切换后的 stale 响应。

3. OpenAPI 与测试
   - `ArtifactPage.required` 修正为 `items + next_cursor`。
   - `ResourceGrantPage` 与 `ShareRequestPage` required 对齐真实 `items + next_cursor + truncated` 响应。
   - 治理 GET 文档补充 cursor query；typed share request 补充 reshare proof 字段。
   - authorization service 测试桩按真实 `(workspaceID, actorID)` 参数判断，覆盖当前 workspace admin 但 source workspace 非 admin 时不得越权。

## 验证

- `go test ./internal/modules/filesystem/... ./api/openapi -count=1`：通过。
- 前端聚焦 contracts / upstream / BFF / web-sdk / Next routes / modal / drawer / shared / accessibility：通过。
- `pnpm typecheck`：通过。
- `git diff --check`：通过。

## 环境限制

- 当前环境未设置 `DATABASE_DSN`，未执行依赖外部 PostgreSQL 的真实数据库集成测试；SQLite repository 边界测试已执行。

## 终审 Important 修复（2026-11-15）

### TDD 记录

- RED：后端缺少与前端一致的 canonical payload 函数；数组重排无法稳定命中同一请求哈希。
- RED：已完成 reshare 在 proof 失效后先执行实时授权，导致合法幂等重放被拒绝。
- RED：前端 expiry 未规范到 UTC 毫秒格式，同一时刻的不同时区表示生成不同 fingerprint。
- GREEN：canonical、完成态重放、首次失效 proof、actor/scope/payload 冲突和前端 fingerprint 测试通过。

### 修复结果

1. 共享请求 canonical payload
   - 后端与前端统一字段顺序和默认语义：`principal_type`、排序后的 `principal_ids/actions`、默认 `ALLOW`、inherit、UTC 毫秒 expiry/null、reshare、排序后的 `source_grant_ids`。
   - 所有数组均复制后排序，不修改调用方输入；nil/空 source grants 统一编码为 `[]`。
   - 服务端安全哈希在 canonical payload 外继续绑定 operation、request、actor、organization、workspace、namespace、node 和 classification。

2. 完成态幂等重放
   - Submit 先按 command ID 识别已完成请求，并校验 actor/org/workspace/node、canonical hash 和确定性 request ID。
   - 同 actor、同 scope、同集合 payload 的完成态请求直接返回稳定结果，不再受后续 SHARE authority、proof 撤销或过期影响。
   - 新命令先 claim，再在同一事务中实时校验 SHARE authority、reshare proof 和策略后写入。
   - 不同 actor、scope 或 payload 仍返回幂等冲突；首次提交 proof 无效仍拒绝。
   - Preview 保持每次实时验证。

### 验证

- `go test ./internal/modules/filesystem/application/command -count=1`：通过。
- `go test ./internal/modules/filesystem/... -count=1`：通过。
- 前端 `share-file-form` 与 `ShareFileModal` canonical 相关测试：通过。
- `go vet ./internal/modules/filesystem/...`：通过。
