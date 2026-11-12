# G1 Task 2：仓储与 AutoMigrate

## RED/GREEN

- RED：先编写 `filesystem_repo_test.go` 契约测试，首次运行
  `go test ./internal/modules/filesystem/infrastructure/persistence -run TestCreateChildRejectsDuplicateNormalizedName -count=1`
  因仓储包不存在而失败。
- GREEN：实现领域错误、仓储接口、GORM 映射、SQLite 语义索引、仓储及 bootstrap 注册后，契约测试通过。

## 映射、错误转换与事务边界

- `filesystem.namespaces`、`nodes`、`blobs`、`file_revisions`、`storage_replicas` 映射到独立 persistence row，FileRevision 只通过 Getter 读取并写入，未暴露 GORM 类型。
- PostgreSQL unique/duplicate/23505 错误转换为 `domain.ErrNodeNameConflict`；记录不存在转换为 `domain.ErrNodeNotFound`（namespace not found 转为 `domain.ErrNamespaceNotFound`）。
- SQLite 测试使用根目录与子目录两组显式 partial unique index，避免 SQLite 对 NULL 唯一性的差异；仓储创建前也执行语义预检查。
- `SaveRevision` 在单一事务中保存 blob、revision、replica；`SetCurrentRevision` 只允许同一 node 的 revision，并支持从一个合法 revision 原子切换到另一个。
- 创建子节点显式校验 parent 与 namespace 一致，保持 Task 1 的跨 Namespace 约束语义。

## 测试与输出

- `go test ./internal/modules/filesystem/infrastructure/persistence ./internal/bootstrap/... -count=1`：PASS。
- `go test ./internal/modules/filesystem/... -count=1`：PASS。
- `git diff --check`：PASS（仅有换行转换提示）。
- lint：PASS，无诊断。

## 重启

已执行 `.\scripts\dev-restart.ps1 -Backend`，Backend 已启动，PID 为 `24008`，监听 `http://localhost:8080`。

## Commit

`3a69cc3 feat(filesystem): add kernel persistence`

## Concerns

- SQLite 运行时没有 PostgreSQL schema/FK 能力，测试 AutoMigrate 使用 attached `filesystem` 数据库与显式唯一索引；PostgreSQL 仍由 schema 与 SQL migration 提供完整 FK。
- 当前 Task 2 仅映射 brief 所需 kernel 仓储表，未实现后续 Gate 的 quota、grant、link 功能。

## 审查修复 RED/GREEN

- 修复 commit：`241dcc5 fix(filesystem): address persistence review findings`
- RED：新增合法 revision 切换、跨 node revision、非法 parent、SaveRevision 冲突回滚测试；首次运行因稳定 sentinel 尚未定义而编译失败。
- GREEN：增加 revision/parent/blob/replica/persistence 稳定错误；`SetCurrentRevision` 先分别确认 node、revision 及归属，再在事务内允许原子切换。
- RED：新增 PG constraint 精确映射、ID 冲突不误报名称冲突、driver 错误不泄漏测试；迁移失败包装测试先观察到裸 `sql: database is closed`。
- GREEN：按 `*pgconn.PgError` 的 Code/ConstraintName 与 SQLite 约束/列精确映射；unknown GORM/driver 错误统一包装 `domain.ErrPersistence`；SaveRevision 对 blob/revision/replica 做明确幂等比较，冲突回滚。
- RED/GREEN：新增 SQLite cursor 独占边界、root/nested 重名、MoveNode 跨 namespace/非目录 parent 测试；实现 parent 存在性、目录类型、namespace 一致性及 NFC/trim/lower 规范化后通过。
- RED/GREEN：新增 SQL migration 已应用的 AutoMigrate guard 与失败传播测试；`ShouldRunAutoMigrate` 为 false 时跳过，迁移失败包装为稳定错误。

## 审查修复验证

- `go test ./internal/modules/filesystem/infrastructure/persistence -count=1`：PASS。
- `go test ./internal/modules/filesystem/infrastructure/persistence ./internal/bootstrap/... -count=1`：PASS。
- `go test ./internal/modules/filesystem/... -count=1`：PASS。
- `go test ./migrations/... -count=1`：PASS。
- `git diff --check`：PASS（仅换行转换提示）。
- lint：PASS，无诊断。
- Backend 已重启，PID 为 `10332`。
- 覆盖确认：revision 切换/归属、parent 校验、root/nested 重名、SaveRevision 原子回滚与精确冲突、cursor 边界、unique 映射、AutoMigrate guard/失败传播均已在上方 RED/GREEN 与本节命令结果中记录。

## 审查修复 Concerns

- 本机无 PostgreSQL DSN，未宣称集成验证；PG 错误转换使用构造的 `*pgconn.PgError` 单测覆盖。
- SQLite 使用 attached `filesystem` schema 并启用 `foreign_keys=1`；SQLite 不支持跨 attached database FK，测试 helper 保留该局限，SQLite DDL 已尽可能声明本 schema FK 与唯一约束。
- 本修复仍未实现后续 Gate 功能。

## 复审 Important/Minor 修复 RED/GREEN

- RED：表驱动 PG 错误映射测试加入 23503/23514 与相似 constraint 名，先暴露非 23505 被误报 conflict；GREEN：仅 `Code == 23505` 且精确 constraint 名映射，其余稳定返回 `ErrPersistence`。
- RED：revision/blob、replica/blob 不匹配测试断言数据库无变化；GREEN：SaveRevision 在事务/写入前校验 blob 关联及 node 存在性，返回稳定领域错误并保持零写入。
- RED：SQLite 非法 node type/state、负数 size/revision、非法 root/current 归属写入及 `PRAGMA foreign_key_check` 测试；GREEN：CREATE TABLE 内补复合循环 FK、CHECK 与外键实际拒绝验证，保持 `foreign_keys=1`。
- RED：CreateNode file parent 与跨 Namespace parent 测试；GREEN：创建子节点前验证 parent 存在、同 namespace 且为 DIRECTORY。

## 本轮验证与提交

- `go test ./internal/modules/filesystem/infrastructure/persistence -count=1`：已通过本轮回归。
- `go test ./internal/modules/filesystem/infrastructure/persistence ./internal/bootstrap/... -count=1`：PASS。
- `go test ./internal/modules/filesystem/... -count=1`：PASS。
- `go test ./migrations/... -count=1`：PASS。
- `git diff --check`：PASS（仅换行转换提示）；lint：PASS，无诊断。
- Backend 已重启，PID 为 `24884`。
- 本轮修复 commit：`66bbfaa fix(filesystem): tighten kernel invariants`。

## 本轮 Concerns

- PostgreSQL 真实 DSN 仍不可用，PG 错误路径继续由 `*pgconn.PgError` 构造测试覆盖。
- SQLite attached schema 不支持跨 attached database FK；本轮未关闭外键，并在 attached schema 内声明循环复合 FK 与实际非法写入测试。
