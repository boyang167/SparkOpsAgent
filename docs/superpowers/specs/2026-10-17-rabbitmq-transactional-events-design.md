# RabbitMQ 事务事件与最终一致性设计

## 背景

SparkOps 当前将领域事件总线固定为进程内实现。`messaging.driver: amqp` 只会把异步任务队列切换到 RabbitMQ，领域事件仍由 `NewInMemoryBus()` 同步分发。因此事件不能跨实例传播，进程崩溃会丢失尚未处理的事件，也无法在业务数据提交与事件发布之间提供一致性保证。

系统已有构建完成、部署完成、工作流完成、工作区创建、密钥更新、Agent 发布和运行完成等事件。这些事件由多个业务模块消费，部分消费者修改本地数据库，部分消费者进一步调用 Jenkins、Kubernetes 或 Webhook。

## 目标

- 所有现有领域事件和集成事件通过 RabbitMQ 跨实例传递。
- 业务数据与待发布事件在同一个 PostgreSQL 事务中提交，不因进程崩溃或 RabbitMQ 暂时不可用而丢失事件。
- 每个逻辑订阅者都收到事件；同一逻辑订阅者的多个服务实例竞争消费。
- RabbitMQ 至少一次投递时，本系统数据库内的业务副作用只生效一次。
- 临时故障持续自动重试，恢复后最终收敛。
- 永久错误进入 DLQ，能够查询、告警并使用原事件 ID 安全重放。
- 所有现有事件发布路径和订阅者一次性迁移，不长期混用可靠与不可靠发布。

## 一致性定义与边界

本设计中的“严格最终一致性”具体指：

- 一个业务事务只要成功提交，对应 Outbox 事件就一定持久化。
- 持久化事件会持续投递，直到 RabbitMQ 确认接收或系统仍在重试。
- RabbitMQ 可以重复投递，但本系统数据库内同一逻辑订阅者的业务效果只提交一次。
- 临时故障解除后，所有可处理事件最终被各逻辑订阅者处理。

PostgreSQL 事务不能与 Jenkins、Kubernetes、Webhook 等外部系统形成原子事务。外部副作用采用 Operation Outbox、稳定幂等键、状态查询和 Reconciler 实现最终收敛：

- 外部 API 支持幂等键时，使用事件 ID 或派生操作 ID 保证效果一次。
- 外部 API 不支持幂等键时，执行前查询状态、使用持久化状态机并定期对账，不能承诺数学意义上的绝对一次。
- Webhook 携带稳定事件 ID，由接收方负责幂等。

## 非目标

- 不引入 Kafka、Debezium 或其他 CDC 基础设施。
- 不实现 PostgreSQL 与外部系统之间的分布式事务。
- 不承诺 RabbitMQ 只投递一次；系统按至少一次投递设计。
- 不在本阶段建设 DLQ 管理前端。
- 不删除现有异步任务 `Queue`；任务队列和领域事件总线保持独立职责。

## 方案选择

采用“Transactional Outbox + RabbitMQ + Transactional Inbox”。

未采用的方案：

- 直接复用现有 AMQP `Queue`：它按主题创建单一队列，只能竞争消费，不能保证每个逻辑订阅者都收到事件。
- 只使用 RabbitMQ Publisher Confirm：只能确认消息进入 RabbitMQ，不能原子保证业务事务和消息发布。
- PostgreSQL CDC + Debezium：可以降低发布端侵入，但会新增 Kafka Connect/Debezium 运维体系，与当前 RabbitMQ 技术栈不匹配。

## 总体架构

发布链路：

1. 应用服务开启 PostgreSQL 事务。
2. 业务仓储使用当前事务写入业务数据。
3. 事件发布器在同一事务写入 `event_outbox`。
4. 事务提交后，Outbox Dispatcher 领取待发送记录。
5. Dispatcher 将事件发布到 RabbitMQ durable topic exchange。
6. 收到 Publisher Confirm 后，将 Outbox 标记为已发布。

消费链路：

