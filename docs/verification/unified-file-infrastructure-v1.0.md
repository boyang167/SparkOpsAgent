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

## 2026-11-14 数据面补缺复跑（UTC+8）

| 项 | 结果 |
|---|---|
| Transfer Gateway `RecordUploaded` → `TransferUploaded` 事件 | **PASS**（单元 + gateway 测试；`go test ./internal/filesystemgateway/...`） |
| 内部 Control API（grant / uploaded） | **PASS**（`RegisterInternalGatewayRoutes` + module 测试） |
| 工作区默认 LOCAL Provider（`default-local`） | **PASS**（`EnsureDefaultLocalProvider` + CreateTransfer 默认解析） |
| `GET /filesystem-audits` 后端路由 | **PASS**（`ListFilesystemAudits` + query 过滤 `filesystem.`） |
| file-worker 订阅 verify/commit | **PASS**（`cmd/file-worker` 启动注册 AMQP 订阅） |
| LOCAL 数据面端到端 | **PASS**（浏览器创建 Transfer Session → Gateway PUT → AMQP verify/commit → Session `COMPLETED` → 文件详情/版本可见；131072 B fixture） |
| Gateway 浏览器边界 | **PASS**（请求体流式交给 Provider；CORS 仅允许配置来源；通知失败释放 nonce，可用同 capability 幂等重试） |
| SFTP `HTTPTransferControl` → Begin/Copy/RecordUploaded | **PASS**（控制面 HTTP + 共享卷落盘；需 `SPARKOPS_SFTP_DEV_*` 做真 SSH 抽查） |
| Export 字节 Worker | **SKIP**（Import 仍为元数据节点，无跨组织字节复制 Worker） |
| Knowledge / 20GiB / 200 并发 / RPO/RTO | **SKIP**（无归档现场报告） |

Compose：`filesystem.transfer_gateway.control_base_url=http://api:8080`，网关依赖 `api` healthy。

## 2026-11-14 Task 8 统一工作台自动验收（UTC+8）

| 项 | 结果 |
|---|---|
| Filesystem 前端矩阵 | **PASS** — fresh 结果为 19 files / 139 tests，exit 0 |
| Monorepo 前端类型检查 | **PASS** — 6 packages，exit 0 |
| 后端全量回归 | **PASS** — 清除 shell 中残留 `DATABASE_DSN` / `DATABASE_URL` 后执行 `go test ./... -count=1`，exit 0 |
| 响应式与无障碍自动契约 | **PASS** — tab/tree/list/grid、键盘移动替代、Drawer Esc、progressbar、窄屏布局、横向滚动；tabs/选择 label/关键按钮为 44px 触控目标，checkbox 保持 16px；focus-visible 使用高对比 `--primary` 的 2px outline + 2px offset并自动验证 token 对比度 ≥4.5 |
| 主动作与 landmark 契约 | **PASS（组件/源码契约）** — 编辑/Modal 打开时背景主动作降为 secondary，当前提交保持唯一 primary；文件浏览不再嵌套 `main`，保留 tab `aria-labelledby` |
| 浏览器手工矩阵 | **PASS（LOCAL 环境）** — 管理员四标签、新建目录、懒加载树、键盘移动、列表/网格保留选择、窄屏目录 Drawer、Esc 关闭、存储治理、审计、传输取消、4 文件 Gateway PUT（实测最大并发 3）、完成态文件详情与版本均通过 |
| MinIO / S3 Compatible | **SKIP** — 本轮未配置真实 MinIO endpoint；不以 LOCAL Provider 结果替代 |
| 实时容量曲线 | **不在范围** — Task 8 未实现容量曲线；不作为发布能力声明 |

上述 PASS 仅证明本轮自动执行范围，不改变 20 GiB、200 并发、Knowledge 真库迁移、RPO/RTO 与其他现场验收项的 SKIP 状态；发布判定仍为 **NOT ACCEPTED / RELEASE BLOCKED**。

## 2026-11-13 Fresh 检查（G5 Task 7 复跑 22:10 UTC+8）

| 检查 | 结果 |
|---|---|
| `go test ./...`（backend） | PASS，约 189s |
| `pnpm --filter web typecheck`（fronted） | PASS |
| `pnpm vitest run`（filesystem 4 files / 8 tests） | PASS |
| `REQUIRE_INTEGRATION=1 go test -tags=integration ./tests/filesystem/... -count=1` | PASS；`TwentyGiB` / `ConcurrentTransfer` / `MetadataCapacity` **SKIP** |
| `go test ./deployments/helm -count=1` | PASS |
| `helm lint deployments/helm/sparkops` | PASS；dependency 未 vendored 警告 |
| `mingw32-make test` / `mingw32-make helm-lint` | **PASS**；本机无 `make.exe`，使用 Make 兼容实现 |

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

当前结论：**Unified File Infrastructure v1.0 已合入 `master`，但在容量、迁移与灾备硬条件完成前仍不可正式发布。**
