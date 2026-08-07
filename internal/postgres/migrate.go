package postgres

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migrateLockKey is an arbitrary constant passed to pg_advisory_lock so that
// two processes starting at once cannot apply the same migration twice.
const migrateLockKey int64 = 8617234501

// Migration is one versioned schema change, read from the embedded FS.
type Migration struct {
	Version  int
	Name     string
	SQL      string
	Checksum string
}

// loadMigrations reads and orders every embedded migration. Filenames must be
// `NNNN_name.sql`; the numeric prefix is the version and defines apply order.
func loadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	seen := make(map[int]string, len(entries))

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		prefix, rest, ok := strings.Cut(strings.TrimSuffix(entry.Name(), ".sql"), "_")
		if !ok {
			return nil, fmt.Errorf("migration %q: want NNNN_name.sql", entry.Name())
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migration %q: bad version prefix: %w", entry.Name(), err)
		}
		if dup, exists := seen[version]; exists {
			return nil, fmt.Errorf("migrations %q and %q share version %d", dup, entry.Name(), version)
		}
		seen[version] = entry.Name()

		body, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", entry.Name(), err)
		}

		sum := sha256.Sum256(body)
		migrations = append(migrations, Migration{
			Version:  version,
			Name:     rest,
			SQL:      string(body),
			Checksum: hex.EncodeToString(sum[:]),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

// Migrate applies every migration that has not yet run, in version order.
//
// Each migration runs inside its own transaction, so a failure leaves the
// database at the last good version rather than half-way through a file. A
// session-level advisory lock serialises concurrent migrators; the loser
// waits, then finds nothing left to do.
//
// Already-applied migrations are checksummed against the embedded copy. An
// edited migration is a deployment error -- the database cannot be brought to
// the state the code now expects -- so it is reported rather than skipped.
func Migrate(ctx context.Context, pool *pgxpool.Pool) ([]Migration, error) {
	migrations, err := loadMigrations()
	if err != nil {
		return nil, err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `select pg_advisory_lock($1)`, migrateLockKey); err != nil {
		return nil, fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		// Best effort: releasing the lock on a broken connection is moot,
		// since Postgres drops session locks when the session ends.
		_, _ = conn.Exec(context.WithoutCancel(ctx), `select pg_advisory_unlock($1)`, migrateLockKey)
	}()

	const createLedger = `
		create table if not exists schema_migrations (
			version    int  primary key,
			name       text not null,
			checksum   text not null,
			applied_at timestamptz not null default now()
		)`
	if _, err := conn.Exec(ctx, createLedger); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}

	rows, err := conn.Query(ctx, `select version, checksum from schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	applied := make(map[int]string)
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[version] = checksum
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}

	var ran []Migration
	for _, m := range migrations {
		if existing, ok := applied[m.Version]; ok {
			if existing != m.Checksum {
				return ran, fmt.Errorf(
					"migration %04d_%s was modified after it was applied "+
						"(recorded %s, embedded %s); write a new migration instead",
					m.Version, m.Name, existing[:12], m.Checksum[:12])
			}
			continue
		}

		if err := applyOne(ctx, conn.Conn(), m); err != nil {
			return ran, err
		}
		ran = append(ran, m)
	}

	return ran, nil
}

func applyOne(ctx context.Context, conn *pgx.Conn, m Migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin %04d_%s: %w", m.Version, m.Name, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if _, err := tx.Exec(ctx, m.SQL); err != nil {
		return fmt.Errorf("apply %04d_%s: %w", m.Version, m.Name, err)
	}

	const record = `insert into schema_migrations (version, name, checksum) values ($1, $2, $3)`
	if _, err := tx.Exec(ctx, record, m.Version, m.Name, m.Checksum); err != nil {
		return fmt.Errorf("record %04d_%s: %w", m.Version, m.Name, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %04d_%s: %w", m.Version, m.Name, err)
	}
	return nil
}
