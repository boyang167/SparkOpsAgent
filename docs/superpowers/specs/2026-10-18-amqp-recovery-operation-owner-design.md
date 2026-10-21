# AMQP 任务队列恢复与嵌套 Operation 身份设计

## 问题

SparkOps 的可靠领域事件传输可在 RabbitMQ 连接中断后重连，但现有任务 Queue 仅在启动时建立连接。Broker 重启或网络闪断后，`deploy.sparkops` 等消费者永久消失，发布持续返回连接关闭；部署 Reconciler 又将该临时故障计入三次部署基础设施预算，最终把运行标记失败。

产品工作流调度器会在带有 `product-workflow.scheduler` 事件元数据的事务中调用创建部署。创建部署当前固定使用 `delivery.command.create-deployment` 作为 Operation owner，违反“事务内 Operation owner 必须等于当前 subscriber”的一致性约束。

## 设计

### 任务 Queue 恢复

- AMQP Queue 保存稳定订阅描述，而不是只保存一次性 consumer channel。
- 发布发现连接或 channel 已关闭时，在同一 Queue 实例内串行重建连接、交换机和发布 channel，然后重试一次当前发布。
- RabbitMQ 通知连接关闭后，后台恢复循环按有界退避重连，并为全部稳定订阅重新建立独立 channel。
- 一个订阅的恢复失败不丢失订阅描述；关闭 Queue 必须取消恢复循环并等待退出。
- 消息处理、三次任务级重试和 DLQ 语义保持不变。

### 部署补发

- AMQP 模式下部署 Reconciler 不直接发布 RabbitMQ，而是通过 Operation Outbox 调度 `delivery.deploy.queue-publish`。
- RabbitMQ 暂时不可用只使 Operation 保持待重试，不消耗部署运行的三次基础设施预算。
- `inmemory` 模式继续直接使用内存 Queue，不依赖 Operation Outbox。

### 嵌套 Operation 身份

- 创建、回滚和卸载部署在存在事件元数据时继承当前 `EventID` 与 `Subscriber`。
- 在普通 HTTP 命令上下文中继续使用各自稳定的命令 owner，并以部署运行 ID 构造稳定事件 ID。
- 幂等键仍为 `eventID:owner:operationType:target`，不放宽 OperationWriter 的 owner 校验。

### 配置

- 本地 RabbitMQ 实际连接为 `192.168.5.24:5672`，默认 vhost `/`。
- `virtual_host` 独立字段暂不加入运行时配置模型；当前 AMQP URL 路径是唯一生效来源，避免双重配置冲突。

## 验证

- 单元测试先复现嵌套 owner 冲突，再验证继承后的 Operation 记录。
- 单元测试先复现 Queue 断线后无法发布/恢复订阅，再验证重连。
- 使用真实 RabbitMQ 验证连接中断后的同实例发布和消费恢复。
- 回归部署命令、工作流调度、Operation Outbox 和消息平台测试。
