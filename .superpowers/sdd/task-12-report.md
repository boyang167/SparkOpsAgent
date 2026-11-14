# Task 12 实施报告

## 范围

- 为 Organization Owner 动态补种 `org:file:admin`；Organization Admin、Member 与所有 Workspace 默认角色均不获得该组织级权限，既有租户无需一次性数据迁移即可幂等生效。
- 新增文件采纳命令与 `POST /api/v1/filesystem/nodes/{nodeId}/adopt`：仅支持活动文件，要求源 READ 与目标 Organization Namespace 父目录 WRITE，同组织复用已校验的不可变 Blob，创建独立 Node/Revision/owner，按目标 Namespace 仅计一次配额，不复制源 Grant。
- 新增连续迁移 `000087` 的 `filesystem.file_lineage`，包含源/目标 Namespace、Node、Revision、关系、命令幂等键、创建人及时间；PostgreSQL down 在存在活动 lineage 时拒绝回滚，SQLite fallback 同步建表、FK、unique 与索引。
- 新增组织级 bounded cursor 待审批接口、contracts/upstream/BFF/web-sdk 与审批面板；管理入口仅由 `org:file:admin` 控制，普通用户不挂载面板也不请求管理 API。
- 审批批准/拒绝使用真实接口与稳定 Idempotency-Key；拒绝原因强制 1–1000 字符，提交期间禁用，完成后刷新列表与计数。
- 补齐 governed grant revoke DELETE 路由与前端 transport/BFF，并增强 create/revoke 审计。
- 分享申请、批准、拒绝、撤销、敏感读取和采纳成功审计写入 namespace/node/principal before-after/reason/trace/action；沿用请求 trace，缺失时保留空值而不伪造。失败路径不写成功审计。
- AuditWorkspace 增加 Namespace、Node、Principal、Action 与日期筛选，查询继续按 Organization、Workspace 权限边界执行。

## TDD 记录

- RED：Organization Owner 缺少 `org:file:admin`；采纳 API、lineage 与幂等行为不存在；审批面板不存在。
- GREEN：新增权限、采纳命令与测试、审批面板与测试，并完成路由/持久化/契约接线。
- 关键采纳测试覆盖：源 READ、目标 WRITE、跨组织拒绝、目录拒绝、活动目录要求、名称冲突、Blob 复用、分类保真、配额、lineage、审计与相同命令只创建一次。

## 首轮审查修复

- Fallback：`fileLineageRow` 已纳入 AutoMigrate，并确保建表早于 ALTER；fresh/partial SQLite fallback、正式 `000087` 静态与条件 PostgreSQL up/down 测试覆盖复合 FK、unique 和活动 lineage down 门禁。
- Adoption：canonical identity 绑定 actor/org/workspace/source/target/name；完成重放早于可变授权；首次执行在事务内重新锁定并验证 source/current revision/blob/target parent/权限。Node、Revision、Blob ref、quota、lineage、command、audit 及 ReliableBus outbox 均在一个事务中。
- 普通目录浏览由 `Node.ParentID` 定义；`FileLink` 是 artifact/外部关联语义，不是目录 child link，因此未机械创建重复 link。持久化测试证明 adopted node 可由目标 namespace children API 浏览。
- Lineage：源/目标 Node 与 Namespace、Revision 与 Node 均使用复合 FK；错误组合被数据库拒绝。采纳不复制源 grants。
- Bootstrap：Workspace role 持久化会过滤并清理所有 `org:*` 权限；Organization Owner 的持久成员角色经权威组织权限投影获得 `org:file:admin`，Admin/Member 与普通 Workspace Owner 不获得。真实 tenant repository 条件集成测试在配置 PostgreSQL 时执行。
- Audit：新增仅 `org:file:admin` 可用的组织级 audit API/BFF/SDK，组织事件不伪造 workspace；namespace/node/principal/action/date 在数据库层过滤。分享 request/approve/reject、grant create/revoke、sensitive read、adoption 的结构化字段矩阵均有断言。
- Approval：组织 pending API 返回独立 COUNT 的 `total_count`；面板消费 `next_cursor`、去重追加、隔离 stale response，审批后重置首屏/cursor/count。测试覆盖 51 条两页。
- Revoke：完成重放前校验 actor/org/workspace/namespace/node/grant/payload 的 canonical identity；同 key 换身份或路径冲突，成功后 grant 状态变化不破坏稳定重放。

