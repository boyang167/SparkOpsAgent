# Project Deploy Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让审核通过且部署绑定完整的项目创建独立 `DeploymentRun`，经 `deploy.sparkops` 异步完成 Jenkins CI、Harbor 镜像产出、Kubernetes Apply 与 Rollout Ready，并在项目工作台展示真实进度和历史。

**Architecture:** 在现有 Go 单体的 `delivery` 限界上下文下新增 `deployrun` 子包，以数据库状态机为唯一编排真相，RabbitMQ 消息只传 `run_id` 与 `workspace_id`；`workspace_app` ACL 提供项目、密钥和 **当前 workspace 已授权** 集群门禁快照，`runtime` 通过端口提供 Apply/等待能力。平台管理员通过 `platform_admin.workspace_clusters` 维护授权。Next.js 继续使用 contracts → upstream → BFF module/route → web-sdk → client component 的现有分层，不新增微服务。

**Tech Stack:** Go 1.26、Gin、GORM/PostgreSQL、RabbitMQ (`amqp091-go`)、gojenkins、Kubernetes client-go v0.32.3、Next.js 15、React 19、TypeScript 5.8、Zod 3、pnpm 9、Vitest（仅为前端纯逻辑/contract 单测新增）。

## Global Constraints

- `DeploymentRun` 独立于 `delivery.builds`；同一项目同时最多一条 `queued|preparing|building|deploying` Run。
- 状态只能 `queued → preparing → building → deploying → succeeded`，任意进行中状态可到 `failed|aborted`，禁止跳跃。
- `messaging.TopicDeploy = "deploy.sparkops"`；消息严格为 `{"run_id":"<uuid>","workspace_id":"<uuid>"}`，每次消费只推进一个阶段。
- 未知 Run、终态 Run、重复消息：业务 handler 返回 nil；基础设施 ACK。业务错误写 `failed` 后 handler 返回 nil。禁止 Nack、禁止 `basic.nack`/requeue 空转。
- AMQP `Consume` 使用 **manual ACK**（`autoAck=false`）。handler 返回 nil → ACK。handler 返回基础设施错误：读取 `x-sparkops-retry`（缺失视为 0）后 +1，若 `next <= 3` 则 republish 到原 topic（header 写入新值）并 ACK 原消息；若 `next > 3` 则 Publish 到 `DeadLetterTopic(topic)`（`<topic>.dlq`）并 ACK 原消息。`Queue` 接口签名保持 `Publish/Subscribe/Close`。
- 创建/中止部署使用现有 `authz.PermProjectOperate`，读取列表/详情/SSE/options 使用 `authz.PermProjectRead`。Workspace 集群授权的增删查使用 Super Admin（与现有 `/api/v1/admin/clusters` 一致）。
- 项目必须 `review_status=approved`；Git、Harbor、Jenkins、Kubernetes 密钥必填且属于同一 Workspace，`enable_static_scan=true` 时 Sonar 密钥也必填。
- 项目 `default_cluster_id` 与部署 `cluster_id` **必须** 存在于 `platform_admin.workspace_clusters` 且 `workspace_id` 等于当前工作空间。部署 options **只返回该 workspace 已授权集群**，禁止返回全局 `cluster_info` 列表。
- Kubernetes 密钥 **只支持** 解密后的 `credentials.extra_config` 完整 kubeconfig；本期不支持 token/CA 分字段拼装。`extra_config` 为空则拒绝创建/推进 Run。
- 未传 `image_tag` 时：`DefaultImageTag(run.ID)` = UUID 字符串去掉连字符后的前 12 位（例：`550e8400-e29b-41d4-a716-446655440000` → `550e8400e29b`）。禁止用 git SHA 或 `git_ref_value` 作为默认 tag。
- Jenkins Folder=`project.namespace`，Job=`project.name`；credential ID=`sparkops-{workspaceID}-{secretID}`。
- 构建参数至少包含 `BRANCH`、`CodeVersion`、`IMAGE_TAG`、`IMAGE_REPOSITORY`，Harbor 坐标来自密钥与项目名，不硬编码 `harbor.local`。
- Rollout 默认超时 10 分钟，Jenkins 默认超时 30 分钟；沿用 `DELIVERY_DRIVER`、`RUNTIME_DRIVER`、`MESSAGING_DRIVER`。
- 本期不做自动回滚、多集群并行/同步、真实 Jenkins/Harbor/Sonar/Kubernetes 外部 E2E；Abort 不删除已 Apply 资源。
- 第一阶段完整闭环必须能在 `delivery.driver=mock`、`runtime.driver=mock`、in-memory messaging 下演示。
- 不修改业务 Git 仓库中的 Jenkinsfile，不处理 `READEMD.md` 明文凭据债务，不改变现有 product → Build 路径。
- 本计划不含任何自动 `git commit` 步骤。

---

## File map

- `backend/migrations/000032_project_deploy_pipeline.{up,down}.sql`：项目绑定列、`workspace_clusters`、Run/Event 表、进行中唯一索引。
- `backend/internal/shared/kernel/ids.go`：新增 `DeploymentRunID`。
- `backend/internal/modules/platform_admin/{domain,application,infrastructure,interface}`：workspace 集群授权仓储与 Super Admin HTTP。
- `backend/internal/modules/delivery/deployrun/domain/{model,repository,port}`：状态机、仓储契约、跨上下文端口。
- `backend/internal/modules/delivery/deployrun/{application,infrastructure,interface}`：创建/查询/中止、消费编排、GORM、ACL、SSE、HTTP。
- `backend/internal/platform/messaging/{bus.go,amqp.go,noop_queue.go,consume.go}`：manual ACK、retry、DLQ。
- `backend/internal/modules/delivery/{domain/port,application/command,infrastructure/jenkins}`：Jenkins Job upsert 与参数化触发。
- `backend/internal/modules/runtime/{domain/port,infrastructure/k8s}`：manifest Apply、Rollout/Service 校验及 mock。
- `backend/internal/modules/workspace_app/{domain,infrastructure,interface}`：项目默认集群及 Jenkins/Kubernetes 密钥绑定。
- `fronted/packages/{contracts,upstream,bff-modules,web-sdk}` 与 `fronted/apps/web`：部署资源、授权管理 UI、工作台。

### Task 1: Migration、DeploymentRun 聚合与状态机

**Files:**
- Create: `backend/migrations/000032_project_deploy_pipeline.up.sql`
- Create: `backend/migrations/000032_project_deploy_pipeline.down.sql`
- Modify: `backend/internal/shared/kernel/ids.go`
- Create: `backend/internal/modules/delivery/deployrun/domain/model/deployment_run.go`
- Create: `backend/internal/modules/delivery/deployrun/domain/model/deployment_run_test.go`
- Create: `backend/internal/modules/delivery/deployrun/domain/repository/deployment_run_repository.go`
- Create: `backend/internal/modules/delivery/deployrun/domain/errors.go`

**Interfaces:**
- Consumes: `kernel.ProjectID`、`kernel.WorkspaceID`、`kernel.ClusterInfoID`、`kernel.MemberID`、`kernel.BuildID`。
- Produces: `kernel.DeploymentRunID`; `model.DefaultImageTag(id kernel.DeploymentRunID) string`; `model.NewDeploymentRun(wsID, projectID, clusterID, createdBy, gitRefType, gitRefValue, imageTag string) *DeploymentRun`（`imageTag` 为空则写入 `DefaultImageTag`）；`(*DeploymentRun).Transition(to Status, at time.Time) error`; `(*DeploymentRun).Fail(msg string, at time.Time)`; `repository.DeploymentRunRepository` with `Create`, `Update`, `FindByID`, `FindByBuildID`, `ListByProject`, `ListEvents`, `HasActiveByProject`。

- [ ] **Step 1: 先写非法跳跃、终态不可变与默认 image_tag 测试**

```go
func TestDefaultImageTagStripsHyphensAndTakesFirst12(t *testing.T) {
	id, err := kernel.ParseDeploymentRunID("550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	require.Equal(t, "550e8400e29b", DefaultImageTag(id))
}

func TestNewDeploymentRunUsesDefaultImageTagWhenEmpty(t *testing.T) {
	run := NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(),
		kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")
	require.Equal(t, DefaultImageTag(run.ID), run.ImageTag)
	require.Len(t, run.ImageTag, 12)
}

func TestNewDeploymentRunKeepsExplicitImageTag(t *testing.T) {
	run := NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(),
		kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "release-1")
	require.Equal(t, "release-1", run.ImageTag)
}

func TestDeploymentRunTransitionRejectsSkippedAndTerminalTransitions(t *testing.T) {
	run := NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(),
		kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "release-1")
	require.ErrorIs(t, run.Transition(StatusSucceeded, time.Now()), domain.ErrInvalidDeploymentTransition)
	require.NoError(t, run.Transition(StatusPreparing, time.Now()))
	require.NoError(t, run.Transition(StatusFailed, time.Now()))
	require.ErrorIs(t, run.Transition(StatusBuilding, time.Now()), domain.ErrInvalidDeploymentTransition)
}

func TestDeploymentRunHappyPathSetsStagesAndTimestamps(t *testing.T) {
	now := time.Date(2026, 8, 29, 1, 2, 3, 0, time.UTC)
	run := NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(),
		kernel.NewClusterInfoID(), kernel.NewMemberID(), "tag", "v1.2.3", "v1.2.3")
	require.Equal(t, StatusQueued, run.Status)
	require.NoError(t, run.Transition(StatusPreparing, now))
	require.Equal(t, StagePreparing, run.CurrentStage)
	require.NotNil(t, run.StartedAt)
	require.NoError(t, run.Transition(StatusBuilding, now.Add(time.Second)))
	require.NoError(t, run.Transition(StatusDeploying, now.Add(2*time.Second)))
	require.NoError(t, run.Transition(StatusSucceeded, now.Add(3*time.Second)))
	require.NotNil(t, run.FinishedAt)
}
```

- [ ] **Step 2: 运行领域测试，确认红灯**

