# Phase 2 严格聚合边界建模模板

> **用途**：作为 `workspace_app`（Project）与 `agent`（Agent + AgentVersion）BC 的实现模板。  
> **与 Phase 1 差异**：Phase 1 偏「实体 + 仓储 CRUD」；Phase 2 起按本文 **聚合根统一不变量、子实体不单独暴露、跨 BC 走 Port/ACL**。

---

## 一、核心原则（Phase 2+ 必须遵守）

| 原则 | 说明 | Phase 1 反例 → Phase 2 正例 |
|------|------|------------------------------|
| **一个聚合 = 一个一致性边界** | 一次事务只改一个聚合根；子实体通过根的方法变更 | `wsRepo.AddMember()` → `Workspace.AddMember()` + `Save(workspace)` |
| **外部只持有聚合根 ID** | 子实体 ID 不对外作为操作入口 | 禁止 `POST /versions/{id}/publish`，只允许 `POST /agents/{id}/publish` |
| **不变量在聚合内校验** | 命名唯一、状态机、配额等由根/工厂保证 | application 里 `ExistsByName` → `Project.Rename()` 内部校验 |
| **仓储只 Load/Save 聚合根** | Repository 接口以聚合为单位，不对子表单独 CRUD | `AgentRepository.Save(agent)` 持久化 agent + versions |
| **跨 BC 用 Port + ACL** | application 依赖本 BC 定义的 port interface，ACL 在 infrastructure | 不 import `tenant/domain/event`，改用 `shared/integration` 契约 |
| **领域事件从聚合产生** | 事件由聚合方法返回或根上的 `PullEvents()`，application 负责 Publish | handler 里手动拼装 event → `agent.Publish()` 返回 `AgentPublished` |

### 依赖方向（与 greenfield-go-ddd.md 一致）

```
interface/http → application → domain ← infrastructure
                                 ↑
                    port（domain/port 或 application/port）
                                 ↑
                         infrastructure/acl
```

---

## 二、共享集成契约（跨 BC 解耦）

Phase 1 中 `governance/subscriber` 直接 import `tenant/domain/event`，Phase 2 起改为 **集成事件契约**，放在 `shared/integration/`：

```
shared/integration/
├── workspace_created.go    # 集成事件 DTO（非 tenant 领域事件）
└── ports.go                  # 跨 BC Port 接口（可选集中放，或各 BC domain/port）
```

```go
// shared/integration/workspace_created.go
package integration

import (
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const EventWorkspaceCreated = "integration.WorkspaceCreated"

type WorkspaceCreated struct {
	WorkspaceID kernel.WorkspaceID
	OrgID       kernel.OrgID
	OwnerID     kernel.MemberID
	OccurredAt  time.Time
}

func (e WorkspaceCreated) EventName() string { return EventWorkspaceCreated }
```

**tenant BC 发布时做映射**（infrastructure 或 application 边界）：

```go
// tenant/application/command/create_workspace.go（片段）
ev := integration.WorkspaceCreated{
	WorkspaceID: ws.ID, OrgID: orgID, OwnerID: ownerID, OccurredAt: time.Now().UTC(),
}
return h.bus.Publish(ctx, ev)
```

**governance subscriber 只依赖 integration 包**，不再 import tenant domain。

---

## 三、workspace_app BC — Project 聚合

### 3.1 聚合边界

```
Project（聚合根）
├── ProjectID
├── WorkspaceID      ← 值对象引用，不做 FK
├── Name             ← 值对象 ProjectName
├── Description
├── Status           ← ProjectStatus: active | archived
├── CreatedBy        ← MemberID
├── CreatedAt
└── （无子实体，MVP 单表即可）
```

**不变量：**
- `(workspace_id, name)` 工作区内唯一
- 只有 `active` 状态允许挂载新 Agent（agent BC 通过 Port 查询）
- `archived` 后不可 rename

### 3.2 领域模型

