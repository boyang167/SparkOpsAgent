package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/bootstrap"
	agentcommand "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/application/command"
	agentacl "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/infrastructure/acl"
	agentpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/infrastructure/persistence"
	govcommand "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/subscriber"
	govrepository "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	govpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	identitycommand "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	identityquery "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/query"
	identitypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
	tenantcommand "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/application/command"
	tenantdomain "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain"
	tenantmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/model"
	tenantrepository "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	tenantpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/infrastructure/persistence"
	wscommand "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	wsmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	wsrepository "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	wsacl "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/acl"
	wspersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	seedEmail    = "demo@sparkops.local"
	seedPassword = "Demo1234!"
	orgSlug      = "demo-org"
	wsSlug       = "demo-ws"
	projectSlug  = "demo-project"
)

var roleDisplayNames = map[string]string{
	"admin":     "管理员",
	"developer": "研发",
	"reviewer":  "审核员",
	"operator":  "运维",
	"viewer":    "只读",
}

type roleAccount struct {
	Role        string
	Email       string
	DisplayName string
}

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.Database.DSN == "" {
		log.Fatal("DATABASE_DSN or config.yaml database.dsn is required")
	}

	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	bus := eventbus.NewInMemory()
	tokens := authn.NewTokenProvider(cfg.JWT)

	if err := bootstrap.AutoMigrate(db); err != nil {
		log.Fatal(err)
	}
	wireGovernanceBootstrap(bus, db)

	memberRepo := identitypersistence.NewMemberRepository(db)
	register := identitycommand.NewRegisterMemberHandler(memberRepo, bus)
	login := identitycommand.NewLoginHandler(memberRepo, tokens)

	memberID, err := ensureMember(ctx, db, register, seedEmail, seedPassword, "Demo User")
	if err != nil {
		log.Fatal(err)
	}

	systemRoleStore := identitypersistence.NewSystemRoleStore(db)
	if err := systemRoleStore.AutoMigrate(); err != nil {
		log.Fatal(err)
	}
	systemRoles := identityquery.NewSystemRoleChecker(systemRoleStore)
	if err := systemRoles.GrantSuperAdmin(ctx, memberID); err != nil {
		log.Fatal(err)
	}

	tenantStore := tenantpersistence.NewTenantStore(db)
	orgRepo := tenantpersistence.NewOrganizationRepository(tenantStore)
	wsRepo := tenantpersistence.NewWorkspaceRepository(tenantStore)
	createOrg := tenantcommand.NewCreateOrganizationHandler(orgRepo, nil)
	createWs := tenantcommand.NewCreateWorkspaceHandler(orgRepo, wsRepo, bus)

	org, err := ensureOrganization(ctx, orgRepo, createOrg, memberID)
	if err != nil {
		log.Fatal(err)
	}
	ws, err := ensureWorkspace(ctx, wsRepo, createWs, org.ID, memberID)
	if err != nil {
		log.Fatal(err)
	}

	projectRepo := wspersistence.NewProjectRepository(db)
	projectMemberRepo := wspersistence.NewProjectMemberRepository(db)
	createProject := wscommand.NewCreateProjectHandler(
		projectRepo, projectMemberRepo, wsacl.NewTenantACL(db), wsacl.NewWorkspaceClusterACL(db), nil, bus,
	)
	project, err := ensureProject(ctx, projectRepo, createProject, ws.ID, memberID)
	if err != nil {
		log.Fatal(err)
	}

	govStore := govpersistence.NewStore(db)
	roleRepo := govpersistence.NewRoleRepository(govStore)
	bindingRepo := govpersistence.NewBindingRepository(govStore)
	if err := govcommand.NewEnsureDefaultWorkspaceRolesHandler(roleRepo).Handle(ctx); err != nil {
		log.Fatal(err)
	}
	if err := govcommand.EnsureMenuRolePolicies(ctx, db, govcommand.ApplyMenuRolePoliciesOptions{SkipIfConfigured: true}); err != nil {
		log.Fatal(err)
	}
	workspaces, err := listAllWorkspaces(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	accounts, err := seedRoleMembersAcrossWorkspaces(
		ctx, db, register, orgRepo, wsRepo, roleRepo, bindingRepo, projectRepo, projectMemberRepo, workspaces,
	)
	if err != nil {
		log.Fatal(err)
	}

	agentRepo := agentpersistence.NewAgentRepository(db)
	projectChecker := agentacl.NewWorkspaceAppACL(db)
	quotaChecker := agentacl.NewTenantACL(db)
	createAgent := agentcommand.NewCreateAgentHandler(agentRepo, projectChecker, quotaChecker, bus)
	publishAgent := agentcommand.NewPublishAgentHandler(agentRepo, projectChecker, bus)

	var agentID kernel.AgentID
	agent, err := createAgent.Handle(ctx, ws.ID, project.ID, "Demo Agent", "You are a helpful assistant.", "mock-gpt")
	if err != nil {
		log.Printf("skip demo agent: %v", err)
	} else {
		versions := agent.Versions()
		if len(versions) == 0 {
			log.Fatal("agent has no versions")
		}
		if err := publishAgent.Handle(ctx, ws.ID, agent.ID, versions[0].ID); err != nil {
			log.Fatal(err)
		}
		agentID = agent.ID
	}

	token := ""
	if loginRes, err := login.Handle(ctx, identitycommand.LoginInput{Email: seedEmail, Password: seedPassword}); err != nil {
		log.Printf("demo login skipped: %v", err)
	} else {
		token = loginRes.Token
	}

	printSummary(token, memberID, ws.ID, project.ID, agentID, accounts)
}

