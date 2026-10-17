package identity

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/service"
	identityevent "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
	identityhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "identity" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("identity module requires database")
	}
	if err := identityevent.Register(c.EventRegistry); err != nil {
		return fmt.Errorf("identity event codecs: %w", err)
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("identity migrate: %w", err)
	}
	systemRoleStore := persistence.NewSystemRoleStore(c.DB)
	if err := systemRoleStore.AutoMigrate(); err != nil {
		return fmt.Errorf("identity system role migrate: %w", err)
	}
	externalStore := persistence.NewExternalIdentityRepository(c.DB)
	if err := externalStore.AutoMigrate(); err != nil {
		return fmt.Errorf("identity external identity migrate: %w", err)
	}
	secretBox, err := crypto.NewSecretBoxFromConfig(c.Config)
	if err != nil {
		return fmt.Errorf("identity SSO secret crypto: %w", err)
	}
	ssoRepo := persistence.NewSSOConfigRepository(c.DB, secretBox)
	if err := ssoRepo.AutoMigrate(); err != nil {
		return fmt.Errorf("identity SSO config migrate: %w", err)
	}

	repo := persistence.NewMemberRepository(c.DB)
	systemRoles := query.NewSystemRoleChecker(systemRoleStore)
	c.SystemRoleChecker = systemRoles

	register := command.NewRegisterMemberHandler(repo, c.EventBus, c.Transactor)
	login := command.NewLoginHandler(repo, m.tokens)
	me := query.NewGetCurrentMemberHandler(repo, systemRoles)
	hasher := model.BcryptHasher{}
	updateProfile := command.NewUpdateProfileHandler(repo)
	changePassword := command.NewChangePasswordHandler(repo, hasher)
	handler := identityhttp.NewHandler(register, login, me, updateProfile, changePassword)

	resolver := command.NewResolveExternalIdentityHandler(repo, externalStore, m.tokens)
	ssoRuntime := service.NewSSORuntime(
		ssoRepo,
		c.Config.Auth,
		&http.Client{Timeout: 10 * time.Second},
	)
	loadContext := c.Context
	if loadContext == nil {
		loadContext = context.Background()
	}
	if err := ssoRuntime.Load(loadContext); err != nil {
		return fmt.Errorf("identity SSO runtime load: %w", err)
	}
	ssoManager := command.NewSSOConfigManager(ssoRepo, ssoRuntime)
	stateStore := command.NewOIDCStateStore()
	ssoHandler := identityhttp.NewSSOHandler(ssoRuntime, resolver, stateStore)
	ssoAdminHandler := identityhttp.NewSSOAdminHandler(ssoManager)

	createPlatformMember := command.NewCreatePlatformMemberHandler(repo, systemRoleStore, c.EventBus, c.Transactor)
	updatePlatformMember := command.NewUpdatePlatformMemberHandler(repo, systemRoleStore)
	resetPlatformPassword := command.NewAdminResetPasswordHandler(repo)
	deletePlatformMember := command.NewDeletePlatformMemberHandler(repo, systemRoleStore)
	listPlatformMembers := query.NewListPlatformMembersHandler(repo, systemRoleStore)
	platformHandler := identityhttp.NewPlatformHandler(
		listPlatformMembers, createPlatformMember, updatePlatformMember, resetPlatformPassword, deletePlatformMember,
	)
	superAdmin := authz.RequireSuperAdmin(c.SystemRoleChecker)

	auth := c.Engine.Group("/api/v1/auth")
	auth.POST("/register", handler.Register)
	auth.POST("/login", handler.Login)
	auth.GET("/me", authn.RequireAuth(m.tokens), handler.Me)
	auth.PATCH("/me", authn.RequireAuth(m.tokens), handler.UpdateProfile)
	auth.PATCH("/password", authn.RequireAuth(m.tokens), handler.ChangePassword)
	auth.GET("/providers", ssoHandler.AuthProviders)
	auth.GET("/oidc/login", ssoHandler.OIDCLogin)
	auth.GET("/oidc/callback", ssoHandler.OIDCCallback)
	auth.POST("/ldap/login", ssoHandler.LDAPLogin)

	adminSSO := c.Engine.Group("/api/v1/admin/sso", authn.RequireAuth(m.tokens), superAdmin)
	adminSSO.GET("", ssoAdminHandler.Get)
	adminSSO.POST("/test", ssoAdminHandler.Test)
	adminSSO.PUT("", ssoAdminHandler.Update)

	platform := c.Engine.Group("/api/v1/platform")
	platform.GET("/members", authn.RequireAuth(m.tokens), superAdmin, platformHandler.ListMembers)
	platform.POST("/members", authn.RequireAuth(m.tokens), superAdmin, platformHandler.CreateMember)
	platform.PATCH("/members/:memberId", authn.RequireAuth(m.tokens), superAdmin, platformHandler.UpdateMember)
	platform.PATCH("/members/:memberId/password", authn.RequireAuth(m.tokens), superAdmin, platformHandler.ResetPassword)
	platform.DELETE("/members/:memberId", authn.RequireAuth(m.tokens), superAdmin, platformHandler.DeleteMember)

	if c.Context != nil {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-c.Context.Done():
					return
				case <-ticker.C:
					_ = ssoRuntime.RefreshIfChanged(c.Context)
				}
			}
		}()
	}

	return nil
}
