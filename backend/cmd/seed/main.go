package main

import (
	"context"
	"fmt"
	"log"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/bootstrap"
	agentcommand "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/application/command"
	agentacl "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/infrastructure/acl"
	agentpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/infrastructure/persistence"
	govcommand "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/subscriber"
	govpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	identitycommand "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	identityquery "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/query"
	identitypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
	tenantcommand "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/application/command"
	tenantpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/infrastructure/persistence"
	wscommand "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	wsacl "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/acl"
	wspersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	seedEmail    = "demo@sparkops.local"
	seedPassword = "Demo1234!"
)

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

	memberID, err := ensureMember(ctx, register, login)
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

	org, err := createOrg.Handle(ctx, memberID, "Demo Organization", "demo-org")
	if err != nil {
		log.Fatal(err)
	}
	ws, err := createWs.Handle(ctx, org.ID, memberID, "Demo Workspace", "demo-ws")
	if err != nil {
		log.Fatal(err)
	}

	projectRepo := wspersistence.NewProjectRepository(db)
	memberRepo := wspersistence.NewProjectMemberRepository(db)
	createProject := wscommand.NewCreateProjectHandler(projectRepo, memberRepo, wsacl.NewTenantACL(db), nil, bus)
	project, err := createProject.Handle(ctx, ws.ID, memberID, "Demo Project", "Seed project")
	if err != nil {
		log.Fatal(err)
	}

	agentRepo := agentpersistence.NewAgentRepository(db)
	projectChecker := agentacl.NewWorkspaceAppACL(db)
	quotaChecker := agentacl.NewTenantACL(db)
	createAgent := agentcommand.NewCreateAgentHandler(agentRepo, projectChecker, quotaChecker, bus)
	publishAgent := agentcommand.NewPublishAgentHandler(agentRepo, projectChecker, bus)

	agent, err := createAgent.Handle(ctx, ws.ID, project.ID, "Demo Agent", "You are a helpful assistant.", "mock-gpt")
	if err != nil {
		log.Fatal(err)
	}
	versions := agent.Versions()
	if len(versions) == 0 {
		log.Fatal("agent has no versions")
	}
	if err := publishAgent.Handle(ctx, ws.ID, agent.ID, versions[0].ID); err != nil {
		log.Fatal(err)
	}

	loginRes, err := login.Handle(ctx, identitycommand.LoginInput{Email: seedEmail, Password: seedPassword})
	if err != nil {
		log.Fatal(err)
	}

	printSummary(loginRes.Token, memberID, ws.ID, project.ID, agent.ID)
}

func wireGovernanceBootstrap(bus eventbus.Bus, db *gorm.DB) {
	store := govpersistence.NewStore(db)
	bootstrap := govcommand.NewBootstrapWorkspaceHandler(
		govpersistence.NewRoleRepository(store),
		govpersistence.NewBindingRepository(store),
		nil,
	)
	bus.Subscribe(integration.EventWorkspaceCreated, subscriber.NewOnWorkspaceCreated(bootstrap).Handle)
}

func ensureMember(ctx context.Context, register *identitycommand.RegisterMemberHandler, login *identitycommand.LoginHandler) (kernel.MemberID, error) {
	if res, err := login.Handle(ctx, identitycommand.LoginInput{Email: seedEmail, Password: seedPassword}); err == nil {
		return res.MemberID, nil
	}
	res, err := register.Handle(ctx, identitycommand.RegisterMemberInput{
		Email: seedEmail, Password: seedPassword, DisplayName: "Demo User",
	})
	if err != nil {
		return kernel.MemberID{}, err
	}
	return res.MemberID, nil
}

func printSummary(token string, memberID kernel.MemberID, wsID kernel.WorkspaceID, projectID kernel.ProjectID, agentID kernel.AgentID) {
	fmt.Println("=== SparkOps demo seed complete ===")
	fmt.Printf("Email:        %s\n", seedEmail)
	fmt.Printf("Password:     %s\n", seedPassword)
	fmt.Printf("Member ID:    %s\n", memberID.String())
	fmt.Printf("Workspace ID: %s\n", wsID.String())
	fmt.Printf("Project ID:   %s\n", projectID.String())
	fmt.Printf("Agent ID:     %s\n", agentID.String())
	fmt.Printf("Token:        %s\n", token)
	fmt.Println()
	fmt.Println("# Chat example")
	fmt.Printf("curl -X POST http://localhost:8080/api/v1/agents/%s/chat \\\n", agentID.String())
	fmt.Println("  -H \"Authorization: Bearer <token>\" \\")
	fmt.Printf("  -H \"X-Workspace-Id: %s\" \\\n", wsID.String())
	fmt.Println("  -H \"Content-Type: application/json\" \\")
	fmt.Println("  -d '{\"message\":\"Hello\"}'")
}