```go
// workspace_app/domain/model/project.go
package model

import (
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectStatus string

const (
	ProjectStatusActive   ProjectStatus = "active"
	ProjectStatusArchived ProjectStatus = "archived"
)

// ProjectName 值对象 — 校验非空、长度、trim
type ProjectName struct{ value string }

func NewProjectName(raw string) (ProjectName, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 255 {
		return ProjectName{}, domain.ErrInvalidProjectName
	}
	return ProjectName{value: v}, nil
}

func (n ProjectName) String() string { return n.value }

type Project struct {
	ID          kernel.ProjectID
	WorkspaceID kernel.WorkspaceID
	Name        ProjectName
	Description string
	Status      ProjectStatus
	CreatedBy   kernel.MemberID
	CreatedAt   time.Time

	events []domain.Event // 未提交的领域事件（可选模式）
}

func NewProject(wsID kernel.WorkspaceID, name ProjectName, createdBy kernel.MemberID, desc string) *Project {
	return &Project{
		ID:          kernel.NewProjectID(),
		WorkspaceID: wsID,
		Name:        name,
		Description: strings.TrimSpace(desc),
		Status:      ProjectStatusActive,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now().UTC(),
	}
}

func (p *Project) Archive() error {
	if p.Status == ProjectStatusArchived {
		return domain.ErrProjectAlreadyArchived
	}
	p.Status = ProjectStatusArchived
	p.record(domain.NewProjectArchived(p.ID, p.WorkspaceID))
	return nil
}

func (p *Project) Rename(name ProjectName) error {
	if p.Status != ProjectStatusActive {
		return domain.ErrProjectNotActive
	}
	p.Name = name
	return nil
}

func (p *Project) IsActive() bool { return p.Status == ProjectStatusActive }

func (p *Project) PullEvents() []domain.Event {
	out := append([]domain.Event(nil), p.events...)
	p.events = nil
	return out
}

func (p *Project) record(e domain.Event) { p.events = append(p.events, e) }
```

### 3.3 仓储接口（只针对聚合根）

```go
// workspace_app/domain/repository/project_repository.go
package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectRepository interface {
	Save(ctx context.Context, project *model.Project) error
	FindByID(ctx context.Context, id kernel.ProjectID) (*model.Project, error)
	FindByWorkspaceAndName(ctx context.Context, wsID kernel.WorkspaceID, name model.ProjectName) (*model.Project, error)
	ListByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Project, error)
	CountByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (int, error)
}
```

> **注意**：`FindByWorkspaceAndName` 供 **工厂/领域服务** 做唯一性校验，而不是 application 直接 `ExistsByName`。

### 3.4 跨 BC Port（tenant ACL）

```go
// workspace_app/domain/port/workspace_checker.go
package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

// WorkspaceChecker — workspace_app 定义，tenant infrastructure 实现
type WorkspaceChecker interface {
	MemberInWorkspace(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID) (bool, error)
	CanCreateProject(ctx context.Context, wsID kernel.WorkspaceID, currentCount int) error
}
```

```go
// workspace_app/infrastructure/acl/tenant_acl.go
package acl

import (
	"context"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type TenantACL struct{ db *gorm.DB }

func NewTenantACL(db *gorm.DB) *TenantACL { return &TenantACL{db: db} }

func (a *TenantACL) MemberInWorkspace(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).Table("tenant.workspace_members").
		Where("workspace_id = ? AND member_id = ?", wsID.String(), memberID.String()).
		Count(&count).Error
	return count > 0, err
}

func (a *TenantACL) CanCreateProject(ctx context.Context, wsID kernel.WorkspaceID, currentCount int) error {
	// 读 tenant.workspaces.max_projects，与 Phase 1 Workspace.CanCreateAgent 对称
	return nil
}

var _ port.WorkspaceChecker = (*TenantACL)(nil)
```

### 3.5 Application 用例（薄编排）

```go
// workspace_app/application/command/create_project.go
func (h *CreateProjectHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, rawName, desc string) (*model.Project, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil || !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}

	name, err := model.NewProjectName(rawName)
	if err != nil {
		return nil, err
	}
	if existing, _ := h.repo.FindByWorkspaceAndName(ctx, wsID, name); existing != nil {
		return nil, domain.ErrProjectNameExists
	}

	count, err := h.repo.CountByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	if err := h.wsChecker.CanCreateProject(ctx, wsID, count); err != nil {
		return nil, err
	}

	project := model.NewProject(wsID, name, memberID, desc)
	if err := h.repo.Save(ctx, project); err != nil {
		return nil, err
	}
	for _, e := range project.PullEvents() {
		_ = h.bus.Publish(ctx, e)
	}
	return project, nil
}
```

### 3.6 事务边界

```go
// workspace_app/infrastructure/persistence/unit_of_work.go（可选，MVP 可用 db.Transaction 包一层）
func (s *ProjectStore) Save(ctx context.Context, project *model.Project) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return saveProject(tx, project)
	})
}
```

---

## 四、agent BC — Agent 聚合（含 AgentVersion 子实体）

### 4.1 聚合边界

