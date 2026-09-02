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
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/api"
	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/config"
	"github.com/RajwatSingh/khel-arena/internal/platform/mail"
	"github.com/RajwatSingh/khel-arena/internal/platform/media"
	"github.com/RajwatSingh/khel-arena/internal/platform/payment"
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
	arenas := postgres.NewArenaRepo(pool)
	payments := postgres.NewPaymentRepo(pool, bookings)

	// ── services ─────────────────────────────────────────────────────────
	issuer, err := token.NewIssuer(cfg.Auth.JWTSecret, cfg.Auth.Issuer, cfg.Auth.AccessTokenTTL)
	if err != nil {
		return err
	}

	clock := service.SystemClock{}
	authService := service.NewAuthService(users, sessions, issuer, clock, cfg.Auth.RefreshTokenTTL)
	bookingService := service.NewBookingService(bookings, clock, cfg.Booking.Timezone, cfg.Booking.HoldWindow)
	profileService := service.NewProfileService(users)
	arenaService := service.NewArenaService(arenas, bookings, clock, cfg.Booking.Timezone)

	gateways := buildGateways(cfg)
	paymentService := service.NewPaymentService(payments, bookings, gateways,
		returnURLs(cfg.HTTPAddr, cfg.AppURL), clock)

	notifier := service.NewNotifier(buildMailSender(cfg), cfg.AppURL, slog.Default())
	ownerService := service.NewOwnerService(arenas, payments, users, clock)
	teamService := service.NewTeamService(postgres.NewTeamRepo(pool))
	callRepo := postgres.NewMatchmakingRepo(pool)
	callService := service.NewMatchmakingService(callRepo, bookings, clock)
	teamRepo := postgres.NewTeamRepo(pool)
	tournamentService := service.NewTournamentService(postgres.NewTournamentRepo(pool), teamRepo, clock)
	matchService := service.NewMatchService(postgres.NewMatchRepo(pool), teamRepo, clock)
	reviewService := service.NewReviewService(postgres.NewReviewRepo(pool))

	// Uploads are optional: with no MEDIA_DIR, galleries still take a URL you
	// host elsewhere and the upload endpoint reports itself unavailable rather
	// than half-working.
	var mediaStore *media.DiskStore
	if cfg.Media.Configured() {
		mediaStore, err = media.NewDiskStore(cfg.Media.Dir, cfg.Media.Prefix)
		if err != nil {
			return err
		}
		slog.Info("uploads configured", "dir", cfg.Media.Dir, "served_at", mediaStore.Prefix())
	} else {
		slog.Warn("MEDIA_DIR is unset; photo uploads are disabled (linked images still work)")
	}
	janitor := service.NewJanitor(bookings, sessions, callRepo, janitorInterval, slog.Default())

	// ── transport ────────────────────────────────────────────────────────
	opts := api.Options{
		Auth:           authService,
		Bookings:       bookingService,
		Profiles:       profileService,
		Arenas:         arenaService,
		Payments:       paymentService,
		Owner:          ownerService,
		Teams:          teamService,
		Calls:          callService,
		Tournaments:    tournamentService,
		Matches:        matchService,
		Reviews:        reviewService,
		Mailer:         notifier,
		Pinger:         pool,
		AppURL:         cfg.AppURL,
		AllowedOrigins: cfg.AllowedOrigins,
		// No TLS on http://localhost, so a Secure cookie would simply never
		// be stored there. Everywhere else it must be set, or the refresh
		// token can travel in clear text.
		SecureCookies: cfg.IsProduction(),
		// Password reset has no mail sender yet, so outside production the
		// token is logged to make the flow testable. The gate is what makes
		// that impossible to reach in production rather than merely unlikely.
		LogResetTokens: !cfg.IsProduction(),
	}

	// Assigned only when there is one. A nil *DiskStore put into an interface
	// field is not a nil interface -- it is a non-nil interface holding a nil
	// pointer -- so `s.media == nil` in the handler would be false and the
	// first call would dereference nothing. This is the one place that can go
	// wrong, so it is the one place it is guarded.
	if mediaStore != nil {
		opts.Media = mediaStore
	}

	srv := api.NewServer(opts)

	// Uploaded files are served alongside the API rather than through it: the
	// API's middleware chain -- auth, rate limiting, JSON error envelopes --
	// has nothing to say about a static image.
	handler := srv.Handler()
	if mediaStore != nil {
		mux := http.NewServeMux()
		mux.Handle(mediaStore.Prefix()+"/", mediaStore.Handler())
		mux.Handle("/", handler)
		handler = mux
	}

	httpServer := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handler,
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

