# G5 Unified File Infrastructure Production 验收证据

> 状态：**CONDITIONAL / RELEASE BLOCKED**。生产化代码、Helm、测试脚手架、Runbook 和静态验证已完成；缺少目标或等价容量环境，20 GiB、200 并发、Knowledge 真库回滚、告警触发和 RPO/RTO 均诚实标记 SKIP，不能签署正式发布。

## 验收上下文

- 验收日期：2026-11-13（UTC+8）
- 分支：`feat/unified-file-infrastructure`
- 目标环境 URL / Token / Workspace / Provider：未提供
- PostgreSQL PITR / Provider 快照环境：未提供
- Knowledge `DATABASE_DSN`：未提供
- 容量报告：未生成（`artifacts/filesystem-load-report.md` 不存在）

## Fresh 检查

| 检查 | 结果 |
|---|---|
| `go test ./...`（`backend`） | PASS |
| `REQUIRE_INTEGRATION=1 go test -tags=integration ./tests/filesystem/... -count=1` | PASS：consistency、integration、performance（现场用例按环境开关跳过）、security |
| `helm lint deployments/helm/sparkops` | PASS；提示 chart dependency 未 vendored（postgresql、redis、rabbitmq） |
| `go test ./deployments/helm -count=1` | PASS |
| `make test` | SKIP：当前 Windows 环境没有 `make`；已分别执行后端 `go test ./...` 和前端 `pnpm --filter web typecheck` |
| `make helm-lint` | SKIP：当前环境没有 `make`；已直接执行等价 `helm lint deployments/helm/sparkops` 并通过 |

## 发布硬条件

- [x] 生产副本数、HPA、PDB、Topology Spread 已静态验证  
  `values-production.yaml` 和 Helm 模板测试验证 API/Web/Gateway/Worker/SFTP/Reconciler 至少 2 副本，Gateway/Worker HPA、PDB 和 Topology Spread 存在。
- [x] NetworkPolicy 与 Secret 引用已静态验证  
  Helm 测试覆盖 API/Web→Gateway、SFTP、Worker/Reconciler、依赖端口和 `providerSecretRefs`；无目标集群网络探测。
- [x] 跨租户安全测试 0 泄漏（夹具级）  
  integration security suite PASS；跨组织读取返回 403 且响应不包含内容。
- [x] 故障注入后对账可收敛（夹具级）  
  consistency suite 覆盖对象写后 DB 失败、Outbox 暂停、Worker commit 前崩溃、multipart part 丢失、迁移中断和租约丢失。
- [ ] 20 GiB 恢复通过 — **SKIP：未提供 20 GiB fixture 和目标环境**
- [ ] 200 并发传输通过 — **SKIP：未提供目标环境；未生成容量报告**
- [ ] Knowledge 迁移与 rollback 通过 — **SKIP：未提供真 PostgreSQL / `DATABASE_DSN`**
- [ ] RPO ≤15 分钟，RTO ≤2 小时 — **SKIP：未提供 PITR、WAL、Provider 快照和现场时间戳**
- [ ] 所有关键指标和告警可触发 — **SKIP：指标名称与 PrometheusRule 已测试，未在 Prometheus/Alertmanager 现场触发**

## 已交付生产化内容

- 单一后端镜像与 Compose 数据面；
- Transfer Gateway、Worker、SFTP、Reconciler 独立 Helm Workload；
- production values、HPA、PDB、Topology Spread、restricted Pod Security；
- NetworkPolicy、Secret 引用、指标与固定告警；
- 安全、故障注入、容量与断点续传测试入口；
- `scripts/run-filesystem-load.ps1`，输出 JSON/Markdown 报告；
- 备份恢复与 Knowledge 迁移/回滚 Runbook；
- 灾备演练记录模板与明确 SKIP 判定。

## Provider 发布范围

首发原生 Provider：Local/PVC、NFS、S3 Compatible。

COS、OSS、Azure Blob：Adapter 扩展路线，不作为 v1.0 已验收原生能力。

## 解除发布阻塞

1. 在目标或等价容量环境运行 `scripts/run-filesystem-load.ps1`，归档脱敏报告并满足 20 GiB 恢复、200 并发错误率 <1%、无重复 Revision、无配额超卖、分页 P95 ≤500 ms。
2. 按灾备文档执行 PostgreSQL PITR、Provider 恢复、对账与抽样 SHA-256，实测 RPO/RTO。
3. 连续执行 Knowledge inventory、precopy、switch、rollback、switch，证明行数与 SHA-256 100% 匹配。
4. 在 Prometheus/Alertmanager 逐个触发并恢复固定告警。
5. 将上述 SKIP 更新为 PASS 后再签署 `Unified File Infrastructure v1.0`。
