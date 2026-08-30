# Plugin Framework SDD Progress

# Kubernetes Plugin SDD Progress

Baseline: `50174f42de8eed62a5e7c7f84a20fef00f3cc043`; `pnpm --filter @sparkops/web typecheck` PASS. `go test ./... -count=1` has one authorized pre-existing failure: `TestDeploymentPipelineCIFailureHasZeroApply` (`delivery.deployment_runs` missing in SQLite); Kubernetes/plugin-related packages PASS.
Plan: `docs/superpowers/plans/2026-08-30-kubernetes-plugin.md`
Execution: current `master` working tree by explicit user choice; task-scoped commits authorized by selecting Subagent-Driven execution. Existing WIP must never be staged.
Task 1–2: complete (commits 50174f4..250eda7, review approved; Minor: report contains duplicate/conflicting RED records).
Task 3: complete (commits 250eda7..b7b372e, review approved; Minor: no explicit two-/three-node parent cycle regression test, implementation rejects them).
Task 4: complete (commits b7b372e..d272160, review approved after explicit `read_only` metadata and `keyMatch2` single-segment fixes).
Task 5: complete (commits d272160..95bccbc, review approved after rejecting multiple manifests; includes pre-existing aligned 50 MiB single-file WIP).
Task 6: complete (commits 95bccbc..1e89452, review approved).
Task 7: complete (commits 1e89452..4ef35df, review approved after seed receipt/router/manifest-single-source and bundled descriptor fixes; Minor: seed version/display name remain hardcoded).
Task 8: complete (commits 4ef35df..1fa1d09, review approved after workspace membership, URL userinfo, transaction locking, and report cleanup fixes).
Task 9: complete (commits 1fa1d09..b078033, review approved after discovery/scope/list semantics, pagination, path, and header-impersonation fixes).
Task 10: complete (commits b078033..8e336d6, review approved; Minor: some client failure branches lack explicit regression tests).
Task 11: complete (commits 8e336d6..d51b59f, review approved; Minors: recursive delete scope and 465-line handler).
Task 12: complete (commits d51b59f..dd0691a, review approved after operation-log SSE omission, strict Bearer, bounded UTF-8-safe log chunks, and boundary tests; includes aligned pre-existing multipart omission WIP).
Task 13: complete (commits dd0691a..5bdeb15, review approved after strict recording cap, workspace-scoped finalize, and manifest fix).
Task 14: complete (commits 5bdeb15..0b6681f, review approved after switching CloudTTY to CloudShell CRD with bounded cleanup and namespace-correct command/kubeconfig).
Task 15: complete (commits 0b6681f..916dfbd, review approved after Velaux streaming/path/cap, Chat SSE/audit, and plugin request-log privacy fixes; Minors: trusted Arthas tar full validation, cleanup batch bound, output quota).
Task 16: complete (commits 916dfbd..2ae74f1, review approved after envelope, server pagination, SSE, YAML, and Velaux sandbox fixes; cross-task gap: generic BFF catch-all assigned to Task 17).
Task 17: complete (commits 2ae74f1..37f1554, review approved after recursive path validation, HTML sandbox, and safe redirect rewrite; closes Task 16 BFF gap).
Task 18: complete (commits 37f1554..802878a, review approved after BFF ZIP coverage and restoring/verifying the unpadded Arthas tar).
Whole-branch review: complete at `98acd44` (Critical/Important fixed in `bd8444e` and `219c67e`; final verdict Ready to merge: Yes; whitespace cleanup `98acd44`).
Final verification: `go test ./... -count=1` PASS; `pnpm -r typecheck` PASS; Kubernetes/BFF/SDK Vitest 70/70 PASS; `git diff --check 50174f42..HEAD` PASS; IDE lints clean.

