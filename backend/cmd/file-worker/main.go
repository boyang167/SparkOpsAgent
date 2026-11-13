package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/subscriber"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf(
		"file-worker listening config at %s; mount %s via messaging bootstrap",
		cfg.Server.Addr,
		subscriber.TransferVerifyCommitSubscriber,
	)
	<-ctx.Done()
}
