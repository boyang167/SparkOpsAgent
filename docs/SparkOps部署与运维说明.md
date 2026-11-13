# SparkOps 部署与运维说明

## 文件系统可观测性

文件系统 API、Transfer Gateway、Worker 与 Reconciler 必须注册
`backend/internal/modules/filesystem/infrastructure/observability` 中的 Prometheus
collector，并通过各进程的 `/metrics` 端点暴露。生产环境至少采集以下信号：

- 活跃传输、传输字节、操作结果、失败原因及状态耗时；
- 扫描耗时与拒绝数；
- Transfer、Outbox 等队列延迟；
- Provider 错误率与延迟；
- Workspace 配额预留量与已用量；
- 副本不一致、对账积压与 GC 积压。

Helm 默认创建 `PrometheusRule`。阈值位于
`observability.filesystemAlerts`，上线前应按环境容量覆盖默认值。固定告警包括：

- `FilesystemTransferFailureRateHigh`
- `FilesystemScannerUnavailable`
- `FilesystemProviderErrorRateHigh`
- `FilesystemQuotaReservationStuck`
- `FilesystemReconcileBacklogHigh`
- `FilesystemReplicaMismatch`
- `FilesystemOutboxLagHigh`

告警处置时先按 `trace_id` 检索。BFF 必须转发 `X-Trace-Id`，API 响应、API/Worker
结构化日志、Transfer Session 事件和审计事件必须保存同一值；缺失时由 API
入口生成。不得在指标标签中写入 `trace_id`、workspace UUID、文件名或对象 key，
以避免高基数和敏感信息泄漏。

### 基础排障顺序

1. 确认相关 Pod、Scanner、消息队列和 Provider 健康状态。
2. 使用响应中的 `trace_id` 串联 API、Transfer、Worker 和审计记录。
3. 检查队列延迟、配额预留、副本不一致及对账积压。
4. Provider 或 Scanner 恢复后观察告警至少一个 `for` 周期，确认自动收敛。

## 文件系统生产部署

### Workload 与端口

生产环境使用同一个后端镜像，分别运行 `server`、`file-transfer-gateway`、
`file-worker`、`sftp-gateway` 和 `file-reconciler`。API、Web、Transfer Gateway、
Worker、SFTP 与 Reconciler 至少 2 副本；Reconciler 通过数据库租约保持单活。

- API：8080
- Web/BFF：3000
- Transfer Gateway：8082
- SFTP：2022

`values-production.yaml` 为 API、Gateway、Worker 和 Web 配置 HPA，为文件数据面
配置 PDB 与 Topology Spread。上线前必须按实际节点区、资源容量和中断预算复核。

### Provider 与 Secret

v1.0 原生支持 Local/PVC、NFS 和 S3 Compatible。COS、OSS、Azure Blob 需要
额外 Adapter，不应配置成已内置 Provider。

Provider credential 必须先创建为 Kubernetes Secret，再将名称加入
`filesystem.providerSecretRefs`。`filesystemCapabilitySecret` 和
`sftpHostKeyPem` 同样通过 Secret 注入。不得把 credential、私钥、DSN 或 Token
写入 values 文件、ConfigMap、压测报告或日志。

### 网络依赖

默认 NetworkPolicy 只允许 API/Web 访问 Transfer Gateway 的 8082，SFTP 对外
暴露 2022；数据面工作负载出站仅允许 DNS 和配置的依赖端口。默认依赖端口为：

- DNS 53；
- PostgreSQL 5432；
- RabbitMQ 5672；
- Redis 6379；
- S3 Compatible / MinIO 9000；
- ClamAV 3310。

若企业依赖使用其他端口，应显式覆盖
`filesystem.networkPolicy.dependencyPorts`，并在变更审批中记录目的；不要放宽为
任意出站。

### 部署检查

```powershell
make helm-lint
helm template sparkops deployments/helm/sparkops `
  -f deployments/helm/sparkops/values-production.yaml
```

模板检查通过只证明 Chart 可渲染。发布前仍需在目标集群验证副本、HPA、PDB、
Topology Spread、NetworkPolicy、Secret 引用、指标抓取与告警触发。

## Knowledge 迁移与回滚

固定顺序为：

```text
inventory → precopy（可重入批次）→ 行数/SHA-256 校验 → switch
→ 观察窗口 → 必要时 rollback → 修复后再 switch
```

操作步骤、停止条件和证据要求见
`docs/runbooks/filesystem-knowledge-migration.md`。迁移观察窗口结束前不得删除
legacy content。没有真 PostgreSQL 与 Provider 演练证据时，不得声称迁移或
rollback 已通过。

## 备份、恢复与发布

文件系统恢复必须严格按以下顺序执行：

```text
freeze writes
→ restore PostgreSQL to PITR target
→ validate schema version
→ reconcile Blob/Replica
→ rebuild search projection
→ verify sample checksums
→ resume workers
→ resume writes
```

完整步骤见 `docs/runbooks/filesystem-backup-restore.md`。现场记录使用
`docs/verification/unified-file-infrastructure-disaster-recovery.md`，并以 UTC
原始时间戳计算 RPO/RTO。缺少 PITR、Provider 快照或目标环境时应标记 SKIP，
不能以单元测试替代。

容量工具通过 `scripts/run-filesystem-load.ps1` 运行，输出
`artifacts/filesystem-load-report.md` 与 JSON。报告不得提交环境 Token 或含敏感
地址的原始日志；只有目标或等价容量环境报告可作为 20 GiB、200 并发和元数据
分页 SLO 的发布证据。
