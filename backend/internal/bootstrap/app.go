package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/integration"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin"
	adminpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/infrastructure/persistence"
	pluginmod "github.com/sparkops/sparkops-agent/backend/internal/modules/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/applog"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/observability"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/operationlog"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/redis"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/routes"
	_ "github.com/sparkops/sparkops-agent/backend/internal/plugins"
)

func Run(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	logger := observability.NewLogger(cfg)

	db, err := openDatabase(cfg, logger)
	if err != nil {
		return err
	}
	if db != nil {
		if err := AutoMigrate(db); err != nil {
			return fmt.Errorf("auto migrate: %w", err)
		}
	}
	if db != nil || config.IsProduction() {
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("config validation: %w", err)
		}
	}

	redisClient, err := openRedis(cfg, logger)
	if err != nil {
		return err
	}

	msgFactory, err := messaging.NewFromConfig(cfg)
	if err != nil {
		return err
	}

	k8sFactory, err := openK8s(cfg, logger)
	if err != nil {
		return err
	}

	c := &appctx.Container{
		Context:      ctx,
		Config:       cfg,
		DB:           db,
		Redis:        redisClient,
		Logger:       logger,
		Engine:       httpx.NewEngine(),
		EventBus:     msgFactory.Bus,
		MessageQueue: msgFactory.Queue,
		K8sFactory:   k8sFactory,
	}
	httpx.RegisterReadiness(c.Engine, httpx.ReadinessDeps{DB: db, Redis: redisClient})

	tokens := authn.NewTokenProvider(cfg.JWT)

	if db != nil {
		appLogStore := applog.NewStore(db)
		if err := appLogStore.AutoMigrate(); err != nil {
			return fmt.Errorf("app log migrate: %w", err)
		}
		observability.SetAppLogStore(appLogStore)

		opStore := operationlog.NewStore(db)
		if err := opStore.AutoMigrate(); err != nil {
			return fmt.Errorf("operation log migrate: %w", err)
		}
		httpx.SetOperationLogMiddleware(operationlog.Middleware(opStore))
	}

	c.Engine.Use(observability.RequestLogMiddleware(logger))

	modules := []appctx.Module{}
	if db != nil {
		modules = append(modules,
			identity.New(tokens),
			tenant.New(tokens),
			governance.New(tokens),
			workspace_app.New(tokens),
			platform_admin.New(tokens),
			runtime.New(tokens),
			delivery.New(tokens),
			product.New(tokens),
			agent.New(tokens),
			workflow.New(tokens),
			knowledge.New(tokens),
			notification.New(tokens),
			integration.New(tokens),
			agent_runtime.New(tokens),
			mcp.New(tokens),
			pluginmod.New(tokens),
		)
	} else {
		logger.Warn("database unavailable, skipping business modules")
	}

	for _, m := range modules {
		if err := m.Register(c); err != nil {
			return fmt.Errorf("register module %s: %w", m.Name(), err)
		}
	}

	entitlements.ConfigureFromEnv()

	if tokens != nil {
		c.Engine.Use(authn.OptionalAuth(tokens))
	}
	if c.PolicyEngine != nil && c.PermissionChecker != nil && c.APIPolicyChecker != nil {
		c.Engine.Use(authz.APIAccessMiddleware(c.PolicyEngine, c.PermissionChecker, c.APIPolicyChecker))
	}
	if c.DB != nil {
		c.Engine.Use(entitlements.Middleware(c.DB))
	}
	c.Engine.Use(authn.RequireMemberForPluginAPIs())

	pluginContext := sparkplugin.PluginContext{Logger: logger, DB: c.DB}
	if c.DB != nil {
		adminRepo := adminpersistence.NewAdminRepository(c.DB)
		pluginContext.Clusters = sparkplugin.NewFactory(newClusterCredentialSource(adminRepo))
	}
	if c.PermissionChecker != nil {
		pluginContext.Writer = newGovernanceWriteAuthorizer(c.PermissionChecker)
	}
	sparkplugin.InstallAllWithContext(c.Engine, pluginContext)
	c.RouteRegistry = routes.Collect(c.Engine)

	server := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           c.Engine,
		ReadHeaderTimeout: 10 * time.Second,
	}
	c.Server = server

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", zap.String("addr", cfg.Server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		logger.Info("server shutting down")
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func loadConfig() (*config.Config, error) {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return config.Load(path)
	}
	if _, err := os.Stat("config.yaml"); err == nil {
		return config.Load("config.yaml")
	}
	logger := observability.NewNopLogger()
	logger.Info("config.yaml not found, using defaults (health only)")
	return &config.Config{
		Server: config.ServerConfig{Addr: ":8080"},
	}, nil
}

func openDatabase(cfg *config.Config, logger *zap.Logger) (*gorm.DB, error) {
	if cfg.Database.DSN == "" {
		logger.Warn("database DSN empty, running without DB")
		return nil, nil
	}
	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf(
			"connect database: %w (host unreachable? use local Postgres via `make docker-up`, or restore VPN/network to remote DB in config.yaml)",
			err,
		)
	}
	logger.Info("database connected")
	return db, nil
}

func openRedis(cfg *config.Config, logger *zap.Logger) (*redis.Client, error) {
	if cfg.Redis.Addr == "" {
		logger.Warn("redis addr empty, running without cache")
		return redis.NewClient(redis.Config{})
	}
	client, err := redis.NewClient(redis.Config{Addr: cfg.Redis.Addr})
	if err != nil {
		return nil, fmt.Errorf("create redis client: %w", err)
	}
	if err := client.Ping(context.Background()); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	logger.Info("redis connected", zap.String("addr", cfg.Redis.Addr))
	return client, nil
}

func openK8s(cfg *config.Config, logger *zap.Logger) (*k8s.Factory, error) {
	if cfg.K8s.KubeconfigPath == "" || len(cfg.K8s.Clusters) == 0 {
		logger.Warn("kubernetes not configured, runtime will use mock adapters")
		return k8s.NewFactory(k8s.Config{})
	}
	clusters := make([]k8s.ClusterConfig, 0, len(cfg.K8s.Clusters))
	for _, c := range cfg.K8s.Clusters {
		clusters = append(clusters, k8s.ClusterConfig{
			Name:           c.Name,
			KubeconfigPath: cfg.K8s.KubeconfigPath,
			Context:        c.Context,
		})
	}
	factory, err := k8s.NewFactory(k8s.Config{
		DefaultCluster: cfg.K8s.DefaultCluster,
		Clusters:       clusters,
	})
	if err != nil {
		return nil, fmt.Errorf("create k8s factory: %w", err)
	}
	logger.Info("kubernetes factory ready", zap.Strings("clusters", factory.Names()))
	return factory, nil
}
