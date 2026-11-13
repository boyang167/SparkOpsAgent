# 文件系统备份与恢复 Runbook

## 适用范围

适用于 SparkOps `filesystem` PostgreSQL 元数据、Provider Blob/Replica 和搜索投影的灾难恢复。目标为 RPO ≤15 分钟、RTO ≤2 小时；目标是否达成必须由现场演练时间戳证明。

## 前置条件

- PostgreSQL 已启用连续归档，WAL 归档延迟有监控，基础备份可读取。
- Provider 已启用版本控制、快照或等价不可变备份；恢复操作不会覆盖当前对象。
- 值班人员可暂停 API 写请求、Transfer Gateway、SFTP、Worker、Reconciler 和迁移任务。
- 已记录目标环境、事故编号、故障时间（UTC）和恢复负责人。
- 所有凭据从 Secret 引用获取；不得复制到命令历史、演练记录或本文。

## 固定恢复顺序

### 1. Freeze writes

1. 宣布维护窗口并记录 `failure_time`。
2. 拒绝文件写 API 和新 Transfer Session，停止 Transfer Gateway/SFTP 写入口。
3. 暂停 Worker、Reconciler、GC、Lifecycle 和 Knowledge migration。
4. 保持只读流量关闭，直到抽样校验完成；避免暴露数据库与 Blob 不一致状态。
5. 记录各工作负载副本数和最后消费的 Outbox/Inbox 位点。

### 2. Restore PostgreSQL to PITR target

1. 选择不晚于故障前最后一致提交的 `pitr_target_time`。
2. 从最近基础备份恢复到隔离 PostgreSQL 实例，回放 WAL 到目标时间并停止恢复。
3. 记录 `latest_recovered_commit`，不得以 WAL 文件时间代替已提交事务时间。
4. 禁止应用连接旧主库；切换 DSN 前保留旧实例只读快照。

### 3. Validate schema version

1. 运行 schema migration 的只读检查，确认最高迁移版本与待发布镜像一致。
2. 检查 `filesystem` 的 Namespace、Node、Revision、Blob、Replica、Transfer、Quota、Outbox/Inbox 表和约束。
3. 检查每个 Workspace 仅有一个 Namespace、Revision 引用的 Blob 存在、配额预留与 Session 状态匹配。
4. 若 schema 版本领先于应用镜像，停止恢复并升级镜像；禁止执行破坏性降级。

### 4. Reconcile Blob/Replica

1. 先以 dry-run/只读模式扫描 Provider，比较数据库 Blob/Replica 与对象版本。
2. 将“对象存在但数据库无引用”放入隔离清单，不立即删除。
3. 对“数据库引用但对象缺失”优先从版本/副本恢复，并校验大小和 SHA-256。
4. 启动一个 Reconciler 实例，观察租约、积压和副本不一致指标收敛；稳定后恢复正常副本数。

### 5. Rebuild search projection

1. 清空或创建新的搜索投影索引，不修改文件事实表。
2. 从可见 Node/Revision 重建投影。
3. 抽查搜索结果，并确认每条结果返回前仍执行实时授权。

### 6. Verify sample checksums

1. 按组织、Workspace、Provider、分类和文件大小分层抽样。
2. 对样本执行完整读取，比较 Revision/Blob 记录的大小与 SHA-256。
3. Restricted 数据必须经受控主机校验，日志只记录 ID、结果和 trace ID。
4. 任一样本失败时保持冻结，扩大抽样并回到 Blob/Replica 对账步骤。

### 7. Resume workers

1. 先启动消息基础设施，再以单副本启动 Worker。
2. 确认 Inbox 幂等、Outbox 延迟、Quota reservation 和待扫描 Session 正常收敛。
3. 逐步恢复 Worker、Reconciler、Lifecycle、GC 与 Preview/Search 投影任务。

### 8. Resume writes

1. 恢复只读请求并执行上传、下载、重命名和分页列表冒烟测试。
2. 创建一个小文件，确认 Verify/Scan/Commit 后只产生一个 Revision。
3. 恢复 Transfer Gateway、SFTP 和 API 写流量。
4. 记录 `service_restored_time`，持续观察至少两个告警周期。

## RPO / RTO 计算与验收

统一使用 UTC：

```text
RPO = failure_time - latest_recovered_commit
RTO = service_restored_time - failure_time
```

演练记录必须包含原始时间戳、数据源、计算值、抽样校验结果和审批人。只有实测 `RPO ≤15 分钟` 且 `RTO ≤2 小时` 才可标记 PASS；缺少可恢复环境、WAL/Provider 备份或时间证据时标记 SKIP，不可用单元测试代替。

## 回退与升级

- PITR、schema 或校验失败：保持写冻结，断开新库，恢复到上一个已验证快照。
- 对账无法收敛：停止 Reconciler/GC，保留隔离清单和 Provider 版本，升级事故等级。
- 已恢复写流量后发现校验错误：立即再次冻结写入，保存当前数据库与 Provider 快照后重走流程。

## 证据清单

- 事故/演练编号与参与人；
- `failure_time`、`pitr_target_time`、`latest_recovered_commit`、`service_restored_time`；
- PostgreSQL 基础备份、WAL 范围和 Provider 快照/版本标识；
- schema 版本、对账前后计数、搜索重建计数；
- 抽样 File/Revision/Blob/Replica ID 与校验结果；
- RPO/RTO 计算和最终 PASS/FAIL/SKIP。
