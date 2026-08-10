一、整体架构愿景
text

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

二、项目目录结构（核心）
text

enterprise-ai-platform/
│
├── app/
│   ├── main.py                         # FastAPI 入口
│   ├── api/                            # 全局 API 注册
│   │   ├── deps.py                     # 依赖注入
│   │   ├── middleware.py               # 中间件
│   │   └── routers/                    # 路由注册器
│   │       └── __init__.py
│   │
│   ├── core/                           # 核心基础设施
│   │   ├── config.py                   # Pydantic Settings
│   │   ├── database.py                 # SQLAlchemy 引擎
│   │   ├── redis.py                    # Redis 客户端
│   │   ├── event_bus.py                # 本地事件总线
│   │   ├── di_container.py             # 依赖注入容器
│   │   └── exceptions.py               # 全局异常
│   │
│   ├── common/                         # 通用组件
│   │   ├── types/                      # 共用类型
│   │   ├── utils/                      # 工具函数
│   │   └── constants/                  # 常量定义
│   │
│   ├── events/                         # 全局事件定义
│   │   ├── agent_events.py
│   │   ├── workflow_events.py
│   │   ├── knowledge_events.py
│   │   └── ...
│   │
│   ├── infrastructure/                 # 外部适配器
│   │   ├── repositories/               # 数据库仓储实现
│   │   │   ├── postgres/
│   │   │   └── redis/
│   │   ├── clients/                    # 外部服务客户端
│   │   │   ├── llm/
│   │   │   ├── vector_db/
│   │   │   └── object_storage/
│   │   └── message_queue/              # Kafka 适配（可选）
│   │
│   ├── plugins/                        # 插件框架
│   │   ├── manager.py                  # 插件管理器
│   │   ├── base.py                     # 插件基类
│   │   └── builtin/                    # 内置插件
│   │       ├── tools/
│   │       ├── models/
│   │       └── memories/
│   │
│   └── modules/                        # ★ 核心：所有业务模块
│       ├── tenant/                     # 多租户
│       │   ├── api/                    # 路由
│       │   ├── application/            # 用例
│       │   ├── domain/                 # 领域模型
│       │   ├── infrastructure/         # 模块专属基础设施
│       │   ├── repositories/           # 仓储接口
│       │   └── events.py               # 模块事件
│       │
│       ├── auth/                       # 认证授权
│       ├── user/                       # 用户管理
│       ├── workspace/                  # 工作空间
│       │
│       ├── agent/                      # Agent 管理
│       │   ├── domain/
│       │   │   ├── agent.py            # 聚合根
│       │   │   ├── version.py          # 实体
│       │   │   └── template.py         # 值对象
│       │   ├── application/
│       │   │   ├── create_agent.py
│       │   │   ├── publish_agent.py
│       │   │   └── ...
│       │   └── infrastructure/
│       │       └── repositories/
│       │
│       ├── runtime/                    # ★ 最核心：运行时引擎
│       │   ├── domain/
│       │   │   ├── session.py
│       │   │   ├── state.py
│       │   │   └── result.py
│       │   ├── application/
│       │   │   └── runtime_service.py  # 编排所有 Engine
│       │   └── engines/                # 各个 Engine
│       │       ├── planner.py
│       │       ├── scheduler.py
│       │       ├── executor.py
│       │       ├── tool.py
│       │       ├── memory.py
│       │       ├── reflection.py
│       │       └── output.py
│       │
│       ├── workflow/                   # 工作流
│       │   ├── domain/
│       │   │   ├── graph.py
│       │   │   ├── node.py
│       │   │   └── edge.py
│       │   └── engine/
│       │       ├── compiler.py
│       │       └── executor.py
│       │
│       ├── prompt/                     # Prompt 管理
│       ├── llm/                        # LLM 网关
│       ├── tool/                       # 工具注册
│       ├── mcp/                        # MCP 协议
│       ├── knowledge/                  # 知识库
│       ├── memory/                     # 记忆系统
│       ├── governance/                 # 治理
│       ├── agentops/                   # Agent 运维
│       ├── audit/                      # 审计
│       └── billing/                    # 计费
│
├── scripts/                            # 运维脚本
├── deployment/                         # K8s/Helm
│   ├── docker/
│   └── kubernetes/
├── docs/                               # 文档
├── tests/                              # 全局测试
│   ├── unit/
│   ├── integration/
│   └── e2e/
└── pyproject.toml                      # 依赖管理

三、模块设计规范（DDD 六边形）