1. 每个“事件类型 + 逻辑订阅者”使用独立持久化队列。
2. 同一逻辑订阅者的多个后端实例消费同一个队列。
3. 消费者开启 PostgreSQL 事务并尝试插入 Inbox 记录。
4. Inbox 已存在时直接提交并 ACK，不重复执行 handler。
5. Inbox 不存在时，在同一事务执行本地数据库副作用并写入 Inbox。
6. 事务成功后 ACK；失败时进入重试链路。
7. 外部副作用只在消费事务内写入 Operation Outbox，由独立执行器完成。

## RabbitMQ 拓扑

- Exchange：`sparkops.events`。
- 类型：`topic`。
- 属性：durable、非 auto-delete。
- 路由键：带版本的事件类型，例如 `delivery.BuildCompleted.v1`。
- Message ID：Outbox 事件 UUID。
- 消息持久化：`delivery_mode=2`。
- 发布：启用 Publisher Confirm。

每个订阅队列使用稳定名称：

`sparkops.event.<subscriber>.<event-type>.v<version>`

例如通知模块和产品模块订阅同一个构建完成事件时拥有不同队列。通知模块的所有后端实例共同消费通知队列，产品模块的所有实例共同消费产品队列。

队列在 Dispatcher 启动前一次性声明和绑定，避免交换机已接收事件但订阅队列尚不存在。

## 事件信封与类型注册

RabbitMQ 消息使用统一事件信封：

- `id`：全局 UUID。
- `type`：稳定事件名。
- `version`：正整数版本。
- `aggregate_type`。
- `aggregate_id`。
- `aggregate_version`：同一聚合的单调版本；不适用时为空。
- `occurred_at`：UTC 时间。
- `correlation_id` 和 `causation_id`。
- `payload`：事件 JSON。

新增显式事件类型注册表，负责把信封反序列化为现有具体 Go 事件类型，避免破坏 handler 的类型断言。新增字段必须保持向后兼容；破坏性变化发布新版本，并在过渡期同时支持旧版本消费者。

未知事件类型、未知版本和无法解析的载荷属于永久错误，进入 DLQ 而不是无限重试。

## 数据模型

### platform.event_outbox

- `id UUID PRIMARY KEY`
- `event_type`
- `event_version`
- `aggregate_type`
- `aggregate_id`
- `aggregate_version`
- `payload JSONB`
- `occurred_at`
- `correlation_id`
- `causation_id`
- `status`：`pending`、`publishing`、`published`
- `attempts`
- `next_attempt_at`
- `locked_at`
- `locked_by`
- `published_at`
- `last_error`
- `created_at`

为 `(status, next_attempt_at)` 和锁租约字段建立索引。

### platform.event_inbox

- `event_id UUID`
- `subscriber`
- `event_type`
- `processed_at`
- `created_at`
- 主键或唯一键：`(event_id, subscriber)`

Inbox 去重记录默认不删除，保证任意时间重放仍然安全。未来如需控制数据量，应归档完整记录并保留可查询的唯一去重账本，不能直接按时间删除。

### platform.operation_outbox

- `id UUID PRIMARY KEY`
- `event_id`
- `subscriber`
- `operation_type`
- `idempotency_key`
- `payload JSONB`
- `status`
- `attempts`
- `next_attempt_at`
- `locked_at`
- `locked_by`
- `completed_at`
- `last_error`
- `created_at`

`idempotency_key` 唯一。外部执行器使用与事件 Dispatcher 相同的租约和恢复原则。

## 稳定逻辑订阅者

现有生产订阅使用以下稳定名称，名称发布后不得随 Go 包、类型或函数重命名而改变：

- `product.build-completed`
- `product.workflow-run-completed`
- `product.deploy-completed`
- `product-workflow.build-completed`
- `product-workflow.deploy-completed`
- `workspace-app.workspace-created.seed-default-templates`
- `workspace-app.deploy-completed.archive-coordinator`
- `delivery.build-completed.deployrun`
- `delivery.build-completed.webhook`
- `delivery.deploy-completed.webhook`
- `delivery.secret-updated.credential-sync`
- `notification.run-completed`
- `notification.workflow-run-completed`
- `notification.build-completed`
- `governance.workspace-created.bootstrap`

新增订阅者必须显式声明稳定名称，启动时拒绝重复的“事件类型 + 订阅者名”注册。

## 事务边界

