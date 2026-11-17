package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
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
	var versions []struct {
		Version int
		Dirty   bool
	}
	if err := db.Raw("SELECT version, dirty FROM public.schema_migrations ORDER BY version DESC LIMIT 5").Scan(&versions).Error; err != nil {
		log.Fatal(err)
	}
	fmt.Println("migrations:", versions)
	var indexes []struct {
		Name string
		Def  string
	}
	if err := db.Raw("SELECT indexname, indexdef FROM pg_indexes WHERE schemaname = 'filesystem' AND indexname LIKE '%nodes%name%'").Scan(&indexes).Error; err != nil {
		log.Fatal(err)
	}
	for _, index := range indexes {
		fmt.Println(index.Name, index.Def)
	}
	var sessions []struct {
		ID          string
		Status      string
		TargetName  string
		ErrorCode   string
		NamespaceID string
	}
	if err := db.Raw(`SELECT id::text, status, target_name, COALESCE(error_code,''), namespace_id::text
		FROM filesystem.transfer_sessions
		WHERE target_name ILIKE '%kubeconfig%'
		ORDER BY updated_at DESC LIMIT 5`).Scan(&sessions).Error; err != nil {
		log.Fatal(err)
	}
	fmt.Println("sessions:", sessions)
	var nodes []struct {
		ID       string
		Name     string
		State    string
		ParentID *string
	}
	if err := db.Raw(`SELECT id::text, name, state, parent_id::text
		FROM filesystem.nodes
		WHERE normalized_name = 'kubeconfig.yaml'
		ORDER BY updated_at DESC LIMIT 10`).Scan(&nodes).Error; err != nil {
		log.Fatal(err)
	}
	fmt.Println("nodes:", nodes)
	var active []struct {
		ID, Name, State string
		ParentID        *string
		NamespaceID     string
	}
	if err := db.Raw(`SELECT id::text, name, state, parent_id::text, namespace_id::text
		FROM filesystem.nodes
		WHERE namespace_id = '60997190-f7de-54b5-9177-3eb1d29b8852'
		AND parent_id = (SELECT id FROM filesystem.nodes WHERE namespace_id = '60997190-f7de-54b5-9177-3eb1d29b8852' AND normalized_name = 'test' AND state = 'ACTIVE' LIMIT 1)
		ORDER BY normalized_name`).Scan(&active).Error; err != nil {
		log.Fatal(err)
	}
	fmt.Println("children in test:", active)
	sid := uuid.MustParse("d9618a8f-4eca-54b3-a6cc-27048cbf9679")
	nodeID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-transfer-node:"+sid.String()))
	fmt.Println("expected node id:", nodeID)
	var detail []struct {
		Status, ErrorCode string
		Version           int64
		ActualSize        *int64
		ActualSHA         *string
	}
	if err := db.Raw(`SELECT status, COALESCE(error_code,''), version, actual_size, actual_sha256, updated_at
		FROM filesystem.transfer_sessions WHERE id = ?`, sid).Scan(&detail).Error; err != nil {
		log.Fatal(err)
	}
	fmt.Println("stuck session:", detail)
	var outbox []struct {
		ID, EventName, AggregateID, Status string
	}
	if err := db.Raw(`SELECT id::text, event_type, aggregate_id, status
		FROM platform.event_outbox
		WHERE aggregate_id IN ('d9618a8f-4eca-54b3-a6cc-27048cbf9679','acf3bf00-7342-5f09-8cdc-bfc74e36371b')
		   OR payload::text LIKE '%d9618a8f%'
		ORDER BY created_at DESC LIMIT 10`).Scan(&outbox).Error; err != nil {
		fmt.Println("outbox query:", err)
	} else {
		fmt.Println("outbox:", outbox)
	}
	var inbox []struct {
		EventID, Subscriber string
		ProcessedAt         *string
	}
	if err := db.Raw(`SELECT event_id, subscriber, processed_at::text
		FROM platform.event_inbox
		WHERE subscriber LIKE '%transfer%'
		ORDER BY processed_at DESC NULLS LAST
		LIMIT 10`).Scan(&inbox).Error; err != nil {
		fmt.Println("inbox query:", err)
	} else {
		fmt.Println("inbox:", inbox)
	}
}