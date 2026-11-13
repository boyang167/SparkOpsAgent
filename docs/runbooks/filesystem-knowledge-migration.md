# Knowledge 统一文件迁移与回滚 Runbook

## 目标与边界

将 Knowledge legacy content 迁移为统一文件基础设施中的 File ID，同时保留可回滚的 legacy content。迁移期间禁止删除 legacy 内容；只有 inventory、precopy、校验、switch 和观察窗口全部通过后，才能另行批准清理。

## 前置条件

- 已备份 PostgreSQL，并验证可恢复。
- 目标环境已配置 `DATABASE_DSN`，且操作账号仅拥有所需 schema 权限。
- 文件 Provider、Scanner、Worker、Outbox/Inbox 和 Reconciler 健康。
- 当前应用版本同时支持 legacy content 和 file_id 双读/切换。
- 报告目录为受控位置，不包含 DSN、Token、对象 key 或正文。

## 迁移顺序

### 1. Inventory

```powershell
cd backend
go run ./cmd/migrate-knowledge-inventory
```

归档 JSON 报告并记录：

- legacy document 总数、空内容数和总字节；
- 已映射、待迁移、校验失败和异常关联计数；
- 每个 Workspace 的统计，不记录正文。

若报告存在跨 Workspace/组织关联、不可解析文档或计数不稳定，停止迁移。

### 2. Precopy

```powershell
go run ./cmd/migrate-knowledge-precopy -batch-size 100
```

按输出和 inventory 继续使用 `-after-id <document-id>` 运行后续批次，直到待迁移计数为零；首次正式写入前可加 `-dry-run`。命令必须可重入。重复执行不得创建重复 File/Revision/Blob；每条映射记录 legacy 行标识、File ID、字节数和 SHA-256。保持应用读取 legacy content。

### 3. Verify

至少验证：

1. `legacy document count = verified mapping count + 明确豁免 count`；
2. 每条映射的字节数和 SHA-256 相同；
3. Knowledge 与 Workspace/组织关联保持不变；
4. 统一文件授权拒绝跨 Workspace/组织读取；
5. 抽样文件可从 Provider 完整读取并通过 Scanner 状态检查。

任何不匹配都必须修复后重新运行 precopy 和 verify，不允许带差异 switch。

### 4. Switch

```powershell
go run ./cmd/migrate-knowledge-switch
```

Switch 前暂停 Knowledge 写入，确认无运行中的迁移任务。Switch 后：

- 新读路径只通过 File ID；
- 新写入进入统一文件基础设施；
- legacy content 保留不变；
- 搜索、Agent 检索和知识库列表执行冒烟测试。

### 5. Observe

至少观察一个约定发布窗口，监控 Knowledge 读取错误、Provider 错误、文件授权拒绝、Outbox 延迟和 Blob/Replica 不一致。观察期内禁止清理 legacy content。

## 回滚

当读取错误、校验不一致、授权异常或依赖不可用时：

1. 冻结 Knowledge 写入和相关 Worker；
2. 记录回滚前映射计数与抽样 SHA-256；
3. 执行：

```powershell
go run ./cmd/migrate-knowledge-switch -rollback
```

4. 验证全部文档可从 legacy content 读取；
5. 验证回滚未删除 File、Revision、Blob 或 legacy content；
6. 修复问题后重跑 inventory → precopy → verify；
7. 再次执行 switch，验证同一抽样的 SHA-256 不变。

具体参数以命令 `-help` 输出为准；若当前构建不支持 `-rollback`，停止演练并标记 FAIL，不直接修改数据库开关。

## 验收与证据

PASS 必须同时具备：

- 真 PostgreSQL 环境连续执行 inventory、precopy、switch、rollback、switch；
- 行数与 SHA-256 100% 匹配（明确豁免需逐项批准）；
- rollback 后所有 legacy 文档可读；
- 再次 switch 后 SHA-256 不变；
- 全程无跨 Workspace/组织泄漏，无重复 File/Revision。

缺少数据库、Provider、凭据或发布窗口时标记 SKIP。单元测试只能证明命令逻辑，不是生产迁移或回滚证据。