引入统一事务协调器，将当前 GORM 事务放入 `context.Context`。所有参与可靠事件发布或消费的仓储必须优先使用上下文中的事务句柄。

发布规则：

- 业务数据和 Outbox 事件必须在同一事务写入。
- 事务回滚时，业务数据和 Outbox 一并回滚。
- 可靠业务事件不得在事务提交后直接调用 RabbitMQ。
- 所有现有发布路径都迁移到事务发布器。

消费规则：

- Inbox 插入、handler 的本地数据库副作用和衍生 Outbox/Operation Outbox 必须在同一事务。
- handler 不直接执行不可回滚的外部调用。
- Inbox 唯一键冲突表示该逻辑订阅者已经处理，直接 ACK。
- 数据库事务提交前不得 ACK RabbitMQ 消息。

## 分发、锁租约与多实例

Dispatcher 使用 `FOR UPDATE SKIP LOCKED` 批量领取到期记录，允许多实例并行工作。领取时写入 `publishing`、`locked_at` 和 `locked_by`，但不在数据库锁事务中等待 RabbitMQ 网络操作。

Publisher Confirm 成功后标记 `published`。若 RabbitMQ 已确认但进程在更新 Outbox 前崩溃，锁租约到期后事件会再次发布；Inbox 负责消除重复效果。

启动时和周期任务会把超过锁租约的 `publishing` 记录恢复为可领取状态。RabbitMQ 不可用时，业务事务仍可提交并积压 Outbox。

## 重试与 DLQ

临时错误持续自动重试。RabbitMQ 不依赖额外插件，使用 TTL + dead-letter exchange 构建固定级别重试队列：

- 5 秒
- 30 秒
- 5 分钟
- 30 分钟
- 之后每 30 分钟重试，直到成功或被判定为永久错误

消息保留重试次数和最近错误分类，但事件 ID 保持不变。

永久错误包括：

- 未注册事件类型或版本。
- 无法反序列化的事件信封或载荷。
- 明确违反不可恢复业务约束的事件。

永久错误进入对应逻辑订阅者的 DLQ，保留原消息、事件 ID、订阅者、失败原因和诊断时间。提供受控 CLI 或管理命令查询和重放 DLQ；修复原因后使用原事件 ID 重放，依赖 Inbox 保证安全。

## 顺序与收敛

RabbitMQ 重试和多实例消费不能提供全局顺序。需要顺序约束的聚合事件携带 `aggregate_version`：

- 低于消费者已处理版本的事件视为旧事件，不覆盖新状态。
- 高于期望版本且存在缺口时按临时错误延迟重试。
- 消费者优先根据事件标识读取当前事实状态，而不是盲目覆盖状态。

不需要聚合顺序的通知类事件不使用版本游标。

## EventBus 接口与生命周期

现有 `Subscribe(eventName, handler)` 无法表达稳定逻辑订阅者。新接口必须显式传入订阅者名，并具有生命周期：

- 注册事件类型和逻辑订阅者。
- 声明 RabbitMQ 拓扑。
- 启动消费者。
- 启动 Outbox Dispatcher。
- 优雅停止 Dispatcher 和消费者。

`messaging.driver: inmemory` 保留给单元测试和轻量本地开发，但不宣称具备可靠性。生产环境强制使用 `amqp + outbox`，且配置校验拒绝不可靠组合。

AMQP 配置保留现有任务交换机，并为事件增加独立配置：

```yaml
messaging:
  driver: amqp
  amqp:
    url: "amqp://user:password@host:5672/"
    exchange: sparkops
    exchange_type: direct
    event_exchange: sparkops.events
  outbox:
    poll_interval: 1s
    batch_size: 100
    lock_timeout: 1m
```

`exchange` 继续服务现有异步任务 Queue；`event_exchange` 固定使用 topic 类型。首期重试层级按本设计固定，不增加无必要的动态配置。

## 启动与关闭

启动顺序：

1. 连接 PostgreSQL。
2. 执行 Outbox、Inbox 和 Operation Outbox 迁移。
3. 连接 RabbitMQ 并声明 durable exchange。
4. 初始化模块并注册全部事件类型和逻辑订阅者。
5. 声明全部业务队列、重试队列和 DLQ。
6. 启动消费者。
7. 启动 Outbox Dispatcher 和 Operation Dispatcher。
8. 开始对外提供服务。

