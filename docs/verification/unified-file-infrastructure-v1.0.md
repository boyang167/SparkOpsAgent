# Unified File Infrastructure v1.0 总体验收

> 发布判定：**NOT ACCEPTED / RELEASE BLOCKED**。G1–G4 候选证据和 G5 生产化交付物已存在，但主计划固定容量、迁移和灾备硬条件缺少现场证据。本文不把 SKIP 改写为 PASS，因此当前版本不得进入正式发布流程。

## 验收范围

- G1：文件内核、权限、配额与审计；
- G2：Transfer Session、Provider、扫描与 Commit；
- G3：Storage Pool、Placement、迁移、Lifecycle、GC 与 Reconciler；
- G4：File Explorer、Knowledge、Artifact、Agent/Workflow、MCP、SFTP、Preview/Search；
- G5：镜像、Compose、Helm、HA、安全、可观测性、容量工具、恢复与发布证据。

分 Gate 证据见：

- `docs/verification/unified-file-infrastructure-g1.md`
- `docs/verification/unified-file-infrastructure-g2.md`
- `docs/verification/unified-file-infrastructure-g3.md`
- `docs/verification/unified-file-infrastructure-g4.md`
- `docs/verification/unified-file-infrastructure-g5.md`
- `docs/verification/unified-file-infrastructure-disaster-recovery.md`

## 2026-11-13 Fresh 检查

| 检查 | 结果 |
|---|---|
| `go test ./...`（backend） | PASS |
| `pnpm --filter web typecheck`（fronted） | PASS |
| `REQUIRE_INTEGRATION=1 go test -tags=integration ./tests/filesystem/... -count=1` | PASS；现场容量用例因 `FS_LOAD_RUN` 未设置而 SKIP |
| `go test ./deployments/helm -count=1` | PASS |
| `helm lint deployments/helm/sparkops` | PASS；dependency 未 vendored 警告 |
| `make test` / `make helm-lint` | SKIP：Windows 执行环境未安装 `make`；上述等价子命令已分别执行 |

## 主计划固定硬条件

- [x] 跨组织越权测试：0 泄漏（集成夹具级）
- [ ] Idempotency-Key：0 重复 Session/Revision — **SKIP：单元/夹具覆盖存在，未执行 200 并发目标环境证明**
- [ ] 配额并发：0 超卖 — **SKIP：单元/夹具覆盖存在，未归档目标环境配额指标/查询证据**
- [ ] Knowledge 迁移：100% 行数与 SHA-256 匹配 — **SKIP：未提供真 PostgreSQL `DATABASE_DSN`**
- [ ] 20 GiB 断点续传：通过 — **SKIP：未提供目标环境和 20 GiB fixture**
- [ ] 200 并发传输：通过 — **SKIP：未提供目标环境；未生成压测报告**
- [ ] RPO：≤15 分钟 — **SKIP：未执行 PostgreSQL PITR / Provider 恢复**
- [ ] RTO：≤2 小时 — **SKIP：无现场故障与恢复完成时间戳**

## 已验证的生产候选能力

- 生产 Helm 工作负载、副本、HPA、PDB、Topology Spread 与 restricted Pod Security 静态测试通过；
- NetworkPolicy、Secret 引用、Provider secret refs 与固定告警模板静态测试通过；
- 文件系统 security / consistency 集成套件通过；
- 后端全量测试、前端类型检查和 Helm lint 通过；
- Local/PVC、NFS、S3 Compatible 为首发原生 Provider；
- COS、OSS、Azure Blob 明确为 Adapter 扩展路线；
- 容量脚本、恢复 Runbook、Knowledge 迁移/回滚 Runbook 和灾备证据模板已交付。

## 未验证或延期

- 目标集群运行态 NetworkPolicy 探测、扩缩容和 Pod disruption 演练；
- 20 GiB 40% 中断后恢复、200 并发、100 TB 元数据分布分页 SLO；
- Knowledge 真库 inventory → precopy → switch → rollback → switch；
- PostgreSQL PITR、Provider 版本/副本恢复、搜索重建和抽样 SHA-256；
- Prometheus/Alertmanager 全部固定告警触发与恢复；
- G4 已记录的 SFTP 真 SSH、Export Worker 字节面、Preview Worker 等 defer。

## 发布签署条件

只有以下条件全部满足，才能将顶部状态改为 ACCEPTED：

1. 归档脱敏的 `filesystem-load-report.md/json`，满足容量、错误率、幂等、配额和分页 SLO；
2. 归档 Knowledge 真库迁移与两次 switch 的行数/SHA-256 报告；
3. 填写灾备记录的 UTC 时间戳、备份标识、校验结果并满足 RPO/RTO；
4. 完成告警触发和目标集群网络/HA 演练；
5. 关闭 G4 仍影响发布的数据面 defer；
6. 发布负责人和运维负责人共同签署。

当前结论：**Unified File Infrastructure v1.0 不可正式发布；分支仍是待合并的生产候选。**
