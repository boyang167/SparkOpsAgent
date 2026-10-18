package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/sparkops/sparkops-agent/backend/internal/bootstrap"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func main() {
	ifUnversioned := flag.Bool(
		"if-unversioned",
		false,
		"run the historical GORM bootstrap only when schema_migrations is absent",
	)
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

	if *ifUnversioned {
		var migrationsTable *string
		if err := db.Raw("SELECT to_regclass('public.schema_migrations')").Scan(&migrationsTable).Error; err != nil {
			log.Fatal(err)
		}
		if migrationsTable != nil {
			fmt.Println("=== schema bootstrap skipped: versioned database ===")
			return
		}
	}

	if err := bootstrap.AutoMigrate(db); err != nil {
		log.Fatal(err)
	}

	type row struct {
		Schema string `gorm:"column:table_schema"`
		Table  string `gorm:"column:table_name"`
	}
	var rows []row
	if err := db.Raw(`
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY 1, 2
	`).Scan(&rows).Error; err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== schema migrated ===")
	for _, r := range rows {
		fmt.Printf("  %s.%s\n", r.Schema, r.Table)
	}
	fmt.Printf("total: %d tables\n", len(rows))
}
