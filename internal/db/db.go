package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// migrationsFS embeds the goose migration files so the binary is self-contained
// and can migrate itself on startup (docs/architecture.md).
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// driver is the database/sql driver name registered by modernc.org/sqlite. It
// is pure Go, so the project builds with CGO_ENABLED=0 (Makefile `build`).
const driver = "sqlite"

// Open opens the SQLite database at path and configures the durability PRAGMAs
// from ADR-0010:
//
//	journal_mode = WAL       one writer, concurrent readers
//	foreign_keys = ON        enforce the schema FKs
//	busy_timeout = 5000      wait instead of failing on a locked writer
//	synchronous = NORMAL     durable enough with WAL, faster than FULL
//
// PRAGMAs are set via the DSN so every pooled connection gets them, not just
// the first one. The pool is pinned to a single connection: this app is a
// single-instance SQLite deployment (ADR-0015) and SQLite serialises writes,
// so one connection keeps transactions deterministic and avoids SQLITE_BUSY.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("db: create data dir: %w", err)
		}
	}

	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)",
		path,
	)

	sqldb, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", path, err)
	}

	sqldb.SetMaxOpenConns(1)
	sqldb.SetMaxIdleConns(1)

	if err := sqldb.Ping(); err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("db: ping %s: %w", path, err)
	}

	return sqldb, nil
}

// Migrate applies every pending goose migration embedded in the binary. It is
// idempotent: goose tracks applied versions in goose_db_version.
func Migrate(sqldb *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("db: set goose dialect: %w", err)
	}
	if err := goose.Up(sqldb, "migrations"); err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}
	return nil
}
