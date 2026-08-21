package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/redis"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres open: %v\n", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres db: %v\n", err)
		os.Exit(1)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "postgres ping: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("postgres: OK")

	if cfg.Redis.Addr != "" {
		client, err := redis.NewClient(redis.Config{Addr: cfg.Redis.Addr})
		if err != nil {
			fmt.Fprintf(os.Stderr, "redis client: %v\n", err)
			os.Exit(1)
		}
		if err := client.Ping(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "redis ping: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("redis: OK")
	}
}