每个 modules/* 模块必须遵循统一结构：
text

module_name/
├── api/                       # 入站适配器（HTTP）
│   ├── routes.py
│   └── schemas.py             # 请求/响应模型
│
├── application/               # 应用层（用例）
│   ├── services.py            # 应用服务
│   ├── commands.py            # 命令对象
│   ├── queries.py             # 查询对象
│   └── handlers.py            # 事件处理器
│
├── domain/                    # 领域层（核心）
│   ├── entities.py            # 实体
│   ├── aggregates.py          # 聚合根
│   ├── value_objects.py       # 值对象
│   ├── events.py              # 领域事件
│   ├── exceptions.py          # 领域异常
│   └── interfaces.py          # 仓储接口
│
├── infrastructure/            # 出站适配器（实现）
│   ├── repositories.py        # 仓储实现
│   └── mappers.py             # ORM 映射
│
└── __init__.py

示例：Agent 模块
python

# modules/agent/domain/aggregates.py
from dataclasses import dataclass
from datetime import datetime
from typing import List, Optional
from .events import AgentPublished, AgentArchived

@dataclass
class Agent:
    id: str
    name: str
    description: str
    version: str
    status: AgentStatus  # Draft | Test | Published | Archived
    config: dict
    created_at: datetime
    updated_at: datetime
    
    def publish(self) -> None:
        if self.status != AgentStatus.TEST:
            raise InvalidAgentStateError("Agent must be in TEST state")
        self.status = AgentStatus.PUBLISHED
        self.updated_at = datetime.now()
        self.add_event(AgentPublished(self.id, self.version))
    
    def archive(self) -> None:
        self.status = AgentStatus.ARCHIVED
        self.add_event(AgentArchived(self.id))

四、Runtime 引擎设计（核心）

Runtime 不是"一个服务"，而是一组引擎的编排者：
python

# modules/runtime/application/runtime_service.py

class RuntimeService:
    def __init__(
        self,
        context_engine: ContextEngine,
        planner_engine: PlannerEngine,
        scheduler_engine: SchedulerEngine,
        executor_engine: ExecutorEngine,
        tool_engine: ToolEngine,
        memory_engine: MemoryEngine,
        reflection_engine: ReflectionEngine,
        output_engine: OutputEngine
    ):
        self.context = context_engine
        self.planner = planner_engine
        self.scheduler = scheduler_engine
        self.executor = executor_engine
        self.tool = tool_engine
        self.memory = memory_engine
        self.reflection = reflection_engine
        self.output = output_engine
    
    async def run(self, request: RuntimeRequest) -> RuntimeResult:
        # 1. 构建上下文
        ctx = await self.context.build(request)
        
        # 2. 规划任务
        plan = await self.planner.plan(ctx)
        
        # 3. 调度执行
        async for step in self.scheduler.schedule(plan):
            # 4. 执行任务
            result = await self.executor.execute(step)
            
            # 5. 调用工具
            if step.requires_tool:
                tool_result = await self.tool.invoke(step.tool_call)
                result = tool_result
            
            # 6. 记忆更新
            await self.memory.remember(ctx, result)
            
            # 7. 反思（可选）
            if self.reflection.should_reflect(step):
                await self.reflection.reflect(ctx, result)
        
        # 8. 生成输出
        return await self.output.generate(ctx)

每个 Engine 都是可插拔的插件：
python

# engines/planner.py
from app.plugins.base import Plugin

class PlannerEngine(Plugin):
    """规划引擎接口"""
    
    async def plan(self, context: RuntimeContext) -> ExecutionPlan:
        raise NotImplementedError
    
    async def replan(self, plan: ExecutionPlan, feedback: dict) -> ExecutionPlan:
        raise NotImplementedError

# 具体实现：ReAct Planner
class ReActPlanner(PlannerEngine):
    def __init__(self, llm_gateway: LLMGateway):
        self.llm = llm_gateway
    
    async def plan(self, context):
        prompt = self._build_react_prompt(context)
        response = await self.llm.chat(prompt)
        return self._parse_plan(response)

五、插件架构（核心扩展点）

所有可扩展能力统一为插件：
python

# plugins/base.py
from abc import ABC, abstractmethod
from typing import Any, Dict

class Plugin(ABC):
    name: str
    version: str
    metadata: Dict[str, Any]
    
    @abstractmethod
    async def initialize(self) -> None:
        """插件初始化"""
        pass
    
    @abstractmethod
    async def execute(self, input: Any) -> Any:
        """执行核心逻辑"""
        pass
    
    async def shutdown(self) -> None:
        """清理资源"""
        pass

# 插件类型
class ToolPlugin(Plugin): ...
class ModelPlugin(Plugin): ...
class MemoryPlugin(Plugin): ...
class RetrieverPlugin(Plugin): ...

插件管理器：
python

# plugins/manager.py
class PluginManager:
    def __init__(self):
        self._plugins: Dict[str, Plugin] = {}
        self._registry = PluginRegistry()
    
    async def load_plugin(self, plugin_path: str):
        """动态加载插件"""
        spec = importlib.util.spec_from_file_location(...)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        
        plugin_class = getattr(module, 'Plugin')
        plugin = plugin_class()
        await plugin.initialize()
        self._plugins[plugin.name] = plugin
    
    def get_tool(self, name: str) -> ToolPlugin:
        return self._plugins.get(name)

六、事件驱动（模块间解耦）

即使单体，也采用事件驱动：
python

# core/event_bus.py
from typing import Callable, Dict, List
import asyncio

class EventBus:
    def __init__(self):
        self._handlers: Dict[str, List[Callable]] = {}
        self._queue = asyncio.Queue()
    
    def subscribe(self, event_type: str, handler: Callable):
        self._handlers.setdefault(event_type, []).append(handler)
    
    async def publish(self, event: Any):
        event_type = event.__class__.__name__
        for handler in self._handlers.get(event_type, []):
            asyncio.create_task(handler(event))
    
    async def start_consumer(self):
        """后台消费事件"""
        while True:
            event = await self._queue.get()
            await self.publish(event)

领域事件示例：
python

# events/agent_events.py
from dataclasses import dataclass

@dataclass
class AgentPublished:
    agent_id: str
    version: str

@dataclass
class AgentArchived:
    agent_id: str

# modules/knowledge/application/handlers.py
class KnowledgeAgentHandler:
    def __init__(self, knowledge_service: KnowledgeService):
        self.knowledge = knowledge_service
    
    async def on_agent_published(self, event: AgentPublished):
        """Agent发布时，刷新知识库索引"""
        await self.knowledge.index_agent_knowledge(event.agent_id)

七、数据库设计（Schema 隔离）

所有表按 PostgreSQL Schema 逻辑隔离：
sql

-- 租户
CREATE SCHEMA tenant;
CREATE TABLE tenant.tenants (...);

-- Agent
CREATE SCHEMA agent;
CREATE TABLE agent.agents (...);
CREATE TABLE agent.versions (...);
CREATE TABLE agent.templates (...);

-- Workflow
CREATE SCHEMA workflow;
CREATE TABLE workflow.workflows (...);
CREATE TABLE workflow.nodes (...);
CREATE TABLE workflow.edges (...);

-- Memory
CREATE SCHEMA memory;
CREATE TABLE memory.conversations (...);
CREATE TABLE memory.episodes (...);

-- Knowledge
CREATE SCHEMA knowledge;
CREATE TABLE knowledge.documents (...);
CREATE TABLE knowledge.chunks (...);

-- Governance
CREATE SCHEMA governance;
CREATE TABLE governance.policies (...);
CREATE TABLE governance.audit_logs (...);

-- AgentOps
CREATE SCHEMA agentops;
CREATE TABLE agentops.evaluations (...);
CREATE TABLE agentops.metrics (...);

八、技术栈（单体版）
层级	技术选型
Web框架	FastAPI
ORM	SQLAlchemy 2.x + Alembic
数据验证	Pydantic v2
依赖注入	FastAPI DI + 自定义容器
异步	asyncio + AnyIO
缓存	Redis (aioredis)
向量数据库	pgvector (PostgreSQL扩展)
对象存储	MinIO / S3 (boto3)
事件总线	本地内存 (后期可换为Kafka)
任务队列	Celery / ARQ (异步RQ)
可观测性	OpenTelemetry + Prometheus
配置管理	Pydantic Settings
测试	pytest + httpx + pytest-asyncio
部署	Docker + Kubernetes (单Pod)
九、启动方式
python

# app/main.py
from fastapi import FastAPI
from app.core.config import settings
from app.core.database import init_db
from app.core.event_bus import EventBus
from app.api import register_routers

app = FastAPI(title="Enterprise AI Platform")

# 初始化
@app.on_event("startup")
async def startup():
    await init_db()
    await init_redis()
    await load_plugins()
    await start_event_consumer()

# 注册所有模块路由
register_routers(app)

# 启动
# uvicorn app.main:app --reload --host 0.0.0.0 --port 8000

十、演进路径（何时拆微服务）
text

阶段1（团队 5-10人）: 模块化单体
              ↓
阶段2（Agent > 1000）: 按模块拆分为微服务
              ↓
阶段3（规模扩展）: Runtime 拆分为独立引擎服务
              ↓
阶段4（超大规模）: 事件溯源 + CQRS

拆分成本几乎为零，因为：

    每个模块已有独立的 Domain 和 Repository

    模块间通过事件通信，已解耦

    每个模块的 API 路由独立注册

    Infrastructure 层已抽象

只需将 modules/runtime 等模块独立部署，并替换 EventBus 为 Kafka/RabbitMQ 即可。
十一、下一步行动清单

    搭建项目骨架（按照上述目录创建）

    定义核心领域模型（Agent、Workflow、Runtime Session）

    实现基础 Engine（Planner、Scheduler、Executor）

    集成 LLM Gateway（支持多模型）

    实现 Tool 插件系统（先内置 Python/Shell/HTTP 工具）

    添加 Memory 和 Knowledge 基础能力

    前端对接 API

    部署 K8s 单服务 Helm Chart

这个方案既保证了企业级架构的严谨性，又避免了微服务的早期运维负担。可以直接进入编码阶段。