func wireGovernanceBootstrap(bus eventbus.Bus, db *gorm.DB) {
	store := govpersistence.NewStore(db)
	menuStore := govpersistence.NewMenuStore(db)
	roleRepo := govpersistence.NewRoleRepository(store)
	bootstrapHandler := govcommand.NewBootstrapWorkspaceHandler(
		roleRepo,
		govpersistence.NewBindingRepository(store),
		nil,
	)
	seedMenus := govcommand.NewSeedWorkspaceMenusHandler(menuStore)
	menuPolicies := govcommand.NewApplyMenuRolePoliciesHandler(menuStore, roleRepo, seedMenus)
	bus.Subscribe(integration.EventWorkspaceCreated, subscriber.NewOnWorkspaceCreated(bootstrapHandler, menuPolicies).Handle)
}

func ensureMember(
	ctx context.Context,
	db *gorm.DB,
	register *identitycommand.RegisterMemberHandler,
	email, password, displayName string,
) (kernel.MemberID, error) {
	if id, ok, err := findMemberIDByEmail(ctx, db, email); err != nil {
		return kernel.MemberID{}, err
	} else if ok {
		if err := db.WithContext(ctx).Unscoped().
			Table("identity.members").
			Where("id = ?", id.String()).
			Update("deleted_at", nil).Error; err != nil {
			return kernel.MemberID{}, err
		}
		return id, nil
	}
	res, err := register.Handle(ctx, identitycommand.RegisterMemberInput{
		Email: email, Password: password, DisplayName: displayName,
	})
	if err != nil {
		return kernel.MemberID{}, err
	}
	return res.MemberID, nil
}

func findMemberIDByEmail(ctx context.Context, db *gorm.DB, email string) (kernel.MemberID, bool, error) {
	var row struct{ ID string }
	err := db.WithContext(ctx).Unscoped().
		Table("identity.members").
		Select("id").
		Where("lower(email) = ?", strings.ToLower(email)).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return kernel.MemberID{}, false, nil
		}
		return kernel.MemberID{}, false, err
	}
	id, err := kernel.ParseMemberID(row.ID)
	if err != nil {
		return kernel.MemberID{}, false, err
	}
	return id, true, nil
}

func ensureOrganization(
	ctx context.Context,
	orgRepo tenantrepository.OrganizationRepository,
	createOrg *tenantcommand.CreateOrganizationHandler,
	ownerID kernel.MemberID,
) (*tenantmodel.Organization, error) {
	orgs, err := orgRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	for _, org := range orgs {
		if org.Slug == orgSlug {
			if err := orgRepo.AddMember(ctx, org.ID, ownerID, tenantdomain.OrgRoleOwner); err != nil {
				return nil, err
			}
			return org, nil
		}
	}
	return createOrg.Handle(ctx, ownerID, "Demo Organization", orgSlug)
}

