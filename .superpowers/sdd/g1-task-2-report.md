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
- `SaveRevision` 在单一事务中保存 blob、revision、replica；`SetCurrentRevision` 只允许同一 node 的 revision，并保持 current revision 只设置一次。
- 创建子节点显式校验 parent 与 namespace 一致，保持 Task 1 的跨 Namespace 约束语义。

## 测试与输出

- `go test ./internal/modules/filesystem/infrastructure/persistence ./internal/bootstrap/... -count=1`：PASS。
- `go test ./internal/modules/filesystem/... -count=1`：PASS。
- `git diff --check`：PASS（仅有换行转换提示）。
- lint：PASS，无诊断。

## 重启

已执行 `.\scripts\dev-restart.ps1 -Backend`，Backend 已启动，PID 为 `24008`，监听 `http://localhost:8080`。

## Commit

`669bd1c feat(filesystem): add kernel persistence`

## Concerns

- SQLite 运行时没有 PostgreSQL schema/FK 能力，测试 AutoMigrate 使用 attached `filesystem` 数据库与显式唯一索引；PostgreSQL 仍由 schema 与 SQL migration 提供完整 FK。
- 当前 Task 2 仅映射 brief 所需 kernel 仓储表，未实现后续 Gate 的 quota、grant、link 功能。
