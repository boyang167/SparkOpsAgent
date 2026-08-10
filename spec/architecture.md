# Agent Cloud Plaform 

## Value


## Requirements
1. Multi-Tenant
2. Multi-Agent
3. WorkFlow Automaztion 
4. MCP
5. 企业知识库
6. Memory
8. Human in the loop 
9. 插件市场
10. 企业权限体系
11. 可观测： logging , metric , Trace
12. 私有化部署


## Core Function 




# Desiger 

## Multi-Tenant

采用 组织（Organization）→ 工作区（Workspace）→ 项目（Project）→ Agent 四层模型
Organization
    │
    ├── Workspace
    │      │
    │      ├── Project
    │      │      │
    │      │      ├── Agent
    │      │      ├── Workflow
    │      │      ├── Knowledge
    │      │      ├── MCP
    │      │      └── Memory
每层都可以配置：

用户
角色
配额
模型
工具
知识库
网络策略
成本预算
Agent Center




- Presentation Layer 
    Chat Ui | Ai Copilot | Workflow Studio | Agent Studio | Prompt Studio | Marketplace | Admin | Open API | SDK
- API GateWay /BFF 
    Authenntication | OAuth | SSO | RateLimit | Streaming | Websocket | REST | GraphQL | APi Key | Audit 
- Control Plane 
    - Tanant Center
    - User Center
    - Organization center
    - WorkSpace Center
    
    - RBAC / ABAC / Policy Engine 
    - Agent Center 
    - WorkFlow Center
    - Prompt Center
    - Memory Center
    - Knowledge Center
    - MCP Center
    - Tool Center
    - Model Center
    - Secret Center
    - Config Center
    - Ai Governance 
    - AgentOps
    - Billing
    - Audit 
    - Notifaction 

- Data Plane
    - Agent Runtime Center
        Planner
        Reflection 
        Critic
        Executor
        Tool Executor
        MCP Client
        WorkFlow Engine
        Meomry Manager
        Context Builder
    - Multi-Agent-Scheduler
        Research Agent
        Coding Agent
        QA Agent
        Reviewer Agent
        Planner Agent
        Custom Agent 
    
LLM Gateway
Prompt Engine
Memory Engine
Knowledge Engine
Workflow Engine
Tool Engine
MCP Engine
Human Approval Engine
Event Bus

OpenAI
Claude
Gemini
DeepSeek
Qwen
Llama
vLLM
Ollama

Redis
Kafka
Postgres
Neo4j
Milvus
MinIO
ElasticSearch

Kubernetes
Istio
Prometheus
Grafana
Jaeger
OpenTelemetry
ArgoCD
Helm







┌─────────────────────────────────────────────────────────────────┐
│                    React / Next.js 前端                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    FastAPI 应用（单进程）                       │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │                      API 网关层                           │ │
│  │  (路由、鉴权、限流、OpenAPI)                              │ │
│  └───────────────────────────────────────────────────────────┘ │
│                              │                                  │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │                   应用层 (Use Cases)                      │ │
│  │  各模块的 Application Service 协调领域对象                │ │
│  └───────────────────────────────────────────────────────────┘ │
│                              │                                  │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │                    领域层 (Domain)                        │ │
│  │  聚合根 | 实体 | 值对象 | 领域事件 | 仓储接口            │ │
│  └───────────────────────────────────────────────────────────┘ │
│                              │                                  │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │                  基础设施层 (Infrastructure)               │ │
│  │  仓储实现 | 消息总线 | 缓存 | 外部API客户端               │ │
│  └───────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
    PostgreSQL          Redis/Vector         MinIO/S3
    (主数据)            (缓存/向量)          (文件存储)


# Tech Stack 

API	FastAPI
Agent Runtime	LangGraph（状态机）+ 自研 Runtime Orchestrator
Workflow	LangGraph + 自定义节点体系（复杂长流程可结合 Temporal）
LLM Gateway	LiteLLM 或自研 Provider Router
ORM	SQLAlchemy 2.x
配置	Pydantic Settings
异步	asyncio
MQ	Kafka（事件）
缓存	Redis
向量数据库	pgvector（默认）或 Qdrant
对象存储	MinIO / S3 / COS
可观测性	OpenTelemetry + Prometheus + Jaeger
部署	Kubernetes + Helm