Run: `cd backend && go test ./internal/modules/delivery/deployrun/domain/model -run 'TestDefaultImageTag|TestNewDeploymentRun|TestDeploymentRun' -v`  
Expected: FAIL，报 `package .../deployrun/domain/model is not in std` 或 `undefined: NewDeploymentRun`。

- [ ] **Step 3: 增加 ID、默认 tag、状态机和仓储契约的最小实现**

```go
type DeploymentRunID struct{ uuid.UUID }
func NewDeploymentRunID() DeploymentRunID { return DeploymentRunID{UUID: uuid.New()} }
func ParseDeploymentRunID(s string) (DeploymentRunID, error) {
	id, err := uuid.Parse(s); return DeploymentRunID{UUID: id}, err
}
func (id DeploymentRunID) String() string { return id.UUID.String() }
```

```go
func DefaultImageTag(id kernel.DeploymentRunID) string {
	s := strings.ReplaceAll(id.String(), "-", "")
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func NewDeploymentRun(
	wsID kernel.WorkspaceID, projectID kernel.ProjectID, clusterID kernel.ClusterInfoID,
	createdBy kernel.MemberID, gitRefType, gitRefValue, imageTag string,
) *DeploymentRun {
	id := kernel.NewDeploymentRunID()
	tag := strings.TrimSpace(imageTag)
	if tag == "" {
		tag = DefaultImageTag(id)
	}
	return &DeploymentRun{
		ID: id, WorkspaceID: wsID, ProjectID: projectID, ClusterID: clusterID,
		CreatedBy: createdBy, GitRefType: gitRefType, GitRefValue: gitRefValue,
		ImageTag: tag, Status: StatusQueued, CreatedAt: time.Now().UTC(),
	}
}
```

```go
type Status string
const (
	StatusQueued Status = "queued"; StatusPreparing Status = "preparing"
	StatusBuilding Status = "building"; StatusDeploying Status = "deploying"
	StatusSucceeded Status = "succeeded"; StatusFailed Status = "failed"; StatusAborted Status = "aborted"
)
var next = map[Status]map[Status]bool{
	StatusQueued: {StatusPreparing:true, StatusFailed:true, StatusAborted:true},
	StatusPreparing: {StatusBuilding:true, StatusFailed:true, StatusAborted:true},
	StatusBuilding: {StatusDeploying:true, StatusFailed:true, StatusAborted:true},
	StatusDeploying: {StatusSucceeded:true, StatusFailed:true, StatusAborted:true},
}
func (r *DeploymentRun) Transition(to Status, at time.Time) error {
	if !next[r.Status][to] { return domain.ErrInvalidDeploymentTransition }
	r.Status = to
	if r.StartedAt == nil { r.StartedAt = &at }
	switch to {
	case StatusPreparing: r.CurrentStage = StagePreparing
	case StatusBuilding: r.CurrentStage = StageBuilding
	case StatusDeploying: r.CurrentStage = StageDeploying
	}
	if to==StatusSucceeded || to==StatusFailed || to==StatusAborted { r.FinishedAt=&at }
	return nil
}
func (r *DeploymentRun) Fail(msg string, at time.Time) {
	_ = r.Transition(StatusFailed, at)
	r.ErrorMessage = msg
}
```

```go
var (
	ErrInvalidDeploymentTransition = kernel.NewDomainError("DELIVERY_DEPLOYMENT_INVALID_TRANSITION", "invalid deployment run transition")
	ErrDeploymentRunNotFound = kernel.NewDomainError("DELIVERY_DEPLOYMENT_NOT_FOUND", "deployment run not found")
	ErrDeploymentAlreadyRunning = kernel.NewDomainError("DELIVERY_DEPLOYMENT_RUNNING", "a deployment is already running for this project")
	ErrProjectNotApproved = kernel.NewDomainError("DELIVERY_PROJECT_NOT_APPROVED", "project is not approved")
	ErrDeploymentBindingMissing = kernel.NewDomainError("DELIVERY_DEPLOYMENT_BINDING_MISSING", "required deployment binding is missing")
	ErrDeploymentClusterNotFound = kernel.NewDomainError("DELIVERY_DEPLOYMENT_CLUSTER_NOT_FOUND", "cluster not found")
	ErrDeploymentClusterNotAuthorized = kernel.NewDomainError("DELIVERY_DEPLOYMENT_CLUSTER_NOT_AUTHORIZED", "cluster is not authorized for this workspace")
	ErrKubernetesKubeconfigMissing = kernel.NewDomainError("DELIVERY_KUBECONFIG_MISSING", "kubernetes secret extra_config kubeconfig is required")
)
```

- [ ] **Step 4: 建立可逆迁移（含 workspace 集群授权表）**

```sql
ALTER TABLE workspace_app.projects
  ADD COLUMN IF NOT EXISTS default_cluster_id UUID,
  ADD COLUMN IF NOT EXISTS jenkins_secret_id UUID,
  ADD COLUMN IF NOT EXISTS kubernetes_secret_id UUID;

CREATE TABLE IF NOT EXISTS platform_admin.workspace_clusters (
  workspace_id UUID NOT NULL,
  cluster_id   UUID NOT NULL,
  created_by   UUID NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (workspace_id, cluster_id)
);
CREATE INDEX IF NOT EXISTS idx_workspace_clusters_cluster
  ON platform_admin.workspace_clusters(cluster_id);

CREATE TABLE IF NOT EXISTS delivery.deployment_runs (
  id UUID PRIMARY KEY, workspace_id UUID NOT NULL, project_id UUID NOT NULL,
  cluster_id UUID NOT NULL, git_ref_type VARCHAR(16) NOT NULL,
  git_ref_value VARCHAR(255) NOT NULL, image_tag VARCHAR(255) NOT NULL,
  status VARCHAR(16) NOT NULL, build_id UUID, error_message TEXT NOT NULL DEFAULT '',
  created_by UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL,
  started_at TIMESTAMPTZ, finished_at TIMESTAMPTZ, current_stage VARCHAR(16) NOT NULL DEFAULT ''
);
ALTER TABLE delivery.deployment_runs ADD CONSTRAINT chk_deployment_run_status
  CHECK (status IN ('queued','preparing','building','deploying','succeeded','failed','aborted'));
CREATE UNIQUE INDEX IF NOT EXISTS uq_deployment_runs_active_project
  ON delivery.deployment_runs(project_id)
  WHERE status IN ('queued','preparing','building','deploying');
CREATE TABLE IF NOT EXISTS delivery.deployment_run_events (
  id BIGSERIAL PRIMARY KEY,
  run_id UUID NOT NULL REFERENCES delivery.deployment_runs(id) ON DELETE CASCADE,
  at TIMESTAMPTZ NOT NULL, stage VARCHAR(16) NOT NULL, message TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_deployment_run_events_run_at
  ON delivery.deployment_run_events(run_id, at);
```

Down migration 顺序：`DROP TABLE delivery.deployment_run_events`；`DROP TABLE delivery.deployment_runs`；`DROP TABLE platform_admin.workspace_clusters`；再 `ALTER TABLE workspace_app.projects DROP COLUMN` 三个绑定列。不对 `workspace_app.projects` 使用 `CASCADE`。

- [ ] **Step 5: 验证状态机与默认 tag**

Run: `cd backend && go test ./internal/modules/delivery/deployrun/domain/model -v`  
Expected: PASS，`TestDefaultImageTag*`、`TestNewDeploymentRun*`、`TestDeploymentRun*` 均通过。  
Run: `cd backend && go test ./internal/shared/kernel -run DeploymentRun -v`  
Expected: PASS（为 `ParseDeploymentRunID`/`String` round-trip 增加测试后通过）。

### Task 2: Workspace 集群授权、项目绑定与创建门禁

**Files:**
- Modify: `backend/internal/modules/platform_admin/domain/repository/admin_repository.go`
- Modify: `backend/internal/modules/platform_admin/infrastructure/persistence/admin_repo.go`
- Modify: `backend/internal/modules/platform_admin/application/command/admin_commands.go`
- Modify: `backend/internal/modules/platform_admin/application/query/admin_queries.go`
- Create: `backend/internal/modules/platform_admin/application/command/workspace_cluster_commands_test.go`
- Modify: `backend/internal/modules/platform_admin/interface/http/handler.go`
- Modify: `backend/internal/modules/platform_admin/module.go`
- Modify: `backend/internal/modules/workspace_app/domain/model/project.go`
- Modify: `backend/internal/modules/workspace_app/infrastructure/persistence/project_repo.go`
- Modify: `backend/internal/modules/workspace_app/application/command/create_project.go`
- Modify: `backend/internal/modules/workspace_app/application/command/update_project.go`
- Modify: `backend/internal/modules/workspace_app/application/query/project_queries.go`
- Modify: `backend/internal/modules/workspace_app/interface/http/handler.go`
- Create: `backend/internal/modules/delivery/deployrun/domain/port/project_deployment_reader.go`
- Create: `backend/internal/modules/delivery/deployrun/infrastructure/acl/workspace_app_acl.go`
- Create: `backend/internal/modules/delivery/deployrun/infrastructure/acl/workspace_app_acl_test.go`

