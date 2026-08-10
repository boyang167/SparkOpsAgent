package main

import (
	"fmt"
	"log"

	"github.com/sparkops/sparkops-agent/backend/internal/bootstrap"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
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
