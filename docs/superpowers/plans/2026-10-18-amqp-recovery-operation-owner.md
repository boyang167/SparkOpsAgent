# AMQP Recovery and Operation Owner Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** RabbitMQ 闪断后任务发布与消费者自动恢复，并允许产品工作流安全地通过 Operation Outbox 创建部署。

**Architecture:** 任务 Queue 维护可重建的连接和稳定订阅注册表；发布路径串行恢复，消费路径由连接关闭通知触发恢复。部署命令的 Operation 身份优先继承当前事件元数据，HTTP 命令仍使用稳定命令身份。

**Tech Stack:** Go 1.26、amqp091-go、GORM、Testify、RabbitMQ 3.x。

## Global Constraints

- 不削弱 `OperationWriter` 对 EventID、Subscriber、owner 和幂等键的一致性校验。
- `inmemory` 行为保持不变。
- RabbitMQ 临时不可用不得把部署运行标记为永久失败。
- 只修改消息 Queue、部署 Operation 发布及对应测试和配置。

---

### Task 1: 嵌套部署 Operation 身份

**Files:**
- Modify: `backend/internal/modules/delivery/deployrun/application/command/operation_publish.go`
- Test: `backend/internal/modules/delivery/deployrun/application/command/create_deployment_test.go`

**Interfaces:**
- Consumes: `events.MetadataFromContext(context.Context)`.
- Produces: `deploymentOperationPublisher.schedule` 使用继承或命令生成的稳定身份。

- [ ] **Step 1: 写失败测试**

在带 `EventID=workflow-event-1`、`Subscriber=product-workflow.scheduler` 的事务上下文调用创建部署，断言保存的 Operation owner 为 `product-workflow.scheduler`，幂等键以前述 EventID 和 owner 开头。

- [ ] **Step 2: 验证 RED**

Run: `go test ./internal/modules/delivery/deployrun/application/command -run TestCreateDeploymentOperationInheritsEventMetadata -count=1`

Expected: FAIL，错误包含 `does not match event subscriber`。

- [ ] **Step 3: 最小实现**

在 `schedule` 中读取事件元数据；存在时使用元数据的 EventID/Subscriber，不存在时使用运行 ID 和命令 owner。按最终身份生成：

```go
eventID, owner := target, p.owner
if metadata, ok := events.MetadataFromContext(ctx); ok {
    eventID, owner = metadata.EventID, metadata.Subscriber
}
key := eventID + ":" + owner + ":" + outbound.OperationTypeDeployQueuePublish + ":" + target
```

- [ ] **Step 4: 验证 GREEN**

Run: `go test ./internal/modules/delivery/deployrun/application/command -count=1`

Expected: PASS。

### Task 2: AMQP Queue 自动恢复

**Files:**
- Modify: `backend/internal/platform/messaging/amqp.go`
- Create: `backend/internal/platform/messaging/amqp_queue_reconnect_test.go`

**Interfaces:**
- Produces: `amqpQueue.ensureConnected(context.Context) error`.
- Produces: 稳定订阅注册表，连接恢复后重建每个订阅。

- [ ] **Step 1: 写失败测试**

使用可控的连接/channel stub：首次连接关闭，第二次连接可用；断言 `Publish` 重拨并成功，订阅在连接关闭后重新调用 Consume。

- [ ] **Step 2: 验证 RED**

Run: `go test ./internal/platform/messaging -run 'TestAMQPQueueReconnects' -count=1`

Expected: FAIL，因为当前 Queue 无重拨入口且订阅永久退出。

- [ ] **Step 3: 最小实现**

为 Queue 注入拨号工厂，保存订阅描述；用互斥锁保证单次重连，使用 25ms 到 1s 有界退避；关闭时取消恢复 context、关闭连接并等待 goroutine。

- [ ] **Step 4: 验证 GREEN**

Run: `go test ./internal/platform/messaging -count=1`

Expected: PASS。

### Task 3: Reconciler 使用 Operation Outbox

**Files:**
- Modify: `backend/internal/modules/delivery/module.go`
- Modify: `backend/internal/modules/delivery/deployrun/application/service/reconciler.go`
- Test: `backend/internal/modules/delivery/deployrun/application/service/reconciler_test.go`

**Interfaces:**
- Consumes: `messaging.OperationWriter`.
- Produces: AMQP 模式补发只调度 Operation，不调用 RabbitMQ。

- [ ] **Step 1: 写失败测试**

模拟 RabbitMQ publisher 失败但 Operation 调度成功，断言运行的 `attempt_count` 不增加。

- [ ] **Step 2: 验证 RED**

Run: `go test ./internal/modules/delivery/deployrun/application/service -run TestReconcilerBrokerOutageDoesNotConsumeRunBudget -count=1`

Expected: FAIL，当前 Reconciler 调用 `RecordInfrastructureFailure`。

- [ ] **Step 3: 最小实现**

模块在 AMQP 模式向 Reconciler 注入 Operation-backed `RunPublisher`；inmemory 继续注入直接 Queue publisher。保留 Reconciler 对真正调度失败的报告，不把 RabbitMQ 执行失败同步回部署预算。

- [ ] **Step 4: 验证 GREEN**

Run: `go test ./internal/modules/delivery/deployrun/application/service ./internal/modules/delivery -count=1`

Expected: PASS。

### Task 4: 配置与最终验证

**Files:**
- Modify: `backend/config.yaml`
- Modify: `docs/superpowers/specs/2026-10-17-rabbitmq-transactional-events-design.md`

- [ ] **Step 1: 修正连接**

使用 `amqp://admin:<encoded-password>@192.168.5.24:5672/`；文档说明 vhost 由 URL 路径指定。

- [ ] **Step 2: 回归**

Run:

```powershell
go test ./internal/platform/messaging/... ./internal/modules/delivery/... ./internal/modules/product_workflow/... -count=1 -timeout 20m
go build ./...
```

Expected: PASS。

- [ ] **Step 3: 真实恢复验证**

确认 `deploy.sparkops` 有消费者，发布测试部署后不进入 DLQ；中断连接并恢复 Broker 后，同一后端实例重新出现消费者并继续处理。