**Interfaces:**
- Consumes: `workspace_app.projects`、`workspace_app.workspace_secrets`、`platform_admin.cluster_info`、`platform_admin.workspace_clusters`，以及现有 `crypto.NewSecretBoxFromConfig` 解密后的 `model.SecretCredentials`。
- Produces:
  - `AdminRepository.GrantWorkspaceCluster(ctx, wsID, clusterID, createdBy) error`
  - `AdminRepository.RevokeWorkspaceCluster(ctx, wsID, clusterID) error`
  - `AdminRepository.ListWorkspaceClusters(ctx, wsID) ([]*ClusterInfo, error)`（仅 JOIN 授权行，不含未授权全局集群）
  - `AdminRepository.WorkspaceClusterGranted(ctx, wsID, clusterID) (bool, error)`
  - `command.GrantWorkspaceClusterHandler.Handle(ctx, memberID, wsID, clusterID) error`
  - `command.RevokeWorkspaceClusterHandler.Handle(ctx, wsID, clusterID) error`
  - `query.ListWorkspaceClustersHandler.Handle(ctx, wsID) ([]map[string]any, error)`，每项含 `id`、`cluster_name`、`server`、`is_build_env`、`priority_id`
  - HTTP：`GET/POST /api/v1/admin/workspaces/:wsId/clusters`、`DELETE /api/v1/admin/workspaces/:wsId/clusters/:clusterId`（`authz.RequireSuperAdmin`）
  - `port.ProjectDeploymentReader.Load(ctx, workspaceID, projectID, clusterID) (port.ProjectDeploymentContext, error)`
  - `port.ProjectDeploymentReader.ListAuthorizedClusters(ctx, workspaceID) ([]port.DeploymentCluster, error)`
  - ACL 私有方法：`loadProject`、`loadSecret`、`loadAuthorizedCluster`、`loadKubernetesSecret`

- [ ] **Step 1: 写授权与门禁测试**

```go
func TestGrantWorkspaceClusterRejectsUnknownCluster(t *testing.T) {
	repo := &grantRepo{clusterExists: false}
	h := command.NewGrantWorkspaceClusterHandler(repo)
	err := h.Handle(context.Background(), kernel.NewMemberID(), kernel.NewWorkspaceID(), kernel.NewClusterInfoID())
	require.ErrorIs(t, err, platformadmindomain.ErrClusterNotFound)
}

func TestLoadRejectsUnauthorizedCluster(t *testing.T) {
	db := fixtureDB(t)
	seedApprovedProjectAndSecrets(t, db)
	clearWorkspaceGrant(t, db)
	_, err := NewWorkspaceAppACL(db, testSecretBox(t)).Load(
		context.Background(), testWorkspaceID, testProjectID, testClusterID)
	require.ErrorIs(t, err, deploydomain.ErrDeploymentClusterNotAuthorized)
}

func TestLoadRejectsKubernetesSecretWithoutExtraConfig(t *testing.T) {
	db := fixtureDB(t)
	seedApprovedProjectAndSecrets(t, db)
	setKubernetesCredentials(t, db, model.SecretCredentials{Token: "only-token"})
	_, err := NewWorkspaceAppACL(db, testSecretBox(t)).Load(
		context.Background(), testWorkspaceID, testProjectID, testClusterID)
	require.ErrorIs(t, err, deploydomain.ErrKubernetesKubeconfigMissing)
}

func TestLoadRejectsIncompleteBindings(t *testing.T) {
	cases := []struct{name, column string; want error}{
		{"unapproved", "review_status", deploydomain.ErrProjectNotApproved},
		{"missing harbor", "registry_secret_id", deploydomain.ErrDeploymentBindingMissing},
		{"missing jenkins", "jenkins_secret_id", deploydomain.ErrDeploymentBindingMissing},
		{"missing kubernetes", "kubernetes_secret_id", deploydomain.ErrDeploymentBindingMissing},
	}
	for _, tc := range cases { t.Run(tc.name, func(t *testing.T) {
		db := fixtureDB(t); seedApprovedProjectAndSecrets(t, db)
		clearBinding(t, db, tc.column)
		_, err := NewWorkspaceAppACL(db, testSecretBox(t)).Load(
			context.Background(), testWorkspaceID, testProjectID, kernel.ClusterInfoID{})
		require.ErrorIs(t, err, tc.want)
	})}
}

func TestLoadRequiresSonarOnlyWhenStaticScanEnabled(t *testing.T) {
	db := fixtureDB(t); seedApprovedProjectAndSecrets(t, db)
	clearBinding(t, db, "sonarqube_secret_id")
	_, err := NewWorkspaceAppACL(db, testSecretBox(t)).Load(context.Background(), testWorkspaceID, testProjectID, kernel.ClusterInfoID{})
	require.NoError(t, err)
	setStaticScan(t, db, true)
	_, err = NewWorkspaceAppACL(db, testSecretBox(t)).Load(context.Background(), testWorkspaceID, testProjectID, kernel.ClusterInfoID{})
	require.ErrorIs(t, err, deploydomain.ErrDeploymentBindingMissing)
}

func TestListAuthorizedClustersOmitsUngranted(t *testing.T) {
	db := fixtureDB(t)
	seedTwoClustersGrantOne(t, db)
	items, err := NewWorkspaceAppACL(db, testSecretBox(t)).ListAuthorizedClusters(context.Background(), testWorkspaceID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, grantedClusterID.String(), items[0].ID.String())
}
```

`seedApprovedProjectAndSecrets` 必须写入 `workspace_clusters` 授权行，且 Kubernetes 密钥 `extra_config` 为完整 kubeconfig YAML。

- [ ] **Step 2: 运行测试，确认红灯**

Run: `cd backend && go test ./internal/modules/platform_admin/application/command -run TestGrantWorkspaceCluster -v`  
Expected: FAIL，`NewGrantWorkspaceClusterHandler` 未定义。  
Run: `cd backend && go test ./internal/modules/delivery/deployrun/infrastructure/acl -run 'TestLoad|TestListAuthorized' -v`  
Expected: FAIL，`NewWorkspaceAppACL`/`ProjectDeploymentContext` 尚未定义。

- [ ] **Step 3: 扩充 AdminRepository、GORM 模型与 Super Admin HTTP**

```go
type workspaceClusterModel struct {
	WorkspaceID string `gorm:"column:workspace_id;primaryKey"`
	ClusterID   string `gorm:"column:cluster_id;primaryKey"`
	CreatedBy   string
	CreatedAt   time.Time
}
func (workspaceClusterModel) TableName() string { return "platform_admin.workspace_clusters" }
```

`AutoMigrate` 增加 `&workspaceClusterModel{}`。

```go
func (r *AdminRepository) GrantWorkspaceCluster(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID, createdBy kernel.MemberID) error {
	var n int64
	if err := r.db.WithContext(ctx).Model(&clusterInfoModel{}).Where("id = ?", clusterID.String()).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrClusterNotFound
	}
	row := workspaceClusterModel{
		WorkspaceID: wsID.String(), ClusterID: clusterID.String(),
		CreatedBy: createdBy.String(), CreatedAt: time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

func (r *AdminRepository) ListWorkspaceClusters(ctx context.Context, wsID kernel.WorkspaceID) ([]*repository.ClusterInfo, error) {
	var rows []clusterInfoModel
	err := r.db.WithContext(ctx).
		Table("platform_admin.cluster_info AS c").
		Joins("INNER JOIN platform_admin.workspace_clusters g ON g.cluster_id = c.id").
		Where("g.workspace_id = ?", wsID.String()).
		Order("c.priority_id ASC").
		Find(&rows).Error
	// map via existing rowToCluster
}
```

```go
func (h *Handler) ListWorkspaceClusters(c *gin.Context) {
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	items, err := h.listWorkspaceClusters.Handle(c.Request.Context(), wsID)
	if err != nil { httpx.HandleError(c, err); return }
	httpx.OK(c, gin.H{"items": items})
}
func (h *Handler) GrantWorkspaceCluster(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok { httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized); return }
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil { httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest); return }
	var req struct{ ClusterID string `json:"cluster_id" binding:"required"` }
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest); return
	}
	clusterID, err := kernel.ParseClusterInfoID(req.ClusterID)
	if err != nil { httpx.Fail(c, "INVALID_REQUEST", "invalid cluster id", http.StatusBadRequest); return }
	if err := h.grantWorkspaceCluster.Handle(c.Request.Context(), memberID, wsID, clusterID); err != nil {
		httpx.HandleError(c, err); return
	}
	httpx.OK(c, gin.H{"granted": true, "workspace_id": wsID.String(), "cluster_id": clusterID.String()})
}
```

`module.go` 注册：

```go
c.Engine.GET("/api/v1/admin/workspaces/:wsId/clusters", auth, superAdmin, handler.ListWorkspaceClusters)
c.Engine.POST("/api/v1/admin/workspaces/:wsId/clusters", auth, superAdmin, handler.GrantWorkspaceCluster)
c.Engine.DELETE("/api/v1/admin/workspaces/:wsId/clusters/:clusterId", auth, superAdmin, handler.RevokeWorkspaceCluster)
```

- [ ] **Step 4: 扩充 Project 字段，并在更新默认集群时校验授权**

```go
type Project struct {
	DefaultClusterID   string
	JenkinsSecretID    string
	KubernetesSecretID string
}
type UpdateProjectInput struct {
	DefaultClusterID *string
	JenkinsSecretID  *string
	KubernetesSecretID *string
}
```

将三个字段加入 `projectModel`、`Save`、`Update`、`rowToProject`、`projectView`；它们属于配置字段，值变化时 `patchChangesConfiguration` 返回 true。`UpdateProjectHandler` 在 `DefaultClusterID` 非空时调用 `WorkspaceClusterGranted`（通过注入的 `port` 或直接查询 `platform_admin.workspace_clusters`）；未授权返回 `deploydomain.ErrDeploymentClusterNotAuthorized`（workspace_app 侧可再包一层 `WORKSPACE_APP_CLUSTER_NOT_AUTHORIZED`，HTTP 映射 400）。

- [ ] **Step 5: 实现 ACL Load / ListAuthorizedClusters / kubeconfig**

```go
type DeploymentCluster struct {
	ID     kernel.ClusterInfoID
	Name   string
	Server string
}
type DeploymentSecret struct {
	ID           kernel.SecretID
	ServiceType  string
	ServiceURL   string
	Username     string
	Credentials  model.SecretCredentials
}
type ProjectDeploymentContext struct {
	WorkspaceID      kernel.WorkspaceID
	ProjectID        kernel.ProjectID
	Name             string
	Namespace        string
	Jenkinsfile      string
	ServiceYAML      string
	DeploymentYAML   string
	ImageRepository  string
	EnableStaticScan bool
	Cluster          DeploymentCluster
	Git              DeploymentSecret
	Harbor           DeploymentSecret
	Jenkins          DeploymentSecret
	Kubernetes       DeploymentSecret
	Sonar            *DeploymentSecret
}

type ProjectDeploymentReader interface {
	Load(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, clusterID kernel.ClusterInfoID) (ProjectDeploymentContext, error)
	ListAuthorizedClusters(ctx context.Context, wsID kernel.WorkspaceID) ([]DeploymentCluster, error)
}
```

