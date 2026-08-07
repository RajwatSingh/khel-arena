// Package postgres holds every line of SQL in the service and the pgx-backed
// repositories that run it. Nothing above this package imports pgx, so the
// service layer stays testable without a database and the storage engine
// stays replaceable.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RajwatSingh/khel-arena/internal/platform/config"
)

// Connect opens the pool and verifies the database is actually reachable.
//
// A pool, rather than the old stack's per-request HTTP round-trip to
// PostgREST: connections are established once and reused, and pgx caches
// prepared statements per connection, so a hot query costs one network
// round-trip on an already-open socket.
func Connect(ctx context.Context, cfg config.Database) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	// Only override what the caller actually set. A zero here means "no
	// opinion", not "expire immediately" -- pgx treats a zero lifetime as
	// instant expiry, which starves the pool and fails every acquire.
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}

	// Every timestamp this service reads or writes is absolute (timestamptz).
	// Pinning the session to UTC keeps server-local settings from silently
	// reinterpreting them; Kathmandu wall-clock conversion is explicit at the
	// few places that need it.
	poolCfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	poolCfg.ConnConfig.RuntimeParams["application_name"] = "khel-arena"

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// DB is the subset of pgxpool.Pool the repositories need. Accepting this
// interface rather than the concrete pool lets a repository method run either
// on the pool or inside a transaction without knowing which.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// InTx runs fn inside a transaction, committing if it returns nil and rolling
// back otherwise. A panic also rolls back before it continues unwinding.
//
// Every multi-statement invariant in this service goes through here, so no
// caller has to remember the rollback path.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	return inTx(ctx, pool, pgx.TxOptions{}, fn)
}

// InRepeatableReadTx runs fn at REPEATABLE READ, for read-modify-write
// sequences whose correctness depends on a stable snapshot. Callers must be
// prepared for a serialization failure and retry.
func InRepeatableReadTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	return inTx(ctx, pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead}, fn)
}

func inTx(ctx context.Context, pool *pgxpool.Pool, opts pgx.TxOptions, fn func(tx pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		// Use a cancellation-free context: if fn failed because ctx was
		// cancelled, the rollback still needs to reach the server.
		_ = tx.Rollback(context.WithoutCancel(ctx))
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	committed = true
	return nil
}

// ---------------------------------------------------------------------------
// Error classification
//
// The repositories translate pgx errors into domain errors, so no caller ever
// switches on a Postgres SQLSTATE. These helpers name the ones that carry
// business meaning.
// ---------------------------------------------------------------------------

const (
	sqlStateUniqueViolation      = "23505"
	sqlStateExclusionViolation   = "23P01"
	sqlStateForeignKeyViolation  = "23503"
	sqlStateCheckViolation       = "23514"
	sqlStateSerializationFailure = "40001"
	sqlStateDeadlockDetected     = "40P01"
)

func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// pgErrConstraint reports which named constraint an error violated, so
// callers can distinguish "duplicate email" from "duplicate username".
func pgErrConstraint(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}

func isUniqueViolation(err error) bool    { return pgErrCode(err) == sqlStateUniqueViolation }
func isExclusionViolation(err error) bool { return pgErrCode(err) == sqlStateExclusionViolation }
func isForeignKeyViolation(err error) bool {
	return pgErrCode(err) == sqlStateForeignKeyViolation
}
func isCheckViolation(err error) bool { return pgErrCode(err) == sqlStateCheckViolation }

// IsRetryable reports whether a failed transaction can simply be tried again.
// Serialization failures and deadlocks are the database asking for a retry,
// not a defect.
func IsRetryable(err error) bool {
	switch pgErrCode(err) {
	case sqlStateSerializationFailure, sqlStateDeadlockDetected:
		return true
	default:
		return false
	}
}

func noRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
