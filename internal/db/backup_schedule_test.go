package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupNameSortsChronologically(t *testing.T) {
	dir := t.TempDir()
	older := BackupName(dir, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	newer := BackupName(dir, time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC))

	if filepath.Base(older) != "app-20260102T030405Z.db" {
		t.Errorf("backup name = %q", filepath.Base(older))
	}
	if older >= newer {
		t.Errorf("older %q does not sort before newer %q", older, newer)
	}
}

func TestPruneBackupsKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		if err := os.WriteFile(BackupName(dir, base.Add(time.Duration(i)*time.Hour)), nil, 0o600); err != nil {
			t.Fatalf("write snapshot: %v", err)
		}
	}
	// An unrelated file must survive pruning.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("keep me"), 0o600); err != nil {
		t.Fatalf("write notes: %v", err)
	}

	if err := PruneBackups(dir, 2); err != nil {
		t.Fatalf("PruneBackups: %v", err)
	}

	remaining, err := filepath.Glob(filepath.Join(dir, "app-*.db"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("remaining snapshots = %d, want 2 (%v)", len(remaining), remaining)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Errorf("unrelated file was removed: %v", err)
	}
}

func TestPruneBackupsZeroKeepsNone(t *testing.T) {
	dir := t.TempDir()
	name := BackupName(dir, time.Now())
	if err := os.WriteFile(name, nil, 0o600); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	if err := PruneBackups(dir, 0); err != nil {
		t.Fatalf("PruneBackups: %v", err)
	}
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Errorf("snapshot still present after keep=0")
	}
}

func TestRunBackupWritesAndPrunes(t *testing.T) {
	_, db, _ := openTestDB(t)
	dir := t.TempDir()

	for i := 0; i < 3; i++ {
		runBackup(db, dir, 1)
		time.Sleep(1100 * time.Millisecond) // distinct second-granularity names
	}

	remaining, err := filepath.Glob(filepath.Join(dir, "app-*.db"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("remaining snapshots = %d, want 1 (%v)", len(remaining), remaining)
	}
	result, err := RestoreDrill(t.Context(), remaining[0])
	if err != nil {
		t.Fatalf("RestoreDrill: %v", err)
	}
	if result != "ok" {
		t.Errorf("integrity_check = %q, want ok", result)
	}
}