```go
func (a *WorkspaceAppACL) loadAuthorizedCluster(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (port.DeploymentCluster, error) {
	var row struct{ ID, ClusterName, Server string }
	err := a.db.WithContext(ctx).Raw(`
		SELECT c.id, c.cluster_name, c.server
		FROM platform_admin.cluster_info c
		INNER JOIN platform_admin.workspace_clusters g
		  ON g.cluster_id = c.id AND g.workspace_id = ?
		WHERE c.id = ?`, wsID.String(), clusterID.String()).Scan(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || row.ID == "" {
		return port.DeploymentCluster{}, deploydomain.ErrDeploymentClusterNotAuthorized
	}
	id, err := kernel.ParseClusterInfoID(row.ID)
	if err != nil { return port.DeploymentCluster{}, err }
	return port.DeploymentCluster{ID: id, Name: row.ClusterName, Server: row.Server}, nil
}

func (a *WorkspaceAppACL) loadKubernetesSecret(ctx context.Context, wsID kernel.WorkspaceID, secretID string) (port.DeploymentSecret, error) {
	sec, err := a.loadSecret(ctx, wsID, secretID, "Kubernetes")
	if err != nil { return port.DeploymentSecret{}, err }
	if strings.TrimSpace(sec.Credentials.ExtraConfig) == "" {
		return port.DeploymentSecret{}, deploydomain.ErrKubernetesKubeconfigMissing
	}
	return sec, nil
}

func (a *WorkspaceAppACL) Load(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, selected kernel.ClusterInfoID) (port.ProjectDeploymentContext, error) {
	project, err := a.loadProject(ctx, wsID, projectID)
	if err != nil { return port.ProjectDeploymentContext{}, err }
	if project.ReviewStatus != "approved" { return port.ProjectDeploymentContext{}, deploydomain.ErrProjectNotApproved }
	clusterID := selected
	if clusterID.UUID == uuid.Nil {
		clusterID, err = kernel.ParseClusterInfoID(project.DefaultClusterID)
		if err != nil { return port.ProjectDeploymentContext{}, deploydomain.ErrDeploymentBindingMissing }
	}
	cluster, err := a.loadAuthorizedCluster(ctx, wsID, clusterID)
	if err != nil { return port.ProjectDeploymentContext{}, err }
	git, err := a.loadSecret(ctx, wsID, project.SecretID, "GitLab", "GitHub")
	if err != nil { return port.ProjectDeploymentContext{}, err }
	harbor, err := a.loadSecret(ctx, wsID, project.RegistrySecretID, "Harbor")
	if err != nil { return port.ProjectDeploymentContext{}, err }
	jenkins, err := a.loadSecret(ctx, wsID, project.JenkinsSecretID, "Jenkins")
	if err != nil { return port.ProjectDeploymentContext{}, err }
	kubernetes, err := a.loadKubernetesSecret(ctx, wsID, project.KubernetesSecretID)
	if err != nil { return port.ProjectDeploymentContext{}, err }
	var sonar *port.DeploymentSecret
	if project.EnableStaticScan {
		value, loadErr := a.loadSecret(ctx, wsID, project.SonarqubeSecretID, "SonarQube")
		if loadErr != nil { return port.ProjectDeploymentContext{}, loadErr }
		sonar = &value
	}
	return newDeploymentContext(project, cluster, git, harbor, jenkins, kubernetes, sonar), nil
}
```

`loadSecret` 校验 `WorkspaceID`、`Status=active` 以及 `ServiceType` 属于传入白名单。

- [ ] **Step 6: 验证授权、绑定与门禁**

Run: `cd backend && go test ./internal/modules/platform_admin/... ./internal/modules/workspace_app/... ./internal/modules/delivery/deployrun/infrastructure/acl -v`  
Expected: PASS；未授权集群、缺 `extra_config`、未审核、缺密钥、Sonar 条件必填均覆盖；`ListAuthorizedClusters` 不含未授权集群。

### Task 3: Repository、manual ACK RabbitMQ、HTTP/SSE 与幂等消费骨架

**Files:**
- Create: `backend/internal/modules/delivery/deployrun/infrastructure/persistence/deployment_run_repo.go`
- Create: `backend/internal/modules/delivery/deployrun/infrastructure/persistence/deployment_run_repo_test.go`
- Create: `backend/internal/platform/messaging/consume.go`
- Create: `backend/internal/platform/messaging/consume_test.go`
- Modify: `backend/internal/platform/messaging/noop_queue.go`
- Create: `backend/internal/platform/messaging/noop_queue_test.go`
- Modify: `backend/internal/platform/messaging/amqp.go`
- Create: `backend/internal/platform/messaging/amqp_consume_test.go`
- Create: `backend/internal/modules/delivery/deployrun/application/command/create_deployment.go`
- Create: `backend/internal/modules/delivery/deployrun/application/command/create_deployment_test.go`
- Create: `backend/internal/modules/delivery/deployrun/application/query/deployment_queries.go`
- Create: `backend/internal/modules/delivery/deployrun/infrastructure/stream/hub.go`
- Create: `backend/internal/modules/delivery/deployrun/infrastructure/consumer/consumer.go`
- Create: `backend/internal/modules/delivery/deployrun/interface/http/handler.go`
- Modify: `backend/internal/modules/delivery/module.go`
- Modify: `backend/internal/platform/httpx/errors.go`

**Interfaces:**
- Consumes: Task 1 repository、Task 2 `ProjectDeploymentReader`、`messaging.Queue`、`httpx.StreamSSE`、现有 auth/workspace middleware。
- Produces:
  - `const TopicDeploy = "deploy.sparkops"`
  - `const HeaderRetry = "x-sparkops-retry"`
  - `const MaxConsumeRetry = 3`
  - `DeadLetterTopic(topic string) string` 返回 `topic+".dlq"`
  - `type ConsumeDecision struct { Ack bool; RepublishTopic string; RepublishRetry int; DeadLetterTopic string }`
  - `DecideConsume(topic string, retry int, handlerErr error) ConsumeDecision`
  - `command.CreateDeploymentHandler.Handle(ctx, wsID, memberID, projectID, CreateInput) (*model.DeploymentRun, error)`，`CreateInput{ClusterID, GitRefType, GitRefValue, ImageTag string}`
  - HTTP：`POST/GET /api/v1/projects/:projectId/deployments`、`GET /api/v1/projects/:projectId/deployments/options`、`GET /api/v1/deployments/:runId`、`GET /api/v1/deployments/:runId/stream`、`POST /api/v1/deployments/:runId/abort`
  - `consumer.Message{RunID, WorkspaceID string}`
  - `Queue` 仍为 `Publish(ctx, topic, payload []byte) error`；AMQP `Publish` 增加可选内部重载 `publishWithHeaders`，不改公共接口

- [ ] **Step 1: 写 topic、retry/DLQ、创建冲突与默认 tag 测试**

```go
func TestValidateTopicAllowsDeployAndDLQ(t *testing.T) {
	require.NoError(t, ValidateTopic(TopicDeploy))
	require.NoError(t, ValidateTopic(DeadLetterTopic(TopicDeploy)))
	require.Equal(t, "deploy.sparkops.dlq", DeadLetterTopic(TopicDeploy))
}

func TestDecideConsumeAcksNilHandler(t *testing.T) {
	d := DecideConsume(TopicDeploy, 0, nil)
	require.True(t, d.Ack)
	require.Empty(t, d.RepublishTopic)
	require.Empty(t, d.DeadLetterTopic)
}

func TestDecideConsumeRepublishesUntilMaxRetry(t *testing.T) {
	infra := errors.New("broker timeout")
	d := DecideConsume(TopicDeploy, 0, infra)
	require.True(t, d.Ack)
	require.Equal(t, TopicDeploy, d.RepublishTopic)
	require.Equal(t, 1, d.RepublishRetry)
	d = DecideConsume(TopicDeploy, 2, infra)
	require.Equal(t, 3, d.RepublishRetry)
	require.Empty(t, d.DeadLetterTopic)
}

func TestDecideConsumePublishesDLQAfterMaxRetry(t *testing.T) {
	d := DecideConsume(TopicDeploy, 3, errors.New("still failing"))
	require.True(t, d.Ack)
	require.Empty(t, d.RepublishTopic)
	require.Equal(t, "deploy.sparkops.dlq", d.DeadLetterTopic)
}

func TestCreateDeploymentRejectsActiveRunWithoutPublishing(t *testing.T) {
	repo := &fakeRepo{active:true}; q := &recordingQueue{}
	h := command.NewCreateDeploymentHandler(repo, approvedReader(), q, fixedClock)
	_, err := h.Handle(context.Background(), wsID, memberID, projectID,
		command.CreateInput{GitRefType:"branch", GitRefValue:"main"})
	require.ErrorIs(t, err, domain.ErrDeploymentAlreadyRunning)
	require.Empty(t, q.payloads)
}

func TestCreateDeploymentDefaultImageTagFromRunID(t *testing.T) {
	repo := &fakeRepo{}
	h := command.NewCreateDeploymentHandler(repo, approvedReader(), &recordingQueue{}, fixedClock)
	run, err := h.Handle(context.Background(), wsID, memberID, projectID,
		command.CreateInput{GitRefType:"branch", GitRefValue:"main"})
	require.NoError(t, err)
	require.Equal(t, model.DefaultImageTag(run.ID), run.ImageTag)
}

func TestConsumerAcknowledgesUnknownAndTerminalRuns(t *testing.T) {
	require.NoError(t, consumer.New(fakeMissingRepo(), nil).Handle(ctx, validMessage))
	require.NoError(t, consumer.New(fakeTerminalRepo(), panicStepper{}).Handle(ctx, validMessage))
}
```

