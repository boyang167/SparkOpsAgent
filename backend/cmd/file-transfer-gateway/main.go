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

	"github.com/sparkops/sparkops-agent/backend/internal/filesystemgateway"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	engine, err := filesystemgateway.NewEngineFromConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			log.Printf("close transfer gateway storage: %v", err)
		}
	}()

	addr := cfg.Filesystem.TransferGateway.Listen
	server := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("file transfer gateway listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
