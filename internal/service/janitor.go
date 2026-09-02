package service

import (
	"context"
	"log/slog"
	"time"
)

// Janitor releases abandoned booking holds and clears out spent tokens.
//
// Availability is already correct without it: an expired hold stops blocking
// its slot the moment its window lapses, because that is what
// `booking_blocks_slot` says. The janitor exists so the stored state catches
// up with that truth -- a player's own list should not show a booking they
// never completed, and an abandoned payment intent should not sit "initiated"
// forever.
//
// This replaces the pg_cron job the old system relied on, which meant
// correctness depended on an extension being enabled in a dashboard. Here it
// is part of the service: if the service is running, the sweeping is running.
type Janitor struct {
	bookings BookingStore
	tokens   TokenSweeper
	calls    CallSweeper
	interval time.Duration
	log      *slog.Logger
}

// TokenSweeper deletes refresh tokens nobody can use any more.
type TokenSweeper interface {
	DeleteExpiredTokens(ctx context.Context, retainRevokedFor time.Duration) (int64, error)
}

// CallSweeper closes matchmaking calls whose kickoff has passed. Same shape as
// the booking sweep: the feed already excludes them by time, so this is the
// stored state catching up rather than something correctness depends on.
type CallSweeper interface {
	ExpireStale(ctx context.Context) (int, error)
}

// retainRevokedTokensFor keeps revoked rows around after they are retired, so
// that reuse detection still has something to match when a stolen token is
// presented shortly after rotation.
const retainRevokedTokensFor = 7 * 24 * time.Hour

func NewJanitor(bookings BookingStore, tokens TokenSweeper, calls CallSweeper, interval time.Duration, log *slog.Logger) *Janitor {
	if interval <= 0 {
		interval = time.Minute
	}
	if log == nil {
		log = slog.Default()
	}
	return &Janitor{bookings: bookings, tokens: tokens, calls: calls, interval: interval, log: log}
}

// Run sweeps until the context is cancelled. It is meant to be started in its
// own goroutine and to return when the service shuts down.
//
// A failed sweep is logged and the loop continues: the next tick will try
// again, and nothing downstream depends on any single pass succeeding.
func (j *Janitor) Run(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	// Sweep once at startup, so a restart after downtime does not wait a full
	// interval to clear what accumulated while the service was down.
	j.sweep(ctx)

	for {
		select {
		case <-ctx.Done():
			j.log.Info("janitor stopping")
			return
		case <-ticker.C:
			j.sweep(ctx)
		}
	}
}

func (j *Janitor) sweep(ctx context.Context) {
	// Bound each pass so a slow query cannot stall every later tick.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if released, err := j.bookings.ReleaseStaleHolds(ctx); err != nil {
		j.log.Error("releasing stale booking holds", "error", err)
	} else if released > 0 {
		j.log.Info("released stale booking holds", "count", released)
	}

	if j.tokens != nil {
		if deleted, err := j.tokens.DeleteExpiredTokens(ctx, retainRevokedTokensFor); err != nil {
			j.log.Error("deleting expired refresh tokens", "error", err)
		} else if deleted > 0 {
			j.log.Info("deleted expired refresh tokens", "count", deleted)
		}
	}

	if j.calls != nil {
		if expired, err := j.calls.ExpireStale(ctx); err != nil {
			j.log.Error("expiring stale matchmaking calls", "error", err)
		} else if expired > 0 {
			j.log.Info("expired stale matchmaking calls", "count", expired)
		}
	}
}
