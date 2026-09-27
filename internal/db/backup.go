package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

// Backup writes a consistent snapshot of the live database to destPath using
// VACUUM INTO (safe while the app is running, unlike copying the file).
func Backup(ctx context.Context, sqldb *sql.DB, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("db: create backup dir: %w", err)
	}
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("db: backup target already exists: %s", destPath)
	}
	if _, err := sqldb.ExecContext(ctx, "VACUUM INTO ?", destPath); err != nil {
		return fmt.Errorf("db: vacuum into %s: %w", destPath, err)
	}
	return nil
}

// RestoreDrill opens backupPath read-only and runs PRAGMA integrity_check,
// returning the raw result (expected "ok"). It never modifies the live DB.
func RestoreDrill(ctx context.Context, backupPath string) (string, error) {
	if _, err := os.Stat(backupPath); err != nil {
		return "", fmt.Errorf("db: backup not found: %s", backupPath)
	}

	sqldb, err := sql.Open(driver, "file:"+backupPath+"?_pragma=query_only(1)")
	if err != nil {
		return "", fmt.Errorf("db: open backup: %w", err)
	}
	defer sqldb.Close()

	var result string
	if err := sqldb.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return "", fmt.Errorf("db: integrity_check: %w", err)
	}
	return result, nil
}
