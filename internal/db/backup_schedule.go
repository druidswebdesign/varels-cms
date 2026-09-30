package db

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// StartBackupSchedule writes VACUUM INTO snapshots into dir on a fixed
// interval and prunes old snapshots, keeping the newest keep. It takes one
// snapshot immediately, then one per interval until ctx is cancelled. An empty
// dir or a non-positive interval disables scheduling.
func StartBackupSchedule(ctx context.Context, sqldb *sql.DB, dir string, interval time.Duration, keep int) {
	if dir == "" || interval <= 0 {
		log.Printf("backup: schedule disabled")
		return
	}
	go func() {
		runBackup(sqldb, dir, keep)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runBackup(sqldb, dir, keep)
			}
		}
	}()
}

// runBackup writes one timestamped snapshot and prunes older ones. Failures are
// logged, never fatal: the live server must keep serving if a backup fails.
func runBackup(sqldb *sql.DB, dir string, keep int) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	dest := BackupName(dir, time.Now().UTC())
	if err := Backup(ctx, sqldb, dest); err != nil {
		log.Printf("backup: %v", err)
		return
	}
	log.Printf("backup: wrote %s", dest)

	if err := PruneBackups(dir, keep); err != nil {
		log.Printf("backup: prune: %v", err)
	}
}

// BackupName returns the snapshot path for a timestamp. The timestamp sorts
// lexically, which PruneBackups relies on.
func BackupName(dir string, at time.Time) string {
	return filepath.Join(dir, "app-"+at.UTC().Format("20060102T150405Z")+".db")
}

// PruneBackups deletes the oldest app-*.db snapshots in dir, keeping the newest
// keep. Other files in dir are left untouched.
func PruneBackups(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "app-") && strings.HasSuffix(name, ".db") {
			names = append(names, name)
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(names))) // newest first
	if keep < 0 {
		keep = 0
	}
	for _, name := range names[min(keep, len(names)):] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}
