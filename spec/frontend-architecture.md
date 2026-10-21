# SparkOps 前端整体架构设计（含 BFF）

> 位置：`A:\21-Agent\SparkOpsAgent\fronted\`  
> 后端 API：`A:\21-Agent\SparkOpsAgent\backend\` · `/api/v1/*`  
> 原则：**页面只调 BFF，BFF 只调 Go Upstream，禁止页面直连 Go API。**

---

## 一、为什么需要 BFF

后端 DDD 重构后，API 是 **BC 粒度、资源导向** 的：

```
/api/v2/projects/{id}
/api/v2/builds?projectId=
/api/v2/runtime/pods?...
/api/v2/governance/permissions?...
```

但前端页面是 **屏幕/用例导向** 的，一次渲染往往需要多个 BC 的数据：

| 页面 | 需要聚合的后端 BC |
|------|-------------------|
| 项目详情 | application + delivery + runtime + governance |
| Dashboard | organization + application + delivery + notification |
| Agent Studio | agent + knowledge + governance + application |
| Pod 详情 | runtime + application（所属项目） |

若没有 BFF，会出现：

- 页面发 5~8 个请求，瀑布加载、竞态、loading 态难统一
- 后端 DTO 直接暴露给 UI，字段改名导致全页面改动
- 绞杀期 v1/v2 API 混存，页面要感知版本差异
- Token、Workspace Header 在每个 fetch 重复拼装
- SSE/WebSocket 跨域、鉴权头传递复杂

**BFF 的职责**：为前端提供「刚好够用的屏幕 API」，内部编排多个 Upstream BC 调用。

---

## 二、三层架构总览

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Layer 1 · Presentation（展示层）                                        │
│  app/(platform)/*  page.tsx · components · hooks                        │
│  只消费 BFF 契约（@sparkops/web-sdk）                                    │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │  /api/bff/v1/*  或  Server 直调 bff-modules
┌───────────────────────────────▼─────────────────────────────────────────┐
│  Layer 2 · BFF（前端专属后端）                                           │
│  Route Handlers · 用例编排 · DTO→ViewModel · 缓存 · 流式代理              │
│  packages/bff-modules/{bc}/                                               │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │  @sparkops/upstream-*（服务端专用）
┌───────────────────────────────▼─────────────────────────────────────────┐
│  Layer 3 · Upstream（Go DDD 后端）                                       │
│  /api/v2/{bc}/*  +  legacy 代理（绞杀期）                                │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.1 各层边界

| 层 | 允许 | 禁止 |
|----|------|------|
| Presentation | 调 BFF、本地 UI 状态 | 直接 import upstream-client；手写 Go URL |
| BFF | 编排 upstream、聚合、裁剪字段、绞杀路由 | 操作数据库；写业务规则（归属 Go domain） |
| Upstream Client | HTTP 调 Go、带 Header、重试 | 被 Client Component 直接引用 |

---

## 三、Monorepo 目录结构（完整版）

```
SparkOpsAgent/
├── backend/                          # Go API（greenfield-go-ddd.md）
├── fronted/
│   ├── apps/
│   │   └── web/                              # Next.js 15 主应用
│   │       ├── app/
│   │       │   ├── (auth)/                   # 展示层路由
│   │       │   ├── (platform)/
│   │       │   ├── (studio)/
│   │       │   └── api/
│   │       │       └── bff/                  # ★ BFF HTTP 入口（薄）
│   │       │           └── v1/
│   │       │               ├── auth/[...path]/route.ts
│   │       │               ├── projects/[...path]/route.ts
│   │       │               ├── runtime/[...path]/route.ts
│   │       │               ├── agents/[...path]/route.ts
│   │       │               └── stream/
│   │       │                   ├── chat/route.ts      # SSE 代理
│   │       │                   └── terminal/route.ts  # WebSocket 升级
│   │       │
│   │       ├── src/
│   │       │   ├── modules/                  # ★ 展示层（按 BC，仅 UI）
│   │       │   │   ├── application/
│   │       │   │   │   ├── components/
│   │       │   │   │   ├── hooks/          # useProjectList → 调 web-sdk
│   │       │   │   │   └── pages/          # 页面级组合（可选）
│   │       │   │   ├── runtime/
│   │       │   │   ├── agent/
│   │       │   │   └── ...
│   │       │   │
│   │       │   ├── server/                   # ★ RSC 专用：直调 BFF 用例（无 HTTP 环）
│   │       │   │   ├── loaders/              # getProjectDetailPageData()
│   │       │   │   └── actions/              # Server Actions（表单提交）
│   │       │   │
│   │       │   └── shared/
│   │       │       ├── providers/            # QueryClient、antd、Tenant
│   │       │       └── middleware.ts         # 浏览器鉴权跳转
│   │       │
│   │       ├── next.config.ts
│   │       └── package.json
│   │
│   ├── packages/
│   │   ├── contracts/                        # ★ BFF 对外契约（前端消费的 API 形状）
│   │   │   ├── src/
│   │   │   │   ├── application/              # zod schema + TS 类型
│   │   │   │   ├── runtime/
│   │   │   │   ├── common/                   # PageResult、ApiError、TenantContext
│   │   │   │   └── index.ts
│   │   │   └── package.json
│   │   │
│   │   ├── web-sdk/                          # ★ 展示层 SDK（调 /api/bff/v1）
│   │   │   ├── src/
│   │   │   │   ├── client.ts                 # fetch 封装、错误处理
│   │   │   │   ├── application/
│   │   │   │   └── runtime/
│   │   │   └── package.json
│   │   │
│   │   ├── bff-core/                         # ★ BFF 基础设施
│   │   │   ├── src/
│   │   │   │   ├── handler/                  # createBffHandler、middleware 链
│   │   │   │   ├── context/                  # BffContext（user、workspace、traceId）
│   │   │   │   ├── response/                 # 统一 { code, data, message, trace_id }
│   │   │   │   ├── errors/                   # BffError、UpstreamError 映射
│   │   │   │   └── stream/                   # SSE/WS 代理工具
│   │   │   └── package.json
│   │   │
│   │   ├── bff-modules/                      # ★ BFF 业务用例（核心）
│   │   │   ├── src/
│   │   │   │   ├── identity/
│   │   │   │   │   ├── login.ts
│   │   │   │   │   └── getSession.ts
│   │   │   │   ├── application/
│   │   │   │   │   ├── getProjectListPage.ts
│   │   │   │   │   ├── getProjectDetailPage.ts   # 聚合 project+builds+pods
│   │   │   │   │   └── createProjectWizard.ts
│   │   │   │   ├── delivery/
│   │   │   │   ├── runtime/
│   │   │   │   ├── agent/
│   │   │   │   ├── workflow/
│   │   │   │   └── dashboard/
│   │   │   │       └── getWorkplaceDashboard.ts  # 跨 BC 并行聚合
│   │   │   └── package.json
│   │   │
│   │   ├── upstream/                         # ★ Go 后端客户端（仅服务端）
│   │   │   ├── core/                          # 基类：auth header、tenant、retry
│   │   │   ├── identity/                     # OpenAPI 生成 + 手工扩展
│   │   │   ├── application/
│   │   │   ├── delivery/
│   │   │   ├── runtime/
│   │   │   ├── governance/
│   │   │   └── legacy/                       # 绞杀期：旧 API 适配器
│   │   │
│   │   ├── ui/                               # 共享 UI 组件
│   │   ├── tenant-context/                   # Workspace Provider（浏览器端）
│   │   └── config-eslint/
│   │
│   ├── pnpm-workspace.yaml
│   └── turbo.json
│
└── spec/
    ├── refactor.md
    └── frontend-architecture.md              # 本文档
```

---

## 四、包依赖关系（强制）

```
apps/web
  → web-sdk, contracts, ui, tenant-context
  → bff-modules（仅 server/ 与 app/api/bff/ 可 import）
  → upstream（禁止：仅 bff-modules 可 import）

bff-modules
  → bff-core, contracts, upstream

web-sdk
  → contracts

upstream/*
  → bff-core（context、errors）

ui, tenant-context
  → contracts（可选）
```

**CI 校验**（`dependency-cruiser` 或 ESLint `no-restricted-imports`）：

- Client Component 文件禁止 import `upstream/*`、`bff-modules/*`
- `upstream/*` 禁止被 `web-sdk` 引用

---

## 五、BFF 用例设计（与后端 BC 的映射）

BFF 模块名与后端 BC **同名但职责不同**：

| 后端 BC（Upstream） | BFF Module（屏幕用例） | 示例 BFF 端点 |
|---------------------|------------------------|---------------|
| identity | identity | `POST /api/bff/v1/auth/login` |
| organization | organization | `GET /api/bff/v1/workspace/current` |
| application | application | `GET /api/bff/v1/projects` |
| application + delivery + runtime | application | `GET /api/bff/v1/projects/:id/overview` |
| delivery | delivery | `GET /api/bff/v1/delivery/builds/:id/logs` |
| runtime | runtime | `GET /api/bff/v1/runtime/pods/:id/detail` |
| agent + knowledge | agent | `GET /api/bff/v1/agents/:id/studio` |
| 跨 BC | dashboard | `GET /api/bff/v1/dashboard/workplace` |

### 5.1 用例函数命名规范

```
get{Screen}{Purpose}     # 读：getProjectDetailPage
execute{Action}          # 写：executePublishProject
stream{Resource}         # 流：streamBuildLogs
```

每个用例一个文件，签名统一：

```typescript
// packages/bff-modules/src/application/getProjectDetailPage.ts
import type { BffContext } from '@sparkops/bff-core';
import type { ProjectDetailPageVM } from '@sparkops/contracts/application';

export async function getProjectDetailPage(
  ctx: BffContext,
  input: { projectId: string }
): Promise<ProjectDetailPageVM> {
  const [project, builds, pods, permissions] = await Promise.all([
    ctx.upstream.application.getProject(input.projectId),
    ctx.upstream.delivery.listRecentBuilds(input.projectId),
    ctx.upstream.runtime.listPodsByProject(input.projectId),
    ctx.upstream.governance.getProjectPermissions(input.projectId),
  ]);

  return mapToProjectDetailPageVM({ project, builds, pods, permissions });
}
```

---

## 六、双通道调用 BFF（RSC + Client）

### 6.1 Server Component / RSC — 直调 bff-modules（推荐）

无额外 HTTP  hop，性能更好：

```typescript
// apps/web/src/server/loaders/project.ts
import { getProjectDetailPage } from '@sparkops/bff-modules/application';
import { createBffContext } from '@sparkops/bff-core';

export async function loadProjectDetailPage(projectId: string) {
  const ctx = await createBffContext(); // 从 cookie 解析 user/workspace
  return getProjectDetailPage(ctx, { projectId });
}

// apps/web/app/(platform)/projects/[id]/page.tsx
import { loadProjectDetailPage } from '@/server/loaders/project';

export default async function Page({ params }: { params: { id: string } }) {
  const data = await loadProjectDetailPage(params.id);
  return <ProjectDetailPage data={data} />;
}
```

### 6.2 Client Component — 走 HTTP `/api/bff/v1`

适合交互、轮询、mutation：

```typescript
// packages/web-sdk/src/application/projects.ts
import type { ProjectDetailPageVM } from '@sparkops/contracts/application';
import { bffFetch } from '../client';

export function getProjectDetailPage(id: string) {
  return bffFetch<ProjectDetailPageVM>(`/projects/${id}/overview`);
}

// apps/web/src/modules/application/hooks/useProjectDetail.ts
'use client';
import { useQuery } from '@tanstack/react-query';
import { getProjectDetailPage } from '@sparkops/web-sdk/application';

export function useProjectDetail(id: string) {
  return useQuery({
    queryKey: ['project', 'detail', id],
    queryFn: () => getProjectDetailPage(id),
  });
}
```

### 6.3 Route Handler — 薄入口，不含业务逻辑

```typescript
// apps/web/app/api/bff/v1/projects/[id]/overview/route.ts
import { createBffHandler } from '@sparkops/bff-core';
import { getProjectDetailPage } from '@sparkops/bff-modules/application';

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const data = await getProjectDetailPage(ctx, { projectId: params.id });
  return data;
});
```

**规则**：Route Handler 不超过 10 行，业务永远在 `bff-modules`。

---

## 七、契约分层（contracts 包）

两套 API 形状，不可混用：

| 契约 | 包 | 消费者 | 来源 |
|------|-----|--------|------|
| Upstream DTO | `upstream/*/types` | bff-modules | Go OpenAPI 生成 |
| ViewModel / Command | `@sparkops/contracts` | web-sdk、pages、hooks | 手写 zod + 导出 TS 类型 |

```typescript
// packages/contracts/src/application/project.ts
import { z } from 'zod';