## 复审收尾修复

- 审计入口：新增权威 `resolveFilesystemAuditMode`。`org:file:admin` 映射 Organization audit，`file:admin` 映射 Workspace audit；同时拥有时优先组织级。两者都没有时隐藏审计 tab。`AuditWorkspace` 接收显式 mode 并再次核验对应权限，`tab=audit` 对 Tenant Owner 保持有效。
- Lineage 组织一致性：`000087` 在建表前创建 `namespaces(id, organization_id)` 唯一索引，并为 source/target namespace 增加包含 `organization_id` 的复合 FK；down 在安全门禁通过后先删 lineage 表、再删其依赖的唯一索引。
- Fallback：PostgreSQL fallback 同步唯一索引与复合 FK；SQLite fresh schema 同步约束，并会事务性重建缺少组织复合 FK 的既有 lineage 表，非法历史组合会使升级失败而不是被静默保留。
- PostgreSQL 条件集成测试会插入合法 lineage，验证带活动数据的 down 明确失败且表/数据保留，清理 lineage 后 down 成功；同时覆盖 cross-org 组合拒绝。
- TDD RED：组织 Owner 审计 tab 被隐藏、audit mode helper 不存在、cross-org lineage 可写入、partial SQLite lineage 缺少组织 FK。
- TDD GREEN：前端权限/Workbench/accessibility 聚焦测试及 migration/fallback 聚焦测试通过。

## 验证

- `go test ./internal/modules/governance/application/command -run Bootstrap -count=1`
- `go test ./internal/modules/filesystem/application/command -run Adopt -count=1`
- `go test ./internal/modules/filesystem/... -count=1`
- `go test ./internal/platform/authz ./internal/platform/audit ./internal/modules/governance/application/command ./internal/modules/tenant/application/query ./migrations ./api/openapi -count=1`
- `go vet ./internal/platform/authz ./internal/platform/audit ./internal/modules/filesystem/... ./internal/modules/governance/application/command ./internal/modules/tenant/application/query`
- `pnpm vitest run apps/web/src/modules/filesystem packages/contracts/src/filesystem packages/upstream/src/filesystem.test.ts packages/bff-modules/src/filesystem/index.test.ts packages/web-sdk/src/filesystem.test.ts apps/web/app/api/bff/v1/filesystem/governed-routes.test.ts apps/web/app/api/bff/v1/workspaces/[wsId]/filesystem-forwarding.test.ts`（32 files / 243 tests）
- `pnpm typecheck`（6 packages）
- OpenAPI 本地引用、运行时路由一致性及 Idempotency 响应矩阵测试通过。
- 复审收尾：`pnpm vitest run apps/web/src/modules/filesystem`（26 files / 198 tests）；`pnpm typecheck`；`go test ./migrations -run FileLineage -count=1`；lineage fallback 聚焦测试；`go test ./internal/modules/filesystem/... -count=1`；受影响包 `go vet`。

`DATABASE_DSN` 未设置，因此真实 PostgreSQL migration up/down 与 tenant repository 条件集成测试已明确 skip，未声称通过；SQLite migration/fallback 与持久化测试已通过。

## 说明

- 当前文件模型没有通用 metadata 字段；采纳完整保留现有可表达的 classification、MIME、size、Blob hash/Revision 引用，未虚构新的 metadata 存储。
- 未实施 Task 13 发布、版本或发布流水线工作。
