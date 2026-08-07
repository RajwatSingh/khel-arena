// Command migrate applies pending database migrations and exits.
//
// Run it as a deploy step before starting the API:
//
//	DATABASE_URL=postgres://... go run ./cmd/migrate
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/platform/config"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	pool, err := postgres.Connect(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	start := time.Now()
	applied, err := postgres.Migrate(ctx, pool)
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		fmt.Println("schema is up to date; nothing to apply")
		return nil
	}
	for _, m := range applied {
		fmt.Printf("applied %04d_%s\n", m.Version, m.Name)
	}
	fmt.Printf("%d migration(s) in %s\n", len(applied), time.Since(start).Round(time.Millisecond))
	return nil
}
