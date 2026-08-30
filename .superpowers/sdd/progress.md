# Plugin Framework SDD Progress

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
