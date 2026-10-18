package main

import (
	"flag"
	"log"
	"os"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/dbmigration"
)

func main() {
	path := flag.String("path", envOrDefault("MIGRATIONS_PATH", "migrations"), "directory containing SQL migrations")
	minimumVersion := flag.Uint("min-version", 0, "ignore migration files below this version")
	target := flag.Uint("target", 0, "target migration version (zero means latest)")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_DSN")
	}
	if databaseURL == "" {
		log.Fatal("DATABASE_URL or DATABASE_DSN is required")
	}
	if err := dbmigration.RunFrom(databaseURL, *path, *minimumVersion, *target); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