export const ProjectDetailPageVMSchema = z.object({
  id: z.string(),
  name: z.string(),
  status: z.enum(['draft', 'running', 'failed']),
  latestBuild: z.object({ id: z.string(), status: z.string() }).nullable(),
  podSummary: z.object({ total: z.number(), running: z.number() }),
  permissions: z.object({ canPublish: z.boolean(), canDelete: z.boolean() }),
});

export type ProjectDetailPageVM = z.infer<typeof ProjectDetailPageVMSchema>;
```

后端 DTO 变更时，只需改 `bff-modules` 里的 mapper，页面不受影响。

---

## 八、BFF 中间件链

```
Request
  → traceId()              # 生成/透传 X-Trace-Id
  → authenticate()         # 解析 httpOnly cookie / session
  → resolveTenant()        # OrgID + WorkspaceID → BffContext
  → authorize()            # 可选：BFF 级粗粒度权限（细粒度仍靠 Go Casbin）
  → handler(useCase)
  → envelope()             # 统一响应包装
  → errorMapper()          # Upstream 4xx/5xx → 前端友好错误
```

```typescript
// packages/bff-core/src/context/index.ts
export interface BffContext {
  traceId: string;
  user: { id: string; email: string };
  tenant: { orgId: string; workspaceId: string };
  upstream: UpstreamClients;  // 预注入 tenant header 的客户端集合
}
```

---

## 九、绞杀期：BFF 屏蔽后端版本差异

```
页面 / web-sdk
      ↓ 始终调 /api/bff/v1/projects/:id/overview
