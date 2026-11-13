package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/service"
	fspersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		log.Fatal(err)
	}
	if err := fspersistence.AutoMigrate(db); err != nil {
		log.Fatal(err)
	}
	lease, err := service.NewDBLease(db)
	if err != nil {
		log.Fatal(err)
	}
	repo := fspersistence.NewRepository(db)
	work := service.NewRepositoryWorkRunner(db, repo)
	owner := reconcilerOwner()
	reconcilerCfg := service.ReconcilerConfigFrom(cfg.Filesystem.Reconciler)
	reconciler := service.NewReconciler(owner, lease, work, reconcilerCfg)
	reconciler.SetErrorReporter(func(err error) { log.Printf("file-reconciler: %v", err) })

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf(
		"file-reconciler %s starting; lease=%s interval=%s batch=%d",
		owner, service.ReconcilerLeaseName, reconcilerCfg.Interval, reconcilerCfg.BatchSize,
	)
	<-reconciler.Start(ctx)
	log.Printf("file-reconciler %s stopped", owner)
}

func reconcilerOwner() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return fmt.Sprintf("%s:%d", host, os.Getpid())
}
