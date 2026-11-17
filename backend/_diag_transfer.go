package main

import (
	"fmt"
	"log"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil { log.Fatal(err) }
	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil { log.Fatal(err) }
	var versions []struct{ Version int; Dirty bool }
	db.Raw("SELECT version, dirty FROM public.schema_migrations ORDER BY version DESC LIMIT 5").Scan(&versions)
	fmt.Println("migrations:", versions)
	var indexes []struct{ Name, Def string }
	db.Raw("SELECT indexname, indexdef FROM pg_indexes WHERE schemaname = 'filesystem' AND indexname LIKE '%nodes%name%'").Scan(&indexes)
	for _, i := range indexes { fmt.Println(i.Name, i.Def) }
	var sessions []struct {
		ID, Status, TargetName, ErrorCode string
		NamespaceID string
	}
	db.Raw(`SELECT id::text, status, target_name, COALESCE(error_code,''), namespace_id::text
		FROM filesystem.transfer_sessions
		WHERE target_name ILIKE '%kubeconfig%'
		ORDER BY updated_at DESC LIMIT 5`).Scan(&sessions)
	fmt.Println("sessions:", sessions)
	var nodes []struct {
		ID, Name, State string
		ParentID *string
	}
	db.Raw(`SELECT id::text, name, state, parent_id::text
		FROM filesystem.nodes
		WHERE normalized_name = 'kubeconfig.yaml'
		ORDER BY updated_at DESC LIMIT 10`).Scan(&nodes)
	fmt.Println("nodes:", nodes)
}
