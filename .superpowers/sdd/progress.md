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
