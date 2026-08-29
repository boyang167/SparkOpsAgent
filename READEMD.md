# Ai Plugin 
- SuperPowers
- BMAD
    npx bmad-method install


# SparkOps Agent Platform

单体仓库：后端 + 前端 + 规格文档

## 快速开始

### 1. 后端

```bash
cd backend
cp config.example.yaml config.yaml   # 编辑 database / redis
go run ./cmd/seed                     # 初始化 demo 数据
go run ./cmd/server                   # :8080
```

密码含 `@` 等特殊字符时，请用结构化 database 配置（见 `config.example.yaml`），不要用未编码的 DSN URL。

### 2. 前端

```bash
cd fronted
cp apps/web/.env.example apps/web/.env.local
pnpm dev                              # :3000
```

### 3. Docker / Helm

- Compose: `make docker-up`
- Helm: `deployments/helm/sparkops/README.md`
- 发布镜像: 打 tag `v1.0.0` 触发 GHCR 推送

## 目录

| 路径 | 说明 |
|------|------|
| [`backend/`](backend/) | Go DDD 后端 · `/api/v1` |
| [`fronted/`](fronted/) | Next.js 前端 |
| [`spec/`](spec/) | 架构与设计文档 |

## 规格文档

- [Greenfield Go DDD（后端）](spec/greenfield-go-ddd.md)
- [前端架构 + BFF](spec/frontend-architecture.md)
- [平台愿景](spec/architecture.md)

## 旧系统（只读参考）

- `A:\21-Agent\platformsuite` — 旧 Go DevOps 后端
- `A:\21-Agent\platformsuite-fronted` — 旧 dva 前端

不做数据迁移、不兼容旧 API。

## 默认 Super Admin

| 项 | 值 |
|---|---|
| 邮箱 | `superadmin@sparkops.local` |
| 密码 | `SuperAdmin1234!` |

账号不存在或登录 401 时，在 `backend` 目录执行（PowerShell 单行）：

```powershell
go run ./cmd/grant-super-admin -email superadmin@sparkops.local -password "SuperAdmin1234!" -display-name "Platform Super Admin" -register
```


Demo / 平台超管（空间 owner）
demo@sparkops.local
Demo1234!
管理员
admin@sparkops.local
Demo1234!
研发
developer@sparkops.local
Demo1234!
审核员
reviewer@sparkops.local
Demo1234!
运维
operator@sparkops.local
Demo1234!
只读
viewer@sparkops.local
Demo1234!

SparkOps = 多租户 Agent 控制面 + 可编排数据面；对外通过 REST/OpenAPI 给系统集成，通过 MCP 给 AI 客户端集成，通过 Webhook 给事件驱动集成。

https://docker.aityp.com/i/search?search=jenkins

gitlab: root asdf!123456
glpat-emaYfTem-JstBup3_U43d286MQp1OjEH.01.0w0hwoe9p

admin 
Harbor12345

test-gitlab  glpat-LOYje5HyX9RQd1Kjs-4Vx286MQp1OjMH.01.0w16qu9hd
ssh://git@192.168.5.24:2224/cicd/test-repo.git
http://192.168.5.24:8929/cicd/test-repo.git


jenkins : root 123456