func ensureWorkspace(
	ctx context.Context,
	wsRepo tenantrepository.WorkspaceRepository,
	createWs *tenantcommand.CreateWorkspaceHandler,
	orgID kernel.OrgID,
	ownerID kernel.MemberID,
) (*tenantmodel.Workspace, error) {
	workspaces, err := wsRepo.ListByMember(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	for _, ws := range workspaces {
		if ws.OrgID == orgID && ws.Slug == wsSlug {
			return ws, nil
		}
	}
	return createWs.Handle(ctx, orgID, ownerID, "Demo Workspace", wsSlug)
}

func ensureProject(
	ctx context.Context,
	projectRepo wsrepository.ProjectRepository,
	createProject *wscommand.CreateProjectHandler,
	wsID kernel.WorkspaceID,
	ownerID kernel.MemberID,
) (*wsmodel.Project, error) {
	name, err := wsmodel.NewProjectName(projectSlug)
	if err != nil {
		return nil, err
	}
	if existing, err := projectRepo.FindByWorkspaceAndName(ctx, wsID, name); err == nil && existing != nil {
		return existing, nil
	}
	return createProject.Handle(ctx, wsID, ownerID, wsmodel.CreateProjectInput{
		Name:        projectSlug,
		DisplayName: "Demo Project",
		Description: "Seed project",
	})
}

func listAllWorkspaces(ctx context.Context, db *gorm.DB) ([]*tenantmodel.Workspace, error) {
	var rows []struct {
		ID    string
		OrgID string
		Name  string
		Slug  string
	}
	err := db.WithContext(ctx).
		Table("tenant.workspaces").
		Select("id, org_id, name, slug").
		Where("deleted_at IS NULL").
		Order("created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*tenantmodel.Workspace, 0, len(rows))
	for _, row := range rows {
		wsID, err := kernel.ParseWorkspaceID(row.ID)
		if err != nil {
			continue
		}
		orgID, err := kernel.ParseOrgID(row.OrgID)
		if err != nil {
			continue
		}
		out = append(out, &tenantmodel.Workspace{
			ID: wsID, OrgID: orgID, Name: row.Name, Slug: row.Slug, Status: "active",
		})
	}
	return out, nil
}

func seedRoleMembersAcrossWorkspaces(
	ctx context.Context,
	db *gorm.DB,
	register *identitycommand.RegisterMemberHandler,
	orgRepo tenantrepository.OrganizationRepository,
	wsRepo tenantrepository.WorkspaceRepository,
	roleRepo govrepository.RoleRepository,
	bindingRepo govrepository.BindingRepository,
	projectRepo wsrepository.ProjectRepository,
	projectMembers wsrepository.ProjectMemberRepository,
	workspaces []*tenantmodel.Workspace,
) ([]roleAccount, error) {
	assign := govcommand.NewAssignRoleHandler(roleRepo, bindingRepo, nil)
	accounts := make([]roleAccount, 0, len(authz.DefaultWorkspaceRoles))
	memberByRole := make(map[string]kernel.MemberID, len(authz.DefaultWorkspaceRoles))

	for _, definition := range authz.DefaultWorkspaceRoles {
		displayName := roleDisplayNames[definition.Name]
		if displayName == "" {
			displayName = definition.Name
		}
		email := definition.Name + "@sparkops.local"
		memberID, err := ensureMember(ctx, db, register, email, seedPassword, displayName)
		if err != nil {
			return nil, err
		}
		memberByRole[definition.Name] = memberID
		accounts = append(accounts, roleAccount{Role: definition.Name, Email: email, DisplayName: displayName})
	}

	for _, ws := range workspaces {
		roles, err := roleRepo.ListByWorkspace(ctx, ws.ID)
		if err != nil {
			return nil, err
		}
		byName := make(map[string]kernel.RoleID, len(roles))
		for _, role := range roles {
			byName[role.Name] = role.ID
		}
		projects, err := projectRepo.ListByWorkspace(ctx, ws.ID)
		if err != nil {
			return nil, err
		}
		for _, definition := range authz.DefaultWorkspaceRoles {
			roleID, ok := byName[definition.Name]
			if !ok {
				return nil, fmt.Errorf("workspace %s missing default role %s", ws.Slug, definition.Name)
			}
			memberID := memberByRole[definition.Name]
			if err := orgRepo.AddMember(ctx, ws.OrgID, memberID, tenantdomain.OrgRoleMember); err != nil {
				return nil, err
			}
			if err := wsRepo.AddMember(ctx, ws.ID, memberID, "member"); err != nil {
				return nil, err
			}
			if err := assign.Handle(ctx, ws.ID, roleID, memberID); err != nil {
				return nil, err
			}
			for _, project := range projects {
				if err := projectMembers.Save(ctx, wsmodel.NewProjectMember(project.ID, memberID, projectACLForRole(definition.Name))); err != nil {
					return nil, err
				}
			}
		}
		fmt.Printf("seeded role accounts into workspace %s (%s)\n", ws.Name, ws.ID.String())
	}
	return accounts, nil
}

func projectACLForRole(roleName string) string {
	switch roleName {
	case "admin":
		return wsmodel.ProjectRoleLead
	case "developer":
		return wsmodel.ProjectRoleMember
	default:
		return wsmodel.ProjectRoleViewer
	}
}

func printSummary(
	token string,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	agentID kernel.AgentID,
	accounts []roleAccount,
) {
	fmt.Println("=== SparkOps demo seed complete ===")
	fmt.Printf("Email:        %s\n", seedEmail)
	fmt.Printf("Password:     %s\n", seedPassword)
	fmt.Printf("Member ID:    %s\n", memberID.String())
	fmt.Printf("Workspace ID: %s\n", wsID.String())
	fmt.Printf("Project ID:   %s\n", projectID.String())
	if agentID != (kernel.AgentID{}) {
		fmt.Printf("Agent ID:     %s\n", agentID.String())
	}
	if token != "" {
		fmt.Printf("Token:        %s\n", token)
	}
	fmt.Println()
	fmt.Println("Role accounts (password: " + seedPassword + ")")
	for _, account := range accounts {
		fmt.Printf("  %-10s  %-28s  %s\n", account.Role, account.Email, account.DisplayName)
	}
	fmt.Println()
	fmt.Println("# Chat example")
	if agentID == (kernel.AgentID{}) {
		fmt.Println("curl example skipped (no demo agent)")
		return
	}
	fmt.Printf("curl -X POST http://localhost:8080/api/v1/agents/%s/chat \\\n", agentID.String())
	fmt.Println("  -H \"Authorization: Bearer <token>\" \\")
	fmt.Printf("  -H \"X-Workspace-Id: %s\" \\\n", wsID.String())
	fmt.Println("  -H \"Content-Type: application/json\" \\")
	fmt.Println("  -d '{\"message\":\"Hello\"}'")
}
