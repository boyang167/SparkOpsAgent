# SparkOps 技术架构说明

## 总体架构

SparkOps 使用 Go 模块化单体作为控制面，Next.js Web/BFF 作为浏览器入口，PostgreSQL 保存业务事实，Redis 用于非事实型缓存/协调，RabbitMQ 承载可靠异步消息。统一文件基础设施是控制面中的 `filesystem` bounded context；文件传输、异步处理、SFTP 和对账以同仓独立进程运行。

## 统一文件基础设施

### 数据与边界

- `filesystem` 是 Namespace、Node、Revision、Blob、Replica、Transfer Session、Quota、Storage Pool/Binding 和 Lifecycle Policy 的唯一元数据写入者。
- 其他业务模块只保存 File ID 或 Artifact ID，不保存物理路径和 object key。
- PostgreSQL 与 Provider 之间不使用分布式事务；状态机、事务 Outbox/Inbox、幂等键、Reconciler 和延迟 GC 共同处理失败恢复。
- 默认拒绝跨组织访问；Workspace 授权、资源 Grant 和短期 Capability Token 共同约束 API、Agent、Workflow、MCP 与传输入口。

### 运行组件

| 组件 | 职责 | 默认端口 |
|---|---|---|
| API | 文件元数据、授权、传输控制面和存储管理 | 8080 |
| Web / BFF | 浏览器控制请求入口；二进制流不包装 JSON | 3000 |
| File Transfer Gateway | Local/PVC、NFS 等二进制流入口 | 8082 |
| File Worker | Verify、恶意文件扫描、Commit、Preview/异步任务 | 无入站端口 |
| SFTP Gateway | 虚拟根与 landing zone 接入 | 2022 |
| File Reconciler | DB 租约单活的副本对账、迁移与 GC 收敛 | 无入站端口 |

同一后端镜像包含上述进程和 Knowledge 迁移命令。生产 Helm 将 API、Gateway、Worker、SFTP 和 Reconciler 拆为独立 Workload，以独立副本、资源和扩缩容策略运行。

### Provider

首发原生 Provider 为：

- Local/PVC：本地或 Kubernetes 持久卷；
- NFS：共享 POSIX 存储；
- S3 Compatible：兼容 S3 API 的对象存储。

腾讯 COS、阿里云 OSS、Azure Blob 作为 Adapter 扩展路线，不属于 v1.0 已验收原生 Provider。

### 高可用与安全

- 生产 values 为 API/Web/Transfer Gateway/Worker/SFTP/Reconciler 配置至少 2 副本；Gateway 和 Worker 配置 HPA。
- PDB、Topology Spread、non-root、只读根文件系统、RuntimeDefault seccomp 和 drop ALL capabilities 由 Helm 模板固定。
- NetworkPolicy 将数据面 Pod 入站限制到 API/Web→Gateway 与 SFTP 端口，出站限制到 DNS、PostgreSQL、Redis、RabbitMQ、对象存储和 Scanner 端口。
- Provider credential 仅通过 `providerSecretRefs` 引用现有 Secret；Capability secret 和 SFTP host key 也来自 Secret。

## 一致性与可观测性

Transfer Session 在上传完成后仍需 Verify、Scanner 和原子 Commit；只有 `COMPLETED` 才发布 Revision。配额预留、Revision 与 Outbox 在数据库事务中提交。Reconciler 负责 Blob/Replica、迁移和 GC 的最终收敛。

Prometheus 指标覆盖传输、字节、失败率、状态耗时、Scanner、队列延迟、Provider、配额、副本不一致和 GC。固定告警和排障顺序见 `SparkOps部署与运维说明.md`。

## 验收边界

代码、Chart 和测试通过不等同于生产发布证据。20 GiB 断点续传、200 并发、100 TB 元数据分布、Knowledge 真库迁移/回滚及 RPO/RTO 必须在目标或等价容量环境实测；缺少环境时在验证文档中标记 SKIP。

截至 2026-11-13，生产候选架构与测试脚手架已完成，但上述现场硬条件仍为
SKIP，v1.0 尚未获得正式发布签署。权威状态见
`docs/verification/unified-file-infrastructure-v1.0.md`。