关闭顺序：

1. 停止领取新 Outbox 和 Operation Outbox。
2. 等待在途 Publisher Confirm。
3. 停止消费者领取新消息。
4. 等待当前数据库事务完成。
5. 关闭 RabbitMQ channel 和 connection。

## 故障窗口

- 业务事务回滚：业务数据和 Outbox 均不存在。
- 业务事务提交后进程崩溃：Outbox 仍在，其他实例或重启后的 Dispatcher 补发。
- RabbitMQ 确认前断线：Outbox 保持未发布并重试。
- RabbitMQ 确认后、Outbox 标记前崩溃：事件重复发布，由 Inbox 去重。
- 消费事务提交前崩溃：事务回滚，消息重投。
- 消费事务提交后、ACK 前崩溃：消息重投，Inbox 命中后直接 ACK。
- 外部操作成功后、Operation Outbox 标记前崩溃：使用幂等键或状态查询避免重复效果；不支持幂等的外部 API 由 Reconciler 收敛。

## 可观测性与运维

指标：

- Outbox 待发布数量和最老记录年龄。
- Outbox 发布尝试、失败率和 Publisher Confirm 延迟。
- 各订阅者消费延迟、重试次数和永久错误数。
- DLQ 数量。
- Inbox 重复命中数。
- Operation Outbox 积压、失败和最老记录年龄。

结构化日志统一包含：

- `event_id`
- `event_type`
- `event_version`
- `subscriber`
- `aggregate_id`
- `attempt`
- `correlation_id`

RabbitMQ 故障不使 HTTP readiness 失败，因为 Outbox 能安全积压，但必须暴露 degraded 指标和告警。PostgreSQL 不可用时 readiness 失败。

## 迁移策略

- 一次性为所有现有事件定义稳定类型、版本、序列化器和订阅者名。
- 逐条把现有“保存后 Publish”改为“事务内保存 + Outbox”。
- 逐个把消费者数据库副作用纳入 Inbox 事务。
- 把消费者中的 Jenkins、Kubernetes 和 Webhook 调用改为 Operation Outbox。
- 自动化检查或代码约束禁止业务模块绕过事务发布器。
- 全部路径迁移并通过集成测试后，生产配置切换为可靠 AMQP EventBus。
- `inmemory` 仅保留测试用途，不作为生产降级路径。

## 测试

单元测试覆盖：

- 事件信封序列化、反序列化和版本注册。
- 业务事务提交与回滚时 Outbox 的原子性。
- Inbox 唯一键去重。
- 临时错误、永久错误和重试层级分类。
- 锁租约领取、过期恢复和多实例竞争。
- 聚合版本的旧事件与版本缺口处理。

PostgreSQL + RabbitMQ 集成测试覆盖：

- 每个发布和消费故障窗口。
- RabbitMQ 停机期间积压、恢复后补发。
- Publisher Confirm 后故障造成的重复消息。
- 消费提交后、ACK 前故障的 Inbox 去重。
- 不同逻辑订阅者各自收到事件。
- 同一逻辑订阅者多实例只产生一次本地业务效果。
- 临时错误持续重试，永久错误进入 DLQ并可安全重放。
- Operation Outbox 的幂等执行和对账恢复。

所有现有事件发布路径和订阅者都要有迁移测试，禁止只验证新基础设施而遗漏业务链路。

## 验收标准

- 任一成功提交的业务事务都存在对应 Outbox 事件。
- RabbitMQ 中断不会导致业务事件丢失，恢复后自动补发。
- 每个逻辑订阅者最终收到事件。
- 重复发布和重复消费不会重复提交本系统数据库副作用。
- 临时失败持续重试；永久失败可观测、可诊断、可安全重放。
- 多实例部署不会造成订阅者遗漏或重复业务效果。
- 现有所有事件发布路径不再使用事务外可靠发布。
- 外部副作用经 Operation Outbox 执行，并按外部能力使用幂等键或 Reconciler 收敛。
- 生产环境不能使用 `inmemory` 事件总线。
