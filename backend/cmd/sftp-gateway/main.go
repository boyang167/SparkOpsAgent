package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/sftpgateway"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if !cfg.Filesystem.SFTP.Enabled {
		log.Printf("sftp gateway disabled (filesystem.sftp.enabled=false)")
		return
	}

	server, err := sftpgateway.NewFromConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("sftp gateway listening on %s", cfg.Filesystem.SFTP.Listen)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, context.Canceled) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	if err := server.Shutdown(context.Background()); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("sftp gateway stopped")
}