- [ ] **Step 2: 运行测试，确认红灯**

Run: `cd backend && go test ./internal/platform/messaging ./internal/modules/delivery/deployrun/... -run 'TestValidateTopicAllowsDeploy|TestDecideConsume|TestCreateDeployment|TestConsumer' -v`  
Expected: FAIL，缺少 `TopicDeploy`、`DecideConsume`、handler、repo 和 consumer。

- [ ] **Step 3: 实现 ConsumeDecision 与仓储**

```go
const (
	TopicDeploy       = "deploy.sparkops"
	HeaderRetry       = "x-sparkops-retry"
	MaxConsumeRetry   = 3
)

func DeadLetterTopic(topic string) string { return topic + ".dlq" }

func ValidateTopic(topic string) error {
	base := strings.TrimSuffix(topic, ".dlq")
	switch base {
	case TopicBuild, TopicCronJob, TopicAlarm, TopicNotice, TopicProclaim, TopicHook, TopicDeploy:
		return nil
	default:
		return fmt.Errorf("unknown queue topic: %s", topic)
	}
}

type ConsumeDecision struct {
	Ack             bool
	RepublishTopic  string
	RepublishRetry  int
	DeadLetterTopic string
}

func DecideConsume(topic string, retry int, handlerErr error) ConsumeDecision {
	if handlerErr == nil {
		return ConsumeDecision{Ack: true}
	}
	next := retry + 1
	if next <= MaxConsumeRetry {
		return ConsumeDecision{Ack: true, RepublishTopic: topic, RepublishRetry: next}
	}
	return ConsumeDecision{Ack: true, DeadLetterTopic: DeadLetterTopic(topic)}
}
```

```go
type DeploymentRunRepository interface {
	Create(ctx context.Context, run *model.DeploymentRun, first model.DeploymentRunEvent) error
	Update(ctx context.Context, run *model.DeploymentRun, event model.DeploymentRunEvent) error
	FindByID(ctx context.Context, id kernel.DeploymentRunID) (*model.DeploymentRun, error)
	FindByBuildID(ctx context.Context, id kernel.BuildID) (*model.DeploymentRun, error)
	ListByProject(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, page, pageSize int) (kernel.Page[*model.DeploymentRun], error)
	ListEvents(ctx context.Context, id kernel.DeploymentRunID, after time.Time) ([]model.DeploymentRunEvent, error)
	HasActiveByProject(ctx context.Context, projectID kernel.ProjectID) (bool, error)
}
```

`Create` 在一个 GORM transaction 写 Run+首事件；Postgres unique violation `uq_deployment_runs_active_project` 映射为 `ErrDeploymentAlreadyRunning`。

- [ ] **Step 4: AMQP manual ACK 与 in-memory 共用 DecideConsume**

```go
func retryFromHeaders(headers amqp.Table) int {
	if headers == nil { return 0 }
	raw, ok := headers[HeaderRetry]
	if !ok { return 0 }
	switch v := raw.(type) {
	case int32: return int(v)
	case int64: return int(v)
	case int: return v
	default: return 0
	}
}

func (q *amqpQueue) Subscribe(ctx context.Context, topic string, h func(context.Context, []byte) error) error {
	if err := ValidateTopic(topic); err != nil { return err }
	if _, err := q.ch.QueueDeclare(topic, true, false, false, false, nil); err != nil {
		return fmt.Errorf("amqp queue declare: %w", err)
	}
	if err := q.ch.QueueBind(topic, topic, q.cfg.Exchange, false, nil); err != nil {
		return fmt.Errorf("amqp queue bind: %w", err)
	}
	deliveries, err := q.ch.Consume(topic, "", false, false, false, false, nil)
	if err != nil { return fmt.Errorf("amqp consume: %w", err) }
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-deliveries:
				if !ok { return }
				retry := retryFromHeaders(d.Headers)
				handlerErr := h(ctx, d.Body)
				dec := DecideConsume(topic, retry, handlerErr)
				if dec.RepublishTopic != "" {
					_ = q.publishWithHeaders(ctx, dec.RepublishTopic, d.Body, amqp.Table{HeaderRetry: int32(dec.RepublishRetry)})
				}
				if dec.DeadLetterTopic != "" {
					_ = q.publishWithHeaders(ctx, dec.DeadLetterTopic, d.Body, amqp.Table{HeaderRetry: int32(retry + 1)})
				}
				_ = d.Ack(false)
			}
		}
	}()
	return nil
}
```

`amqp_consume_test.go` **不连接真实 broker**：直接测 `retryFromHeaders` 与 `DecideConsume` 组合。禁止调用 `Nack`。

`noopQueue.Publish` 同步调用 handler；若返回错误，按 `DecideConsume` 再 Publish 到 republish/DLQ topic（若已 Subscribe）。用于本地 in-memory 有限重试，次数仍受 `MaxConsumeRetry` 限制。

- [ ] **Step 5: 实现创建、options 与路由**

```go
type CreateInput struct {
	ClusterID   string
	GitRefType  string
	GitRefValue string
	ImageTag    string
}

run := model.NewDeploymentRun(wsID, projectID, clusterID, memberID, in.GitRefType, in.GitRefValue, in.ImageTag)
payload, _ := json.Marshal(consumer.Message{RunID: run.ID.String(), WorkspaceID: wsID.String()})
if err := h.queue.Publish(ctx, messaging.TopicDeploy, payload); err != nil {
	run.Fail("publish deploy message: "+err.Error(), h.clock())
	_ = h.repo.Update(ctx, run, model.NewEvent(run, "queue publish failed", h.clock()))
	return nil, err
}
```

`GET .../deployments/options` 调用 `ListAuthorizedClusters` 与门禁缺口（缺密钥、未审核、无授权集群、Kubernetes 无 `extra_config`），**不得**查询未 JOIN `workspace_clusters` 的全局 `cluster_info`。

创建返回 `c.JSON(http.StatusCreated, httpx.Envelope{Code:0, Message:"ok", Data:view})`。`httpx.HandleError` 增加 `DELIVERY_DEPLOYMENT_RUNNING` → 409、`DELIVERY_DEPLOYMENT_NOT_FOUND` → 404、`DELIVERY_DEPLOYMENT_CLUSTER_NOT_AUTHORIZED` / `DELIVERY_KUBECONFIG_MISSING` / `DELIVERY_DEPLOYMENT_BINDING_MISSING` / `DELIVERY_PROJECT_NOT_APPROVED` → 400。

```go
c.Engine.POST("/api/v1/projects/:projectId/deployments", auth, ws, wsMember,
	authz.RequirePermission(c.PermissionChecker, authz.PermProjectOperate), deployHTTP.Create)
c.Engine.GET("/api/v1/projects/:projectId/deployments", auth, ws, wsMember,
	authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), deployHTTP.List)
c.Engine.GET("/api/v1/projects/:projectId/deployments/options", auth, ws, wsMember,
	authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), deployHTTP.Options)
c.Engine.GET("/api/v1/deployments/:runId", auth, ws, wsMember,
	authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), deployHTTP.Get)
c.Engine.GET("/api/v1/deployments/:runId/stream", auth, ws, wsMember,
	authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), deployHTTP.Stream)
c.Engine.POST("/api/v1/deployments/:runId/abort", auth, ws, wsMember,
	authz.RequirePermission(c.PermissionChecker, authz.PermProjectOperate), deployHTTP.Abort)
```

- [ ] **Step 6: 验证 repo、HTTP 409、默认 tag 与 ACK 语义**

Run: `cd backend && go test ./internal/platform/messaging ./internal/modules/delivery/deployrun/... -v`  
Expected: PASS；并发创建只有一个成功，另一个得到 `DELIVERY_DEPLOYMENT_RUNNING`（HTTP 409）；未传 `image_tag` 时 tag 等于 `DefaultImageTag(run.ID)`；未知/终态消息不调用 stepper；`DecideConsume` 在 retry=3 且仍失败时只指向 DLQ 且 `Ack=true`。

### Task 4: Jenkins Job upsert、凭据和构建参数

**Files:**
- Modify: `backend/internal/modules/delivery/domain/port/ports.go`
- Modify: `backend/internal/modules/delivery/application/command/trigger_build.go`
- Modify: `backend/internal/modules/delivery/infrastructure/jenkins/executor.go`
- Modify: `backend/internal/modules/delivery/infrastructure/jenkins/mock_executor.go`
- Create: `backend/internal/modules/delivery/infrastructure/jenkins/executor_test.go`
- Create: `backend/internal/modules/delivery/deployrun/application/service/prepare.go`
- Create: `backend/internal/modules/delivery/deployrun/application/service/prepare_test.go`

**Interfaces:**
- Consumes: Task 2 `ProjectDeploymentContext`（Jenkinsfile、Git/Harbor/Sonar/Jenkins credentials）、现有 Build repository/EventBus/stream。
- Produces: `port.JenkinsJobSpec`; `port.JenkinsCredential`; `port.BuildRequest`; `JenkinsExecutor.CreateOrUpdate(ctx, spec) error`; `JenkinsExecutor.RunStreaming(ctx, buildID, req BuildRequest, hub LogPublisher) (string, error)`; `TriggerBuildHandler.TriggerDeployment(ctx, wsID, memberID, projectID, req port.BuildRequest) (*model.Build, error)`。

- [ ] **Step 1: 写 XML、稳定 credential ID 和参数测试**