// buildGateways assembles the payment adapters that are actually configured.
//
// A provider with no credentials is simply absent from the registry, and
// /v1/payments/providers reports only what is there -- so a deployment that
// takes only cash offers only cash, rather than offering a gateway that fails
// at the moment somebody tries to use it.
func buildGateways(cfg config.Config) payment.Registry {
	registry := payment.Registry{
		// Always available: settling at the arena needs no credentials. It
		// refuses to verify anything by itself, which is the honest state of
		// affairs until owner-facing reconciliation exists.
		domain.ProviderCash: payment.NewCash(),
	}

	if cfg.Payment.EsewaConfigured() {
		registry[domain.ProviderEsewa] = payment.NewEsewa(
			cfg.Payment.EsewaSecretKey, cfg.Payment.EsewaProductCode,
			cfg.Payment.EsewaFormURL, cfg.Payment.EsewaStatusURL)
		slog.Info("payment gateway configured", "provider", domain.ProviderEsewa)
	}
	if cfg.Payment.KhaltiConfigured() {
		registry[domain.ProviderKhalti] = payment.NewKhalti(
			cfg.Payment.KhaltiSecretKey, cfg.Payment.KhaltiBaseURL, cfg.AppURL)
		slog.Info("payment gateway configured", "provider", domain.ProviderKhalti)
	}

	return registry
}

// returnURLs builds where a gateway sends the player back to.
//
// Back to this API rather than straight to the interface: the callback has to
// be handled server-side (the gateway is asked what happened, and the booking
// is confirmed) before anybody is shown an outcome. The handler redirects on
// to the interface afterwards.
func returnURLs(httpAddr, appURL string) func(domain.PaymentProvider) payment.ReturnURLs {
	// The API's own public origin. In every deployment this service is
	// reachable on the same origin as the interface -- that is what the
	// refresh cookie already depends on -- so the interface's URL is the API's
	// too, and the /v1 prefix reaches these handlers.
	base := strings.TrimRight(appURL, "/") + "/v1/payments"

	return func(provider domain.PaymentProvider) payment.ReturnURLs {
		callback := fmt.Sprintf("%s/%s/callback", base, provider)
		// Both point at the same handler. A gateway's "failure" redirect is
		// not evidence of failure any more than its success one is evidence of
		// success: either way we ask the gateway what actually happened.
		return payment.ReturnURLs{Success: callback, Failure: callback}
	}
}

// buildMailSender picks how transactional email leaves the process.
//
// Production is required by config.Load to name an SMTP host, so the logging
// sender cannot be reached there: a deployment where the only route back into
// a locked-out account writes the link to a log file has no recovery path at
// all.
func buildMailSender(cfg config.Config) mail.Sender {
	if cfg.Mail.Configured() {
		slog.Info("mail configured", "host", cfg.Mail.SMTPHost, "from", cfg.Mail.From)
		return mail.SMTPSender{
			Host:     cfg.Mail.SMTPHost,
			Port:     cfg.Mail.SMTPPort,
			Username: cfg.Mail.Username,
			Password: cfg.Mail.Password,
			From:     cfg.Mail.From,
			FromName: cfg.Mail.FromName,
		}
	}

	slog.Warn("no SMTP host configured; email will be logged instead of sent")
	return mail.LogSender{Log: slog.Default()}
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