bff-modules/getProjectDetailPage
      ↓ 内部决策
      ├── upstream/application (v2)  ← 已迁移
      └── upstream/legacy/projects   ← 未迁移字段 fallback
```

**页面永远不知道 v1/v2/legacy 的存在**，迁移后端 BC 时只改 bff-modules 和 upstream adapter。

---

## 十、流式场景（Chat / 构建日志 / 终端）

| 场景 | BFF 实现 | 路径 |
|------|----------|------|
| Agent Chat SSE | Route Handler 代理 Go SSE，追加鉴权头 | `/api/bff/v1/stream/chat` |
| 构建日志 | 同上 | `/api/bff/v1/stream/builds/:id/logs` |
| Pod 终端 | WebSocket 升级代理 | `/api/bff/v1/stream/terminal` |

```typescript
// apps/web/app/api/bff/v1/stream/chat/route.ts
export async function GET(req: Request) {
  const ctx = await createBffContextFromRequest(req);
  return proxySSE(ctx, `${UPSTREAM_URL}/api/v2/agents/chat`, {
    headers: upstreamHeaders(ctx),
  });
}
```

Client Component 只连同源 BFF，避免 CORS 与 Token 暴露。

---

## 十一、展示层目录（modules）规范

```
src/modules/{bc}/
├── components/           # 纯 UI，接收 ViewModel props
├── hooks/                # Client 数据 hook（调 web-sdk）
├── schemas/              # 表单 zod（可与 contracts 的 Command 复用）
└── index.ts              # 对外导出