```go
func TestJobXMLContainsPipelineAndParameters(t *testing.T) {
	xml := renderJobXML(port.JenkinsJobSpec{Folder:"team-a", Job:"orders",
		PipelineScript:"pipeline { stages {} }"})
	require.Contains(t, xml, "<definition class=\"org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition\"")
	require.Contains(t, xml, "<name>IMAGE_TAG</name>")
	require.Contains(t, xml, "<script>pipeline { stages {} }</script>")
}
func TestBuildParamsCarryImageCoordinates(t *testing.T) {
	p := buildParams(port.BuildRequest{GitRefValue:"main", ImageTag:"550e8400e29b",
		ImageRepository:"registry.example/team/orders"})
	require.Equal(t, "main", p["BRANCH"]); require.Equal(t, "main", p["CodeVersion"])
	require.Equal(t, "550e8400e29b", p["IMAGE_TAG"])
	require.Equal(t, "registry.example/team/orders", p["IMAGE_REPOSITORY"])
}
```

- [ ] **Step 2: 运行 Jenkins 单测，确认红灯**

Run: `cd backend && go test ./internal/modules/delivery/infrastructure/jenkins -run 'TestJobXML|TestBuildParams' -v`  
Expected: FAIL，`renderJobXML`、`JenkinsJobSpec`、`BuildRequest` 未定义。

- [ ] **Step 3: 把执行器改为显式 JobSpec/BuildRequest**

```go
type JenkinsCredential struct {
	ID, Username, Secret, Kind string
}
type JenkinsJobSpec struct {
	Folder, Job, PipelineScript string
	Credentials []JenkinsCredential
}
type BuildRequest struct {
	Folder, Job, GitRefValue, ImageTag, ImageRepository string
	RollbackOf int
}
type JenkinsExecutor interface {
	CreateOrUpdate(context.Context, JenkinsJobSpec) error
	RunStreaming(context.Context, kernel.BuildID, BuildRequest, LogPublisher) (string, error)
}
```

真实 driver 使用 Jenkins API 先保证 Folder 存在，再 `CreateJob` 或 `UpdateConfig`；凭据逐个按 `sparkops-{workspaceID}-{secretID}` upsert。mock driver 记录最后的 JobSpec/BuildRequest，并继续发布可预测日志。现有 `Handle`/`TriggerAutomated` 组装 `BuildRequest{Job: projectName+"-build", GitRefValue: branch}` 以保持普通 Build 兼容。

- [ ] **Step 4: preparing 渲染 Jenkinsfile 并 upsert**

```go
func (s *PrepareService) Run(ctx context.Context, project port.ProjectDeploymentContext) error {
	spec := port.JenkinsJobSpec{
		Folder: strings.TrimSpace(project.Namespace), Job: project.Name,
		PipelineScript: project.Jenkinsfile,
		Credentials: stableCredentials(project.WorkspaceID, project),
	}
	if err := s.jenkins.CreateOrUpdate(ctx, spec); err != nil {
		return fmt.Errorf("upsert jenkins job: %w", err)
	}
	return nil
}
```

部署路径不使用 `jobSuffix`。

- [ ] **Step 5: 验证 mock/real adapter 契约**

Run: `cd backend && go test ./internal/modules/delivery/application/command ./internal/modules/delivery/infrastructure/jenkins ./internal/modules/delivery/deployrun/application/service -v`  
Expected: PASS；普通 Build 测试不回归，部署准备测试断言 Folder/Job、credential ID 和四个参数。

### Task 5: Kubernetes Apply、Rollout Ready 与 Service 校验

**Files:**
- Modify: `backend/internal/modules/runtime/domain/port/ports.go`
- Modify: `backend/internal/modules/runtime/domain/errors.go`
- Create: `backend/internal/modules/runtime/infrastructure/k8s/deployer.go`
- Create: `backend/internal/modules/runtime/infrastructure/k8s/deployer_test.go`
- Create: `backend/internal/modules/runtime/infrastructure/k8s/mock_deployer.go`
- Modify: `backend/internal/modules/runtime/infrastructure/k8s/adapters.go`
- Modify: `backend/internal/platform/k8s/factory.go`
- Modify: `backend/internal/modules/runtime/module.go`
- Modify: `backend/internal/platform/appctx/container.go`

**Interfaces:**
- Consumes: Task 2 Kubernetes `Credentials.ExtraConfig`（完整 kubeconfig bytes）、cluster server、rendered Service/Deployment YAML；client-go dynamic/fake。
- Produces: `runtimeport.ManifestDeployer.Deploy(ctx, DeploymentTarget, []Manifest) error`; `appctx.Container.ManifestDeployer`；delivery 不反向 import runtime。

- [ ] **Step 1: 写空 kubeconfig、server 不匹配、Apply 与超时测试**

```go
func TestBuildClientsRejectsEmptyKubeconfig(t *testing.T) {
	_, _, err := buildClients(nil, "https://cluster-a.example")
	require.ErrorIs(t, err, runtimeDomain.ErrKubeconfigMissing)
}

func TestBuildClientsRejectsServerMismatch(t *testing.T) {
	raw := kubeconfigForServer("https://cluster-a.example")
	_, _, err := buildClients([]byte(raw), "https://cluster-b.example")
	require.ErrorIs(t, err, runtimeDomain.ErrClusterServerMismatch)
}

func TestDeployAppliesAndWaitsForReadyDeploymentAndService(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	d := NewManifestDeployer(dyn, fake.NewSimpleClientset(readyDeployment(), expectedService()), 10*time.Millisecond)
	err := d.Deploy(context.Background(), target(), manifests())
	require.NoError(t, err)
}

func TestDeployReturnsDeadlineWhenRolloutNeverReady(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond); defer cancel()
	err := neverReadyDeployer(t).Deploy(ctx, target(), manifests())
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
```

- [ ] **Step 2: 运行 runtime 测试，确认红灯**

Run: `cd backend && go test ./internal/modules/runtime/infrastructure/k8s -run 'TestBuildClients|TestDeploy' -v`  
Expected: FAIL，deployer 和 `ErrClusterServerMismatch` 尚不存在。

- [ ] **Step 3: 仅从 extra_config kubeconfig 创建 client 并校验 host**

```go
func buildClients(kubeconfig []byte, expectedServer string) (kubernetes.Interface, dynamic.Interface, error) {
	if len(bytes.TrimSpace(kubeconfig)) == 0 {
		return nil, nil, runtimeDomain.ErrKubeconfigMissing
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil { return nil, nil, err }
	want, err := url.Parse(expectedServer); if err != nil { return nil,nil,err }
	got, err := url.Parse(cfg.Host); if err != nil { return nil,nil,err }
	if !strings.EqualFold(got.Host, want.Host) { return nil,nil,runtimeDomain.ErrClusterServerMismatch }
	typed, err := kubernetes.NewForConfig(cfg)
	if err != nil { return nil, nil, err }
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil { return nil, nil, err }
	return typed, dyn, nil
}
```

不读取 `Token`/`Certificate` 字段拼 REST config。

- [ ] **Step 4: 实现 server-side apply 和 Ready 判定**

```go
data, err := json.Marshal(obj.Object)
if err != nil { return err }
_, err = d.dynamic.Resource(mapping.Resource).Namespace(obj.GetNamespace()).Patch(
	ctx, obj.GetName(), types.ApplyPatchType, data,
	metav1.PatchOptions{FieldManager:"sparkops-deploy", Force:ptr.To(true)})
```

等待条件：Deployment `observedGeneration` 已追上 `generation`，`AvailableReplicas >= *Spec.Replicas`；随后 `CoreV1().Services(ns).Get(serviceName)` 必须成功。只响应 `ctx.Done()`，不删除/回滚资源。

- [ ] **Step 5: 注入跨模块端口与 mock**

```go
type Manifest struct { YAML string }
type ManifestDeployer interface {
	Deploy(context.Context, DeploymentTarget, []Manifest) error
}
type DeploymentTarget struct {
	ClusterID, Server, Kubeconfig, Namespace, DeploymentName, ServiceName string
	RolloutTimeout time.Duration
}
```

`runtime.Module.Register` 设置 `c.ManifestDeployer=adapters.Deployer`。mock 要求 `target.Kubeconfig != ""`，并检查 Deployment YAML 的 image 已含本次 tag，然后立即成功。

- [ ] **Step 6: 验证 runtime 单测**

Run: `cd backend && go test ./internal/modules/runtime/... ./internal/platform/k8s/... -v`  
Expected: PASS；覆盖空 kubeconfig、Apply 成功、Rollout 超时、Service 缺失和 server host 不匹配。

### Task 6: BuildCompleted 驱动的完整 DeploymentRun 编排

**Files:**
- Create: `backend/internal/modules/delivery/deployrun/application/service/orchestrator.go`
- Create: `backend/internal/modules/delivery/deployrun/application/service/orchestrator_test.go`
- Create: `backend/internal/modules/delivery/deployrun/application/service/renderer.go`
- Create: `backend/internal/modules/delivery/deployrun/application/subscriber/on_build_completed.go`
- Create: `backend/internal/modules/delivery/deployrun/application/subscriber/on_build_completed_test.go`
- Modify: `backend/internal/modules/delivery/deployrun/infrastructure/consumer/consumer.go`
- Modify: `backend/internal/modules/delivery/module.go`
- Modify: `backend/internal/platform/config/config.go`
- Modify: `backend/config.example.yaml`
- Modify: `backend/config.remote.example.yaml`
- Modify: `backend/deployments/docker/config.docker.yaml`

**Interfaces:**
- Consumes: Tasks 1–5 的 repo、`ProjectDeploymentReader`、`PrepareService.Run(ctx, ProjectDeploymentContext) error`、`TriggerDeployment`、`ManifestDeployer`，以及现有 `deliveryevent.BuildCompleted`。
- Produces: `Orchestrator.Step(ctx, runID kernel.DeploymentRunID, workspaceID kernel.WorkspaceID) error`; `ManifestRenderer.Render(ctx ProjectDeploymentContext, imageTag string) ([]runtimeport.Manifest, error)`; `OnBuildCompleted.Handle(ctx context.Context, e events.Event) error`；`publisher.PublishRun(ctx, run *model.DeploymentRun) error`。

- [ ] **Step 1: 写重复消息、CI 失败和成功推进测试**

