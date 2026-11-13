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