```
Agent（聚合根）
├── AgentID
├── ProjectID
├── Name                 ← AgentName 值对象
├── Status               ← draft | published | archived
├── CurrentVersionID     ← 指向已发布版本（nullable）
├── CreatedAt
└── versions[]           ← AgentVersion 子实体（仅通过 Agent 方法追加）
        ├── VersionID
        ├── VersionNum   ← 单调递增 int
        ├── SystemPrompt
        ├── ModelName
        ├── ModelConfig
        └── ToolIDs
```

**不变量：**
- `(project_id, name)` 项目内唯一
- **AgentVersion 不可变**：创建后不能改，只能新增版本
- **Publish 状态机**：仅 `draft`/`published` 可 publish；必须 publish 指定已存在的 version
- **CurrentVersionID** 只能指向本聚合内的 version
- 新建 Agent 时自动创建 v1 草稿版本（可选，推荐）

### 4.2 领域模型

```go
// agent/domain/model/agent.go
package model

type AgentStatus string

const (
	AgentStatusDraft     AgentStatus = "draft"
	AgentStatusPublished AgentStatus = "published"
	AgentStatusArchived  AgentStatus = "archived"
)

type Agent struct {
	ID               kernel.AgentID
	ProjectID        kernel.ProjectID
	Name             AgentName
	Status           AgentStatus
	CurrentVersionID *kernel.AgentVersionID
	CreatedAt        time.Time
	versions         []*AgentVersion // 小写 — 包外不可直接修改

	events []domain.Event
}

// AgentVersion — 子实体，不单独作为聚合根
type AgentVersion struct {
	ID           kernel.AgentVersionID
	AgentID      kernel.AgentID
	Version      int
	SystemPrompt string
	ModelName    string
	ModelConfig  ModelConfig
	ToolIDs      []string
	CreatedAt    time.Time
}

func NewAgent(projectID kernel.ProjectID, name AgentName) *Agent {
	agent := &Agent{
		ID:        kernel.NewAgentID(),
		ProjectID: projectID,
		Name:      name,
		Status:    AgentStatusDraft,
		CreatedAt: time.Now().UTC(),
	}
	return agent
}

// AddVersion — 唯一入口创建新版本
func (a *Agent) AddVersion(prompt, modelName string, cfg ModelConfig, tools []string) (*AgentVersion, error) {
	if a.Status == AgentStatusArchived {
		return nil, domain.ErrAgentArchived
	}
	next := len(a.versions) + 1
	ver := &AgentVersion{
		ID:           kernel.NewAgentVersionID(),
		AgentID:      a.ID,
		Version:      next,
		SystemPrompt: prompt,
		ModelName:    modelName,
		ModelConfig:  cfg,
		ToolIDs:      append([]string(nil), tools...),
		CreatedAt:    time.Now().UTC(),
	}
	a.versions = append(a.versions, ver)
	a.record(domain.NewAgentVersionCreated(a.ID, ver.ID, ver.Version))
	return ver, nil
}

// Publish — 状态机 + 产生集成事件
func (a *Agent) Publish(versionID kernel.AgentVersionID) (domain.AgentPublished, error) {
	if a.Status == AgentStatusArchived {
		return domain.AgentPublished{}, domain.ErrAgentArchived
	}
	ver := a.findVersion(versionID)
	if ver == nil {
		return domain.AgentPublished{}, domain.ErrVersionNotFound
	}
	a.Status = AgentStatusPublished
	a.CurrentVersionID = &ver.ID
	ev := domain.NewAgentPublished(a.ID, a.ProjectID, ver.ID, ver.Version)
	a.record(ev)
	return ev, nil
}

func (a *Agent) PublishedVersion() (*AgentVersion, error) {
	if a.CurrentVersionID == nil {
		return nil, domain.ErrAgentNotPublished
	}
	return a.findVersion(*a.CurrentVersionID), nil
}

func (a *Agent) Versions() []*AgentVersion {
	return append([]*AgentVersion(nil), a.versions...)
}

func (a *Agent) findVersion(id kernel.AgentVersionID) *AgentVersion {
	for _, v := range a.versions {
		if v.ID == id {
			return v
		}
	}
	return nil
}
```

### 4.3 仓储 — 一次 Load 整个聚合

```go
// agent/domain/repository/agent_repository.go
type AgentRepository interface {
	Save(ctx context.Context, agent *model.Agent) error
	FindByID(ctx context.Context, id kernel.AgentID) (*model.Agent, error)
	FindByProjectAndName(ctx context.Context, projectID kernel.ProjectID, name model.AgentName) (*model.Agent, error)
	ListByProject(ctx context.Context, projectID kernel.ProjectID) ([]*model.Agent, error)
	CountByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (int, error) // 通过 project join 或 ACL
}
```