```go
func TestBuildingMessageDoesNotInvokeJenkinsAgain(t *testing.T) {
	f := fixture(StatusBuilding); require.NoError(t, f.orchestrator.Step(ctx, f.run.ID, f.wsID))
	require.Equal(t, 0, f.jenkins.upserts); require.Equal(t, 0, f.trigger.calls)
}
func TestBuildCompletedFailureFailsRunWithoutApply(t *testing.T) {
	f := fixture(StatusBuilding); f.run.BuildID = &f.buildID
	err := f.subscriber.Handle(ctx, event.NewBuildCompleted(f.buildID, f.projectID, f.memberID, f.wsID, "p", "failed", 1))
	require.NoError(t, err); require.Equal(t, StatusFailed, f.repo.saved.Status)
	require.Equal(t, 0, f.deployer.calls)
}
func TestBuildCompletedSuccessPublishesDeployMessage(t *testing.T) {
	f := fixture(StatusBuilding); f.run.BuildID = &f.buildID
	require.NoError(t, f.subscriber.Handle(ctx, successEvent(f)))
	require.JSONEq(t, `{"run_id":"`+f.run.ID.String()+`","workspace_id":"`+f.wsID.String()+`"}`, string(f.queue.last))
}
```

- [ ] **Step 2: 运行编排测试，确认红灯**

Run: `cd backend && go test ./internal/modules/delivery/deployrun/application/... -run 'TestBuildingMessage|TestBuildCompleted' -v`  
Expected: FAIL，Orchestrator/subscriber 未定义。

- [ ] **Step 3: 每条消息只推进一个状态**

```go
switch run.Status {
case model.StatusQueued:
	_, err := o.projects.Load(ctx, wsID, run.ProjectID, run.ClusterID)
	if err != nil { return o.failBusiness(ctx, run, err) }
	if err := run.Transition(model.StatusPreparing, o.clock()); err != nil { return err }
	if err := o.repo.Update(ctx, run, model.NewEvent(run, "deployment preparing", o.clock())); err != nil { return err }
	return o.publisher.PublishRun(ctx, run)
case model.StatusPreparing:
	project, err := o.projects.Load(ctx, wsID, run.ProjectID, run.ClusterID)
	if err != nil { return o.failBusiness(ctx, run, err) }
	if err := o.prepare.Run(ctx, project); err != nil { return o.failBusiness(ctx, run, err) }
	build, err := o.trigger.TriggerDeployment(ctx, wsID, run.CreatedBy, run.ProjectID, port.BuildRequest{
		Folder: project.Namespace, Job: project.Name, GitRefValue: run.GitRefValue,
		ImageTag: run.ImageTag, ImageRepository: project.ImageRepository,
	})
	if err != nil { return o.failBusiness(ctx, run, err) }
	run.BuildID = &build.ID
	if err := run.Transition(model.StatusBuilding, o.clock()); err != nil { return err }
	return o.repo.Update(ctx, run, model.NewEvent(run, "build started", o.clock()))
case model.StatusBuilding:
	return nil
case model.StatusDeploying:
	project, err := o.projects.Load(ctx, wsID, run.ProjectID, run.ClusterID)
	if err != nil { return o.failBusiness(ctx, run, err) }
	manifests, err := o.renderer.Render(project, run.ImageTag)
	if err != nil { return o.failBusiness(ctx, run, err) }
	target := runtimeport.DeploymentTarget{
		ClusterID: project.Cluster.ID.String(), Server: project.Cluster.Server,
		Kubeconfig: project.Kubernetes.Credentials.ExtraConfig,
		Namespace: project.Namespace, DeploymentName: project.Name + "-deployment",
		ServiceName: project.Name + "-service", RolloutTimeout: o.rolloutTimeout,
	}
	if err := o.deployer.Deploy(ctx, target, manifests); err != nil {
		return o.failBusiness(ctx, run, err)
	}
	if err := run.Transition(model.StatusSucceeded, o.clock()); err != nil { return err }
	return o.repo.Update(ctx, run, model.NewEvent(run, "deployment succeeded", o.clock()))
default:
	return nil
}
```

`failBusiness` 调用 `run.Fail`、写事件、返回 **nil**（业务失败 ACK）。JSON 解析失败、`FindByID` 的非 NotFound DB 错误原样返回，由 `DecideConsume` 做最多 3 次 republish。

- [ ] **Step 4: BuildCompleted 关联 build_id 后再发布**

```go
func (s *OnBuildCompleted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(deliveryevent.BuildCompleted)
	if !ok { return nil }
	run, err := s.repo.FindByBuildID(ctx, ev.BuildID)
	if errors.Is(err, domain.ErrDeploymentRunNotFound) { return nil }
	if err != nil { return err }
	if run.Status != model.StatusBuilding { return nil }
	if ev.Status != string(buildmodel.BuildStatusSuccess) {
		run.Fail("build completed with status "+ev.Status, s.clock())
		return s.repo.Update(ctx, run, model.NewEvent(run, "build failed", s.clock()))
	}
	if err := run.Transition(model.StatusDeploying, s.clock()); err != nil { return err }
	if err := s.repo.Update(ctx, run, model.NewEvent(run, "build succeeded", s.clock())); err != nil { return err }
	return s.publisher.PublishRun(ctx, run)
}
```

- [ ] **Step 5: 配置超时并注册订阅**

```go
type DeployConfig struct {
	JenkinsTimeout time.Duration `yaml:"jenkins_timeout"`
	RolloutTimeout time.Duration `yaml:"rollout_timeout"`
}
if c.Deploy.JenkinsTimeout == 0 { c.Deploy.JenkinsTimeout = 30 * time.Minute }
if c.Deploy.RolloutTimeout == 0 { c.Deploy.RolloutTimeout = 10 * time.Minute }
c.EventBus.Subscribe(deliveryevent.BuildCompletedName, onBuildCompleted.Handle)
```

三个 YAML 均加入 `deploy.jenkins_timeout: 30m`、`deploy.rollout_timeout: 10m`。

- [ ] **Step 6: 验证完整后端闭环**

Run: `cd backend && go test ./internal/modules/delivery/... ./internal/modules/runtime/... ./internal/platform/messaging -v`  
Expected: PASS；mock 集成测试观察 `queued→preparing→building→deploying→succeeded`，重复消息不重复调用 Jenkins，CI 失败时 deployer 调用数为 0。

### Task 7: Contracts、BFF、SDK、授权 UI 与项目工作台