Baseline: `go test ./...` PASS; `pnpm --filter web typecheck` PASS.
Plan: `docs/superpowers/plans/2026-08-30-plugin-framework.md`
Execution: current `master` working tree by explicit user choice; task-scoped commits authorized.
Task 1: complete (commits 39cef7d..e1ddd73, review clean; Minor: empty install_test.go and test-only reset API exposure).
Task 2: complete (commits e1ddd73..f264646, review clean; Minor: task report contains unrelated legacy content after the current report).
Task 3: complete (commits f264646..004814b, review clean after strict API method fix).
Task 4: complete (commits 004814b..80c8fd4, review clean; Minor: timestamp defaults differ from SQL and FK existence check may duplicate a migration-created FK).
Task 5: complete (commits 80c8fd4..d0583e7, review clean after duplicate-entry and AST fixes; Minor: no decompressed-size limit).
Task 6: complete (commits d0583e7..6b3fb2c, review clean after Git worktree/branch/path/URL safety fixes; Minor: command runner inherits process environment).
Task 7: complete (commits 6b3fb2c..5376610, review iterated through ZIP limits, package digest, OPEN PR recovery, remote-branch reuse, and digest CAS).
Task 8: complete (commits 5376610..6a2aefb, review clean after plugin path matching was moved before generic entitlement skips).
Task 9: complete (commits 6a2aefb..fa39755, review clean; Minor: no browser E2E; /platform/plugins already covered by existing /platform/ prefix).
Task 10: complete (commits fa39755..3af8d5c, review approved; Minor: tests don't lock full plugin.go text; YAML display_name unquoted; dual {{id}}/{{.ID}} placeholders).
Whole-branch review: Important gaps fixed in d52cb41 (plugin JWT, sidebar parent menu, /admin/plugins nav, example deployed seed, frontend path + BC import checks, PR template). Remaining Minors: git env inheritance; AutoMigrate timestamps vs SQL; no browser E2E; scaffold test lock; YAML quoting; dual placeholders. Uninstall of non-deployed leftovers not addressed.

# Project Deploy Pipeline SDD Progress

Baseline: `6f439fb4eca37148d35faf15ffed36fac435733b`
Plan: `docs/superpowers/plans/2026-08-29-project-deploy-pipeline.md`
Execution: current `master` working tree by explicit user choice; no commits without separate authorization.

Task 1: complete (uncommitted working tree, review clean; Minor: terminal immutability test covers `succeeded` explicitly, while implementation covers all terminal states).
Task 2: complete (uncommitted working tree, review clean).
Task 3: complete (uncommitted working tree, review clean; Minor: AMQP consumer channel natural-end cleanup and lock-held broker I/O remain for final review).
Task 4: complete (uncommitted working tree, review clean).
Task 5: complete (uncommitted working tree, review clean; Minor: client-go fake cannot assert SSA FieldManager/Force although production sets both).
Task 6: complete (uncommitted working tree, review clean).
Task 7: complete (uncommitted working tree, review clean; cross-task gap: deployment stage SSE hub is not yet fed by orchestration events, assigned to Task 8).
Task 8: complete (uncommitted working tree, review clean; real PostgreSQL and external Jenkins/Harbor/Kubernetes/AMQP E2E not available locally).

# PlatformSuite Migration — SDD Progress

## Phase A–G — 完成 ✅

功能迁移、生产就绪（AutoMigrate、/ready、Webhook、CronJob、GitLab、Docker Compose）均已落地。

# PlatformSuite Migration — SDD Progress

## Phase H — K8s / CI ✅

Helm Chart、CI 四 Job、GHCR Release（tag `v*` 触发）

## Phase I — 本地联调 / 配置 ✅

| 项 | 状态 |
|----|------|
| 结构化 database 配置（密码含 `@` 自动 URL 编码） | ✅ |
| `DATABASE_HOST/PASSWORD/...` 环境变量 | ✅ |
| Makefile 常用命令 | ✅ |
| GHCR release workflow | ✅ `.github/workflows/release.yaml` |
| Helm 可选 Bitnami 依赖（PG/Redis/RabbitMQ） | ✅ Chart.yaml |


| 项 | 状态 |
|----|------|
| Helm Chart（API + Web + Ingress + HPA + Secrets） | ✅ `deployments/helm/sparkops/` |
| values-production.yaml 生产覆盖 | ✅ |
| CI：backend + frontend + helm lint + docker build | ✅ `.github/workflows/ci.yaml` |
| Docker Compose UPSTREAM_URL 修正 | ✅ |

## 部署方式

### 1. Docker Compose（开发/单机）

```bash
cd backend/deployments/docker
cp .env.example .env
docker compose up --build -d
curl http://localhost:8080/ready
```

### 2. Helm（K8s 生产）

```bash
helm lint deployments/helm/sparkops
helm upgrade --install sparkops deployments/helm/sparkops \
  -f deployments/helm/sparkops/values-production.yaml \
  --set secrets.jwtSecret=$JWT_SECRET \
  --set secrets.databaseDsn=$DATABASE_DSN
```

详见 `deployments/helm/sparkops/README.md`。

## 验证

```bash
cd backend && go test ./...
cd fronted && pnpm --filter web typecheck
helm lint deployments/helm/sparkops
```

## 已知限制

- OAuth/LDAP/WeChat — WONTFIX v1
- Istio VirtualService — 未实现
- PG/Redis/RabbitMQ 需外部或自行部署（Helm 不含数据层）
- CronJob 为应用内分钟级调度