**持久化策略**（infrastructure）：

```
Save(agent):
  1. UPSERT agent.agents（status, current_version_id）
  2. INSERT 新增的 agent_versions（对比已持久化版本号）
  — 不 UPDATE 已有 version 行（不可变）
```

```go
// agent/infrastructure/persistence/agent_repo.go（片段）
func (r *AgentRepo) FindByID(ctx context.Context, id kernel.AgentID) (*model.Agent, error) {
	agentRow, err := loadAgentRow(ctx, r.db, id)
	if err != nil {
		return nil, err
	}
	versionRows, err := loadVersionRows(ctx, r.db, id)
	if err != nil {
		return nil, err
	}
	return mapToAgent(agentRow, versionRows), nil
}
```

### 4.4 跨 BC Port

```go
// agent/domain/port/project_checker.go
type ProjectChecker interface {
	ProjectInWorkspace(ctx context.Context, projectID kernel.ProjectID, wsID kernel.WorkspaceID) (bool, error)
	ProjectIsActive(ctx context.Context, projectID kernel.ProjectID) (bool, error)
}

// agent/domain/port/quota_checker.go
type QuotaChecker interface {
	CanCreateAgent(ctx context.Context, wsID kernel.WorkspaceID, currentCount int) error
}
```

| Port | 定义方 | 实现方 | ACL 位置 |
|------|--------|--------|----------|
| `ProjectChecker` | agent | workspace_app + tenant | `agent/infrastructure/acl/workspace_app_acl.go` |
| `QuotaChecker` | agent | tenant | `agent/infrastructure/acl/tenant_acl.go` |

### 4.5 Publish 用例（完整编排）

```go
// agent/application/command/publish_agent.go
func (h *PublishAgentHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, agentID kernel.AgentID, versionID kernel.AgentVersionID) error {
	agent, err := h.repo.FindByID(ctx, agentID)
	if err != nil {
		return err
	}

	inWs, err := h.projectChecker.ProjectInWorkspace(ctx, agent.ProjectID, wsID)
	if err != nil || !inWs {
		return domain.ErrProjectAccessDenied
	}

	ev, err := agent.Publish(versionID) // ← 不变量全在聚合内
	if err != nil {
		return err
	}

	if err := h.repo.Save(ctx, agent); err != nil {
		return err
	}
	return h.bus.Publish(ctx, ev)
}
```

### 4.6 CreateAgent 用例（含配额）

```go
func (h *CreateAgentHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, rawName, prompt, modelName string) (*model.Agent, error) {
	active, err := h.projectChecker.ProjectIsActive(ctx, projectID)
	if err != nil || !active {
		return nil, domain.ErrProjectNotActive
	}

	count, err := h.repo.CountByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	if err := h.quotaChecker.CanCreateAgent(ctx, wsID, count); err != nil {
		return err // → tenant.ErrQuotaExceeded
	}

	name, err := model.NewAgentName(rawName)
	if err != nil {
		return nil, err
	}
	if existing, _ := h.repo.FindByProjectAndName(ctx, projectID, name); existing != nil {
		return nil, domain.ErrAgentNameExists
	}

	agent := model.NewAgent(projectID, name)
	if _, err := agent.AddVersion(prompt, modelName, model.DefaultModelConfig(), nil); err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, agent); err != nil {
		return nil, err
	}
	for _, e := range agent.PullEvents() {
		_ = h.bus.Publish(ctx, e)
	}
	return agent, nil
}
```

---

## 五、platform/authz 解耦（Phase 2 一并修正）

Phase 1 中 `platform/authz` 依赖 `governance/application/query.Checker`，应改为：

```go
// platform/authz/checker.go
package authz

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PermissionChecker interface {
	HasPermission(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, perm string) (bool, error)
}

func RequirePermission(checker PermissionChecker, perm string) gin.HandlerFunc { /* 不变 */ }
```

`governance/module.go` 注册时：`authz.RequirePermission(checker, authz.PermProjectWrite)`  
`Checker` 实现 `PermissionChecker` 接口（同包适配，零成本）。

---

## 六、目录结构（Phase 2 BC 标准）