**Files:**
- Modify: `fronted/package.json`
- Modify: `fronted/pnpm-lock.yaml`
- Create: `fronted/vitest.config.ts`
- Modify: `fronted/packages/contracts/src/delivery/index.ts`
- Modify: `fronted/packages/contracts/src/workspace/index.ts`
- Modify: `fronted/packages/contracts/src/platform/index.ts`
- Modify: `fronted/packages/upstream/src/delivery.ts`
- Modify: `fronted/packages/upstream/src/platform.ts`
- Modify: `fronted/packages/bff-modules/src/delivery/index.ts`
- Modify: `fronted/packages/bff-modules/src/platform/index.ts`
- Modify: `fronted/packages/web-sdk/src/delivery.ts`
- Modify: `fronted/packages/web-sdk/src/platform.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/projects/[projectId]/deployments/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/projects/[projectId]/deployments/options/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/deployments/[runId]/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/deployments/[runId]/abort/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/deployments/[runId]/stream/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/admin/workspaces/[wsId]/clusters/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/admin/workspaces/[wsId]/clusters/[clusterId]/route.ts`
- Modify: `fronted/apps/web/src/modules/platform_admin/components/ClusterListPanel.tsx`
- Create: `fronted/apps/web/src/modules/workspace/components/project/ProjectDeployModal.tsx`
- Create: `fronted/apps/web/src/modules/workspace/components/project/deployment-gate.ts`
- Create: `fronted/apps/web/src/modules/workspace/components/project/deployment-gate.test.ts`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectWorkbenchPage.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectDetailModal.tsx`

**Interfaces:**
- Consumes: Task 2 授权 HTTP、Task 3 Run HTTP/SSE；现有 `BffContext`、`upstreamFetch`、`bffFetch`、`Modal`、`ProjectVM`。
- Produces:
  - `DeploymentRunVM`、`CreateDeploymentCommand`、`DeploymentOptionsVM`、`WorkspaceClusterGrantCommand`
  - `delivery.listDeployments/createDeployment/getDeployment/abortDeployment/streamDeployment/getDeploymentOptions`
  - `platform.listWorkspaceClusters/grantWorkspaceCluster/revokeWorkspaceCluster`
  - `deploymentGate(project, secrets, options, activeRun)`

- [ ] **Step 1: 写 Zod、门禁与默认 tag 测试**

```ts
it("parses deployment run", () => {
  expect(DeploymentRunVMSchema.parse({
    id:"550e8400-e29b-41d4-a716-446655440000",
    workspace_id:"w", project_id:"p", cluster_id:"c",
    git_ref_type:"branch", git_ref_value:"main", image_tag:"550e8400e29b",
    status:"queued", current_stage:"", created_by:"m", created_at:new Date().toISOString()
  }).image_tag).toBe("550e8400e29b");
});
it("requires sonar only when static scan is enabled", () => {
  const project = approvedProject({ enable_static_scan:false, sonarqube_secret_id:undefined });
  const options = { clusters: [{id:"c", cluster_name:"dev", server:"https://k8s"}], missing: [] as string[] };
  expect(deploymentGate(project, boundSecrets(), options, null).allowed).toBe(true);
  expect(deploymentGate({...project, resources:{...project.resources, enable_static_scan:true}}, boundSecrets(), options, null).missing)
    .toContain("Sonarqube 密钥");
});
it("disables deploy when options has no authorized clusters", () => {
  const gate = deploymentGate(approvedProject({}), boundSecrets(), { clusters: [], missing: ["未授权集群"] }, null);
  expect(gate.allowed).toBe(false);
  expect(gate.missing).toContain("未授权集群");
});
```

- [ ] **Step 2: 安装并运行 Vitest，确认红灯**

Run: `cd fronted && pnpm add -D vitest@latest && pnpm vitest run apps/web/src/modules/workspace/components/project/deployment-gate.test.ts`  
Expected: FAIL，缺少 `DeploymentRunVMSchema` 和 `deploymentGate`。锁文件变更属于本任务，不运行 git 命令。

- [ ] **Step 3: 定义精确 contracts**

```ts
export const DeploymentStatusSchema = z.enum(["queued","preparing","building","deploying","succeeded","failed","aborted"]);
export const DeploymentRunVMSchema = z.object({
  id:z.string(), workspace_id:z.string(), project_id:z.string(), cluster_id:z.string(),
  git_ref_type:z.enum(["branch","tag"]), git_ref_value:z.string(), image_tag:z.string(),
  status:DeploymentStatusSchema, build_id:z.string().optional(), error_message:z.string().default(""),
  current_stage:z.enum(["","preparing","building","deploying"]), created_by:z.string(),
  created_at:z.string(), started_at:z.string().optional(), finished_at:z.string().optional(),
});
export const CreateDeploymentCommandSchema = z.object({
  cluster_id:z.string().optional(), git_ref_type:z.enum(["branch","tag"]),
  git_ref_value:z.string().min(1), image_tag:z.string().max(255).optional(),
});
export const AuthorizedClusterVMSchema = z.object({
  id:z.string(), cluster_name:z.string(), server:z.string().optional(),
});
export const DeploymentOptionsVMSchema = z.object({
  clusters:z.array(AuthorizedClusterVMSchema),
  missing:z.array(z.string()),
});
export const GrantWorkspaceClusterCommandSchema = z.object({
  cluster_id:z.string().uuid(),
});
```

`ProjectVMSchema` 与 create/update schema 加 `default_cluster_id`、`jenkins_secret_id`、`kubernetes_secret_id`。弹窗文案：镜像版本可选，占位说明「留空则使用本次部署 ID 去连字符后的前 12 位」，**不得**写 git SHA。

- [ ] **Step 4: 镜像 delivery 与 platform 三层**

```ts
async createDeployment(workspaceId:string, projectId:string, input:CreateDeploymentCommand) {
  return upstreamFetch<DeploymentRunVM>(`/api/v1/projects/${projectId}/deployments`, {
    method:"POST", body:JSON.stringify(input), options:{...options, workspaceId},
  });
}
async getDeploymentOptions(workspaceId:string, projectId:string) {
  return upstreamFetch<DeploymentOptionsVM>(`/api/v1/projects/${projectId}/deployments/options`, {
    method:"GET", options:{...options, workspaceId},
  });
}
async listWorkspaceClusters(workspaceId:string) {
  return upstreamFetch<{items: AuthorizedClusterVM[]}>(`/api/v1/admin/workspaces/${workspaceId}/clusters`, {
    method:"GET", options,
  });
}
async grantWorkspaceCluster(workspaceId:string, clusterId:string) {
  return upstreamFetch<{granted:true}>(`/api/v1/admin/workspaces/${workspaceId}/clusters`, {
    method:"POST", body:JSON.stringify({cluster_id: clusterId}), options,
  });
}
async revokeWorkspaceCluster(workspaceId:string, clusterId:string) {
  return upstreamFetch<{revoked:true}>(`/api/v1/admin/workspaces/${workspaceId}/clusters/${clusterId}`, {
    method:"DELETE", options,
  });
}
```

BFF `admin/workspaces/[wsId]/clusters/route.ts`：GET 调 `listWorkspaceClusters`，POST 用 `GrantWorkspaceClusterCommandSchema`。DELETE 在 `[clusterId]/route.ts`。SSE 复制现有 build stream 的 header/abort 转发；web-sdk `EventSource('/api/bff/v1/deployments/${runId}/stream')`。

- [ ] **Step 5: ClusterListPanel 授权 UI、部署弹窗与真实历史**

`ClusterListPanel` 增加「授权工作空间」：输入 `workspace_id`，调用 `platform.listWorkspaceClusters(wsId)` 展示该 workspace 已授权集群；对当前集群行提供「授权给该工作空间」按钮调用 `grantWorkspaceCluster(wsId, clusterId)`，以及从列表撤销 `revokeWorkspaceCluster`。不展示「全部集群可选」给部署弹窗。

```tsx
const options = await delivery.getDeploymentOptions(currentProject.id, workspaceId);
const gate = deploymentGate(project, secrets, options, activeRun);
<button type="button" disabled={!gate.allowed} onClick={() => setDeployOpen(true)}>部署</button>
{gate.missing.map((item) => <li key={item}>{item}</li>)}
<select value={clusterId}>
  {options.clusters.map((c) => <option key={c.id} value={c.id}>{c.cluster_name}</option>)}
</select>
```

`ProjectWorkbenchPage.onDeploy` 打开 `ProjectDeployModal`，不再打开详情；默认选中 `project.default_cluster_id`（且必须仍出现在 `options.clusters`）。提交时若 `image_tag` 为空则省略该字段，由后端 `DefaultImageTag` 填充。building 且有 `build_id` 时同时 `delivery.streamBuild`。`ProjectDetailModal` 调用 `delivery.listDeployments`，用 `items.length` 与首项状态/时间替换 `"0"` / `"N/A"`。

- [ ] **Step 6: 验证前端测试、类型与构建**

Run: `cd fronted && pnpm vitest run`  
Expected: PASS，contract/gate 测试通过。  
Run: `cd fronted && pnpm typecheck`  
Expected: PASS，所有 workspace 包零 TypeScript 错误。  
Run: `cd fronted && pnpm build`  
Expected: PASS，Next.js route 与 client component 构建成功。

### Task 8: 整体验证、回归矩阵与文档同步

**Files:**
- Create: `backend/internal/modules/delivery/deployrun/application/deployment_pipeline_integration_test.go`
- Modify: `backend/config.example.yaml`
- Modify: `READEMD.md`

**Interfaces:**
- Consumes: Tasks 1–7 的公开接口与 mock drivers。
- Produces: 可重复执行的第一期验收测试和本地演示命令；不产出真实外部 E2E。

- [ ] **Step 1: 写最终 mock 验收测试**

```go
func TestDeploymentPipelineAcceptance(t *testing.T) {
	env := newPipelineFixture(t, "mock", "mock", "inmemory")
	run := env.CreateApprovedDeployment(t, command.CreateInput{GitRefType: "branch", GitRefValue: "main"})
	require.Equal(t, model.DefaultImageTag(run.ID), run.ImageTag)
	env.Drain(t)
	got := env.Repository.Find(t, run.ID)
	require.Equal(t, model.StatusSucceeded, got.Status)
	require.Equal(t, []model.Status{
		model.StatusQueued, model.StatusPreparing, model.StatusBuilding,
		model.StatusDeploying, model.StatusSucceeded,
	}, env.EventStatuses(t, run.ID))
	require.Contains(t, env.Runtime.LastDeploymentYAML(), "registry.example/team/orders:"+run.ImageTag)
	require.NotEmpty(t, env.Runtime.LastKubeconfig())
}

func TestCIFailureDoesNotApply(t *testing.T) {
	env := newPipelineFixture(t, "mock-fail", "mock", "inmemory")
	run := env.CreateApprovedDeployment(t, command.CreateInput{GitRefType: "branch", GitRefValue: "main"})
	env.Drain(t)
	require.Equal(t, model.StatusFailed, env.Repository.Find(t, run.ID).Status)
	require.Equal(t, 0, env.Runtime.ApplyCalls())
}

func TestUnauthorizedClusterRejectedAtCreate(t *testing.T) {
	env := newPipelineFixture(t, "mock", "mock", "inmemory")
	_, err := env.CreateOnUnauthorizedCluster(t)
	require.ErrorIs(t, err, domain.ErrDeploymentClusterNotAuthorized)
}
```

`newPipelineFixture` 必须插入 `workspace_clusters` 授权行。

- [ ] **Step 2: 运行验收测试，确认在未完成接线时红灯**

Run: `cd backend && go test ./internal/modules/delivery/deployrun/application -run 'TestDeploymentPipelineAcceptance|TestCIFailureDoesNotApply|TestUnauthorizedClusterRejectedAtCreate' -v`  
Expected: 若任何注册/推进/授权缺失则 FAIL，并明确显示停留状态或 Apply 调用数；不得通过 sleep 掩盖竞态。

- [ ] **Step 3: 只做使验收通过的接线修正并同步 README**

```yaml
deploy:
  jenkins_timeout: 30m
  rollout_timeout: 10m
```

README 记录：先 Super Admin `POST /api/v1/admin/workspaces/:wsId/clusters`；Kubernetes 密钥只填 `extra_config` kubeconfig；未传 `image_tag` 时 tag 为 Run ID 去连字符前 12 位；mock 演示环境变量与 API 顺序；明确「本地 mock 验收不连接 Jenkins/Harbor/Kubernetes」。

- [ ] **Step 4: 执行后端全量验证**

Run: `cd backend && go test ./...`  
Expected: PASS。  
Run: `cd backend && go vet ./...`  
Expected: PASS，无新增诊断。

- [ ] **Step 5: 执行前端全量验证**

Run: `cd fronted && pnpm vitest run && pnpm typecheck && pnpm build`  
Expected: 三条命令均以 exit code 0 完成。

- [ ] **Step 6: 执行迁移与范围审计**

Run: `rg -n "workspace_clusters|TopicDeploy|deploy.sparkops|x-sparkops-retry|MaxConsumeRetry|DeadLetterTopic|DefaultImageTag|PermProjectOperate|extra_config" backend fronted`  
Expected: 授权表、topic、retry、默认 tag、权限、kubeconfig 字段均有命中。  
Run: `rg -n "auto.?rollback|d\.Nack|basic.nack|git SHA|DefaultImageTag.*git_ref" backend/internal/modules/delivery/deployrun backend/internal/platform/messaging backend/internal/modules/runtime/infrastructure/k8s/deployer.go`  
Expected: 无自动 rollback；无 `Nack`；无 git SHA 默认 tag。
