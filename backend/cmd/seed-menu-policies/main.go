package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	govcommand "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	govpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func main() {
	force := flag.Bool("force", false, "overwrite existing role_menus bindings")
	flag.Parse()

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
	if err := govpersistence.AutoMigrate(db); err != nil {
		log.Fatal(err)
	}
	menuStore := govpersistence.NewMenuStore(db)
	if err := menuStore.AutoMigrate(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	opts := govcommand.ApplyMenuRolePoliciesOptions{SkipIfConfigured: !*force}
	if err := govcommand.EnsureMenuRolePolicies(ctx, db, opts); err != nil {
		log.Fatal(err)
	}

	mode := "skip existing bindings"
	if *force {
		mode = "overwrite all bindings"
	}
	fmt.Printf("Menu role policies applied (%s) for %d paths.\n", mode, len(govcommand.DefaultMenuRolePolicies))
	fmt.Fprintln(os.Stdout, "Restart the server or re-login to refresh navigation.")
}