```
internal/modules/workspace_app/
├── module.go
├── domain/
│   ├── model/
│   │   ├── project.go          # 聚合根 + 值对象
│   │   └── project_test.go     # 不变量 / 状态机测试
│   ├── event/
│   │   └── project_created.go
│   ├── port/
│   │   └── workspace_checker.go
│   ├── repository/
│   │   └── project_repository.go
│   └── errors.go
├── application/
│   ├── command/
│   │   ├── create_project.go
│   │   └── archive_project.go
│   └── query/
│       ├── get_project.go
│       └── list_projects.go
├── infrastructure/
│   ├── persistence/
│   │   └── project_repo.go
│   └── acl/
│       └── tenant_acl.go
└── interface/http/
    ├── handler.go
    └── dto.go

internal/modules/agent/
├── domain/
│   ├── model/
│   │   ├── agent.go            # 聚合根 + AgentVersion 子实体
│   │   └── agent_test.go       # Publish 状态机、版本不可变
│   ├── event/
│   │   └── agent_published.go
│   ├── port/
│   │   ├── project_checker.go
│   │   └── quota_checker.go
│   └── repository/
│       └── agent_repository.go
├── application/
│   ├── command/
│   │   ├── create_agent.go
│   │   ├── add_version.go
│   │   └── publish_agent.go
│   └── query/
│       ├── get_agent.go
│       └── list_agents.go
├── infrastructure/
│   ├── persistence/
│   │   └── agent_repo.go       # Load/Save 整聚合
│   └── acl/
│       ├── workspace_app_acl.go
│       └── tenant_acl.go
└── interface/http/
    ├── handler.go
    └── dto.go
```

---

## 七、测试清单（每个 BC 必做）

### domain/model（>80% 覆盖）

```go
// agent/domain/model/agent_test.go
func TestAgent_Publish_Success(t *testing.T) {
	agent := model.NewAgent(projectID, mustName("bot"))
	ver, _ := agent.AddVersion("you are helpful", "gpt-4o", model.DefaultModelConfig(), nil)
	ev, err := agent.Publish(ver.ID)
	require.NoError(t, err)
	require.Equal(t, model.AgentStatusPublished, agent.Status)
	require.Equal(t, ver.ID, *agent.CurrentVersionID)
	require.Equal(t, agent.ID, ev.AgentID)
}

func TestAgent_Publish_UnknownVersion(t *testing.T) {
	agent := model.NewAgent(projectID, mustName("bot"))
	_, err := agent.Publish(kernel.NewAgentVersionID())
	require.ErrorIs(t, err, domain.ErrVersionNotFound)
}

func TestAgent_AddVersion_AfterArchived(t *testing.T) {
	agent := model.NewAgent(projectID, mustName("bot"))
	agent.Archive() // 需实现
	_, err := agent.AddVersion("x", "gpt-4o", model.DefaultModelConfig(), nil)
	require.ErrorIs(t, err, domain.ErrAgentArchived)
}
```

### application（mock port + memory repo）

- CreateAgent 超 quota → `ErrQuotaExceeded`
- Publish 未归属 workspace 的 agent → `ErrProjectAccessDenied`

### interface/http（httptest ≥ 1 happy path）

- Create Project → List → Get
- Create Agent → Add Version → Publish → Get 含 current_version

---

## 八、与 Phase 1 对照迁移（可选，非阻塞）

| Phase 1 现状 | Phase 2 建议 |
|--------------|-------------|
| `governance` import `tenant/domain/event` | 改为 `shared/integration.WorkspaceCreated` |
| `platform/authz` import governance query | 改为 `PermissionChecker` 接口 |
| `tenant.WorkspaceRepository.AddMember` | 后续重构为 `Workspace` 聚合方法 |
| `governance` Role/Binding 分开 Save | 后续合并为 `Role` 聚合根 |

Phase 2 **新 BC 严格按本文**；Phase 1 三个 BC 可在 Phase 2 Task 开头做 **小步重构**（integration 事件 + authz 接口），不必一次性改完。

---

## 九、Phase 2 实施顺序

```
1. shared/integration 集成事件 + platform/authz PermissionChecker 接口
2. workspace_app BC（Project 聚合 + tenant ACL）
3. agent BC（Agent 聚合 + workspace_app/tenant ACL）
4. bootstrap 注册 + migrations 000004/000005
5. HTTP 验收 + domain 测试
```

**验收标准**（与 backend-implementation-plan.md 一致）：

- [ ] Project CRUD，`(workspace, name)` 唯一
- [ ] Agent 创建 → 添加 Version → Publish，状态机正确
- [ ] 超 `max_agents` quota → 403
- [ ] 所有 BC `go test ./...` 通过
- [ ] domain 层无 GORM/HTTP import

---

**文档路径**：`spec/aggregate-modeling-template.md`  
**下一步**：按 Task 2.1 实现 `workspace_app` BC，或先执行 §八 小步重构。
