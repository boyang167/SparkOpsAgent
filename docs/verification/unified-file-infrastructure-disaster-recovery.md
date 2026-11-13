# Unified File Infrastructure 灾备与迁移演练记录

> 状态：**SKIP（无可用现场恢复环境）**。本记录是可执行证据模板，不把 Runbook、单元测试或配置审查当作 RPO/RTO 与 Knowledge 真库回滚证据。

## 本次判定

- 验收日期：2026-11-13（UTC+8）
- PostgreSQL PITR 目标环境：未提供
- WAL / 基础备份标识：未提供
- Provider 快照或对象版本环境：未提供
- Knowledge 演练 `DATABASE_DSN`：未提供
- 结论：无法执行会改变数据库和存储状态的灾备演练；RPO、RTO 和 Knowledge 真库 rollback 均为 SKIP

## 灾备演练模板

| 字段 | 现场值 |
|---|---|
| 环境 / 集群 | `<required>` |
| 演练编号 | `<required>` |
| 负责人 / 审批人 | `<required>` |
| `failure_time`（UTC） | `<required>` |
| PITR target（UTC） | `<required>` |
| `latest_recovered_commit`（UTC） | `<required>` |
| `service_restored_time`（UTC） | `<required>` |
| PostgreSQL 备份 / WAL 标识 | `<required>` |
| Provider 快照 / 版本标识 | `<required>` |

固定执行顺序：

- [ ] freeze writes
- [ ] restore PostgreSQL to PITR target
- [ ] validate schema version
- [ ] reconcile Blob/Replica
- [ ] rebuild search projection
- [ ] verify sample checksums
- [ ] resume workers
- [ ] resume writes

计算：

```text
RPO = failure_time - latest_recovered_commit
RTO = service_restored_time - failure_time
```

验收：

- [ ] RPO ≤15 分钟 — **SKIP：缺少现场时间戳**
- [ ] RTO ≤2 小时 — **SKIP：缺少现场时间戳**
- [ ] Blob/Replica 对账收敛 — **SKIP：缺少恢复后的数据库与 Provider**
- [ ] 分层抽样 SHA-256 全部一致 — **SKIP：缺少 Provider 恢复环境**
- [ ] 恢复写流量后告警保持正常 — **SKIP：缺少目标监控环境**

## Knowledge rollback 演练模板

现场依次归档以下命令的输出：

```powershell
go run ./cmd/migrate-knowledge-inventory
go run ./cmd/migrate-knowledge-precopy -batch-size 100
# 按 inventory 和输出继续使用 -after-id，直到全部批次完成
go run ./cmd/migrate-knowledge-switch
go run ./cmd/migrate-knowledge-switch -rollback
go run ./cmd/migrate-knowledge-switch
```

验收：

- [ ] inventory / precopy 行数 100% 匹配 — **SKIP：未提供 `DATABASE_DSN`**
- [ ] precopy 字节数与 SHA-256 100% 匹配 — **SKIP**
- [ ] rollback 后全部 legacy content 可读 — **SKIP**
- [ ] rollback 不删除 legacy content 或统一文件 — **SKIP**
- [ ] 再 switch 后抽样 SHA-256 不变 — **SKIP**
- [ ] 全程无重复 File/Revision、无跨 Workspace/组织泄漏 — **SKIP**

## 已完成的准备证据

- 恢复操作顺序、停止条件、校验、回退和证据要求：`docs/runbooks/filesystem-backup-restore.md`
- Knowledge inventory、precopy、switch、rollback 与再 switch：`docs/runbooks/filesystem-knowledge-migration.md`
- Knowledge 迁移逻辑的夹具测试已在 G4 验收中记录；该证据不替代真库演练。

## 将 SKIP 转为 PASS 的条件

1. 提供隔离的等价容量环境、可恢复 PostgreSQL 备份/WAL 和 Provider 版本/快照。
2. 提供受控的 Knowledge 演练数据库与操作窗口。
3. 逐项填写原始 UTC 时间戳、备份标识、命令输出摘要和抽样校验结果。
4. 计算并复核 RPO/RTO；所有固定验收项通过后由负责人和审批人签署。
