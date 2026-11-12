# Unified File Infrastructure v1.0 Master Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在一个正式版本中交付 SparkOps 企业级统一文件基础设施，并通过 G1–G5 五个内部验收门控制依赖、质量和上线风险。

**Architecture:** 控制面作为现有 Go 模块化单体中的 `filesystem` BC；传输、异步处理、SFTP 和对账使用同仓独立进程。前端沿用 contracts/upstream/bff-modules/BFF/web-sdk 分层，可靠异步路径复用 PostgreSQL Outbox/Inbox 与 RabbitMQ。

**Tech Stack:** Go 1.26、Gin、GORM、PostgreSQL 15、Redis 7、RabbitMQ 3.13、Next.js 15、TypeScript、Zod、Vitest、Kubernetes/Helm、Local/PVC、NFS、S3 Compatible。

## Global Constraints

- 权威规格：`docs/superpowers/specs/2026-11-12-enterprise-unified-file-infrastructure-design.md`。
- 首发容量：单文件 ≤20 GB、总量 ≤100 TB、并发传输 ≤200。
- 可用性：99.9%，RPO ≤15 分钟，RTO ≤2 小时。
- 首发 Provider：Local/PVC、NFS、S3 Compatible。
- 业务模块只保存 File ID/Artifact ID，不保存物理路径或 object key。
- `filesystem` 是文件元数据唯一写入者；跨 BC 只使用 Port、ACL 或领域事件。
- 身份、角色、Casbin/OPA、Entitlement 与审计复用现有平台，不创建第二套 IAM。
- Redis 不是配额、授权或文件元数据事实源。
- 数据库与存储之间不做分布式事务，必须使用状态机、Outbox、幂等、对账与 GC。
- 默认跨组织拒绝；组织内 Blob 去重；Agent/Workflow 使用短期 Capability Token。
- 浏览器控制请求只走 BFF；二进制流不包装 JSON Envelope。
- 每个任务采用 TDD，任务结束必须执行指定测试并独立提交。
- 修改 `backend/` 运行时代码后，测试完成必须执行 `.\scripts\dev-restart.ps1 -Backend`。

---

## Plan Suite

| Gate | Plan | 独立交付物 |
|---|---|---|
| G1 | `2026-11-12-unified-file-infrastructure-g1-kernel.md` | Namespace/Node/Revision/Blob/Replica、Local Provider、权限、配额、审计、REST 内核 |
| G2 | `2026-11-12-unified-file-infrastructure-g2-transfer.md` | Transfer Session、分片断点续传、NFS/S3、扫描与原子 Commit、Transfer Gateway |
| G3 | `2026-11-12-unified-file-infrastructure-g3-storage-fabric.md` | Pool/Binding/Placement、迁移、Lifecycle、GC、Reconciler |
| G4 | `2026-11-12-unified-file-infrastructure-g4-integrations.md` | File Explorer、Knowledge 迁移、Artifact、Agent/Workflow、MCP、SFTP、Preview/Search |
| G5 | `2026-11-12-unified-file-infrastructure-g5-production.md` | Helm/Compose、HA、安全、性能、故障注入、恢复与发布证据 |

## Dependency Graph

```mermaid
flowchart LR
  G1[G1 Kernel] --> G2[G2 Transfer]
  G2 --> G3[G3 Storage Fabric]
  G3 --> G4[G4 Integrations]
  G4 --> G5[G5 Production]
```

任何 Gate 未通过时不得开始依赖其数据结构或公开契约的下一 Gate。可以并行的工作仅限：

- G1 后端领域模型与前端 contracts。
- G2 Provider Adapter 与 Transfer Gateway。
- G4 File Explorer 与 Knowledge 迁移工具，但二者都必须基于已冻结的 G3 API。
- G5 Helm 模板与性能工具可提前开发，但只能在 G4 完成后执行发布验收。

### Task 1: 建立执行基线

**Files:**
- Read: `docs/superpowers/specs/2026-11-12-enterprise-unified-file-infrastructure-design.md`
- Read: `docs/superpowers/plans/2026-11-12-unified-file-infrastructure-g1-kernel.md`
- Modify: none

