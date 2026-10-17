package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/sparkops/sparkops-agent/backend/internal/devmemory"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	identityhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/observability"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

const (
	seedEmail    = "demo@sparkops.local"
	seedPassword = "Demo1234!"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	logger := observability.NewLogger(cfg)
	engine := httpx.NewEngine()
	bus := eventbus.NewInMemory()
	tokens := authn.NewTokenProvider(cfg.JWT)

	repo := devmemory.NewMemberRepository()
	register := command.NewRegisterMemberHandler(repo, bus, postgres.NewDirectTransactor())
	login := command.NewLoginHandler(repo, tokens)
	me := query.NewGetCurrentMemberHandler(repo, nil)
	hasher := model.BcryptHasher{}
	updateProfile := command.NewUpdateProfileHandler(repo)
	changePassword := command.NewChangePasswordHandler(repo, hasher)
	handler := identityhttp.NewHandler(register, login, me, updateProfile, changePassword)

	auth := engine.Group("/api/v1/auth")
	auth.POST("/register", handler.Register)
	auth.POST("/login", handler.Login)
	auth.GET("/me", authn.RequireAuth(tokens), handler.Me)
	auth.PATCH("/me", authn.RequireAuth(tokens), handler.UpdateProfile)
	auth.PATCH("/password", authn.RequireAuth(tokens), handler.ChangePassword)

	seedDemoUser(context.Background(), register, logger)

	addr := cfg.Server.Addr
	if addr == "" {
		addr = ":8080"
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("dev server starting (in-memory identity)", zap.String("addr", addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Info("dev server shutting down")
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
}

func loadConfig() (*config.Config, error) {
	if cfg, err := config.LoadFromEnv(); err == nil {
		return cfg, nil
	}
	return &config.Config{
		Server: config.ServerConfig{Addr: ":8080"},
		JWT: config.JWTConfig{
			Secret:     "sparkops-agent-secret",
			ExpireHour: 24,
		},
	}, nil
}

func seedDemoUser(ctx context.Context, register *command.RegisterMemberHandler, logger *zap.Logger) {
	_, err := register.Handle(ctx, command.RegisterMemberInput{
		Email:       seedEmail,
		Password:    seedPassword,
		DisplayName: "Demo User",
	})
	if err != nil {
		logger.Warn("demo user seed skipped", zap.Error(err))
		return
	}
	logger.Info("demo user seeded", zap.String("email", seedEmail))
}
