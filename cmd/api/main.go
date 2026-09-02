// Command api serves the Khel Arena HTTP API.
//
// Run it after cmd/migrate has brought the schema up to date:
//
//	DATABASE_URL=postgres://... JWT_SECRET=... go run ./cmd/api
//
// This file is the only place the layers are wired to each other. Everything
// below it -- domain, service, postgres, api -- takes its dependencies as
// arguments and knows nothing about how they were built.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/api"
	"github.com/RajwatSingh/khel-arena/internal/platform/config"
	"github.com/RajwatSingh/khel-arena/internal/platform/token"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
	"github.com/RajwatSingh/khel-arena/internal/service"
)

const (
	// shutdownTimeout is how long in-flight requests get to finish once a
	// shutdown signal arrives. Comfortably longer than the API's own
	// per-request timeout, so a request that was going to succeed still can.
	shutdownTimeout = 20 * time.Second

	// janitorInterval is how often abandoned holds and spent tokens are swept.
	janitorInterval = time.Minute

	// startupTimeout bounds the work between "process started" and "listening".
	startupTimeout = 30 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("api failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// The signal context is the whole shutdown mechanism: SIGINT or SIGTERM
	// cancels it, everything below waits on it. Same shape as cmd/migrate.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Set up logging before anything else can want to log. JSON in
	// production, where something machine-readable ingests it; text in
	// development, where a person reads it in a terminal.
	slog.SetDefault(newLogger(cfg))

	// ── storage ──────────────────────────────────────────────────────────
	connectCtx, cancelConnect := context.WithTimeout(ctx, startupTimeout)
	defer cancelConnect()

	pool, err := postgres.Connect(connectCtx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	users := postgres.NewUserRepo(pool)
	sessions := postgres.NewSessionRepo(pool)
	bookings := postgres.NewBookingRepo(pool)

	// ── services ─────────────────────────────────────────────────────────
	issuer, err := token.NewIssuer(cfg.Auth.JWTSecret, cfg.Auth.Issuer, cfg.Auth.AccessTokenTTL)
	if err != nil {
		return err
	}

	clock := service.SystemClock{}
	authService := service.NewAuthService(users, sessions, issuer, clock, cfg.Auth.RefreshTokenTTL)
	bookingService := service.NewBookingService(bookings, clock, cfg.Booking.Timezone, cfg.Booking.HoldWindow)
	profileService := service.NewProfileService(users)
	janitor := service.NewJanitor(bookings, sessions, janitorInterval, slog.Default())

	// ── transport ────────────────────────────────────────────────────────
	srv := api.NewServer(api.Options{
		Auth:           authService,
		Bookings:       bookingService,
		Profiles:       profileService,
		Pinger:         pool,
		AllowedOrigins: cfg.AllowedOrigins,
		// No TLS on http://localhost, so a Secure cookie would simply never
		// be stored there. Everywhere else it must be set, or the refresh
		// token can travel in clear text.
		SecureCookies: cfg.IsProduction(),
		// Password reset has no mail sender yet, so outside production the
		// token is logged to make the flow testable. The gate is what makes
		// that impossible to reach in production rather than merely unlikely.
		LogResetTokens: !cfg.IsProduction(),
	})

	httpServer := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: srv.Handler(),
		// Without ReadHeaderTimeout a client can hold a connection open
		// forever by sending headers one byte at a time -- the Slowloris
		// attack. This is the field that closes it.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// Longer than the API's own request timeout, so the timeout handler
		// is what answers a slow request rather than the connection being cut
		// out from under it mid-response.
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		ErrorLog:     slog.NewLogLogger(slog.Default().Handler(), slog.LevelWarn),
	}

	// ── run ──────────────────────────────────────────────────────────────
	// The janitor gets its own context so it can be stopped independently of,
	// and after, the HTTP server.
	janitorCtx, stopJanitor := context.WithCancel(context.Background())
	janitorDone := make(chan struct{})
	go func() {
		defer close(janitorDone)
		janitor.Run(janitorCtx)
	}()

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		// ErrServerClosed is what Shutdown produces, not a failure.
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		stopJanitor()
		<-janitorDone
		return err
	case <-ctx.Done():
	}

	// ── shutdown, in this order and no other ─────────────────────────────
	//
	// Drain HTTP first, then stop the janitor, then close the pool. Closing
	// the pool while a request is still being served fails that request's
	// query and hands the client a 500 instead of the response it was about
	// to get -- the opposite of graceful. The pool close is the deferred one
	// above, which runs last because it was deferred first.
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		// The drain window expired with requests still running. Close what is
		// left rather than hanging, and report it: repeated appearances mean
		// something is holding connections longer than the deadline allows.
		slog.Error("graceful shutdown timed out; closing connections", "error", err)
		_ = httpServer.Close()
	}

	stopJanitor()
	<-janitorDone

	slog.Info("stopped")
	return nil
}

// newLogger builds the process-wide logger. Every layer logs through
// slog.Default(), so this is the one place the format and level are decided.
func newLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelDebug
	if cfg.IsProduction() {
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