**Interfaces:**
- Consumes: 已批准设计规格与当前仓库测试基线。
- Produces: 可复现的基线测试结果和独立工作树。

- [ ] **Step 1: 创建隔离工作树**

按 `superpowers:using-git-worktrees` 技能创建工作树，分支名使用：

```text
feat/unified-file-infrastructure
```

- [ ] **Step 2: 记录后端基线**

Run:

```powershell
cd backend
go test ./...
```

Expected: exit code 0；任何既有失败必须先记录，不能归因于新功能。

- [ ] **Step 3: 记录前端基线**

Run:

```powershell
cd fronted
pnpm typecheck
pnpm vitest run
```

Expected: exit code 0。

- [ ] **Step 4: 验证迁移基线**

Run:

```powershell
cd backend
go test ./migrations/... ./internal/bootstrap/...
```

Expected: exit code 0，当前最高 SQL migration 为 `000076`。

### Task 2: 执行 G1–G5

**Files:**
- Read: 本计划套件中的五份 Gate 计划
- Modify: 各 Gate 计划列出的文件

**Interfaces:**
- Consumes: Task 1 的绿色基线。
- Produces: 五个 Gate 的提交序列与验收证据。

- [ ] **Step 1: 执行 G1**

完成 `2026-11-12-unified-file-infrastructure-g1-kernel.md` 的全部复选项。

Expected: Namespace、文件元数据、Local Provider、权限、配额和审计 API 可独立使用。

- [ ] **Step 2: 执行 G2**

完成 `2026-11-12-unified-file-infrastructure-g2-transfer.md` 的全部复选项。

Expected: 20 GB 协议上限、分片恢复、NFS/S3、扫描与 Commit Protocol 可验证。

- [ ] **Step 3: 执行 G3**

完成 `2026-11-12-unified-file-infrastructure-g3-storage-fabric.md` 的全部复选项。

Expected: Pool、Placement、迁移、Lifecycle 和 GC 在故障后可收敛。

- [ ] **Step 4: 执行 G4**

完成 `2026-11-12-unified-file-infrastructure-g4-integrations.md` 的全部复选项。

Expected: File Explorer、Knowledge、Agent、Workflow、MCP 与 SFTP 使用统一 File ID。

- [ ] **Step 5: 执行 G5**

完成 `2026-11-12-unified-file-infrastructure-g5-production.md` 的全部复选项。

Expected: HA、安全、容量、恢复和迁移回滚证据满足规格。

### Task 3: v1.0 总体验收

**Files:**
- Create: `docs/verification/unified-file-infrastructure-v1.0.md`
- Modify: `docs/SparkOps技术架构说明.md`
- Modify: `docs/SparkOps部署与运维说明.md`

**Interfaces:**
- Consumes: G1–G5 的测试、压测、恢复与迁移证据。
- Produces: 可签署的发布验收记录。

- [ ] **Step 1: 运行仓库总检查**

Run:

```powershell
make test
make helm-lint
```

Expected: 全部通过。

- [ ] **Step 2: 验证设计硬条件**

在验收文档中逐项记录以下固定结果格式：

```markdown
- [x] 跨组织越权测试：0 泄漏
- [x] Idempotency-Key：0 重复 Session/Revision
- [x] 配额并发：0 超卖
- [x] Knowledge 迁移：100% 行数与 SHA-256 匹配
- [x] 20 GB 断点续传：通过
- [x] 200 并发传输：通过
- [x] RPO：≤15 分钟
- [x] RTO：≤2 小时
```

- [ ] **Step 3: 更新架构与运维文档**

新增已实现组件、端口、配置、迁移顺序、回滚步骤和告警，不把设计目标写成已上线事实，除非验收证据已存在。

- [ ] **Step 4: 提交总体验收**

```powershell
git add docs/verification/unified-file-infrastructure-v1.0.md docs/SparkOps技术架构说明.md docs/SparkOps部署与运维说明.md
git commit -m "docs(filesystem): record v1.0 production acceptance"
```

## Release Rule

G1–G4 只产生内部候选构建。只有 G5 和 Task 3 全部通过后，才允许：

```text
Unified File Infrastructure v1.0
```

进入正式发布流程。