apps/web/app/(platform)/projects/[id]/page.tsx   # 薄：组装 loader + 组件
```

**页面文件只做三件事**：取数据（loader 或 hook）→ 传 props → 渲染 layout。

---

## 十二、与后端 DDD 模块的对照

```
Go internal/modules/application/     ↔  upstream/application + bff-modules/application + modules/application(UI)
Go internal/modules/runtime/         ↔  upstream/runtime       + bff-modules/runtime       + modules/runtime(UI)
```

| 层级 | 对齐方式 |
|------|----------|
| upstream/* | 1:1 映射 Go BC，OpenAPI 生成 |
| bff-modules/* | 1:1 同名，但函数是「屏幕用例」非 CRUD |
| modules/* (UI) | 1:1 同名，只含组件与 hooks |

---

## 十三、请求链路示例

### 项目详情页（RSC 首屏）

```
1. 浏览器 GET /projects/proj-001
2. Next.js RSC 执行 page.tsx
3. loadProjectDetailPage('proj-001')
4. bff-modules/getProjectDetailPage(ctx, ...)
5. Promise.all([
     upstream.application.getProject,
     upstream.delivery.listRecentBuilds,
     upstream.runtime.listPods,
     upstream.governance.getPermissions,
   ])
6. Go /api/v2/* × 4（并行）
7. mapper → ProjectDetailPageVM
8. HTML 流式返回（含首屏数据）
9. Client 子组件 hydration 后，useQuery 按需 refetch /api/bff/v1/...
```

---

## 十四、部署拓扑

```
                    ┌─────────────────┐
                    │  Ingress/Nginx  │
                    └────────┬────────┘
                             │
              ┌──────────────┴──────────────┐
              │                             │
     ┌────────▼────────┐         ┌────────▼────────┐
     │  Next.js :3000  │         │  Go API :8081   │
     │  · 静态/UI      │         │  · /api/v2/*    │
     │  · /api/bff/*   │────────▶│  · legacy/*     │
     └─────────────────┘  内网   └─────────────────┘
```

- 浏览器 **只访问 Next.js 域名**
- BFF → Go 走内网 Service，Upstream URL 不暴露给浏览器
- `upstream` 包的 `BASE_URL` 仅服务端环境变量

---

## 十五、Phase 0 最小骨架（BFF 优先）

```
Phase 0 交付物：
├── packages/bff-core/           createBffHandler + BffContext + 响应 envelope
├── packages/contracts/          common + identity 契约
├── packages/upstream/identity/  登录、session（调 Go v2 或 legacy）
├── packages/bff-modules/identity/login.ts
├── packages/web-sdk/            bffFetch 基座
├── apps/web/app/api/bff/v1/auth/login/route.ts
├── apps/web/app/(auth)/login/page.tsx
└── apps/web/src/modules/identity/
```

**验收**：登录页 → BFF → Go，全程不直连 Go；contracts 类型贯穿 web-sdk → page。

---

## 十六、设计原则小结

1. **页面只认识 BFF 契约**，不认识 Go DTO。
2. **BFF 薄路由、厚用例**：`app/api/bff` 只做转发，`bff-modules` 做编排。
3. **upstream 服务端专用**，Client 永不 import。
4. **RSC 直调 bff-modules**，Client 走 `/api/bff`，同一套用例函数复用。
5. **绞杀期差异收敛在 bff-modules + upstream/legacy**，UI 零感知。
6. **目录与后端 BC 同构**，降低全栈协作成本。
