package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	dbpkg "github.com/yourname/varels_cms/internal/db"
)

// HandleHealth reports 200 when the database is reachable.
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.DB.PingContext(r.Context()); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

// HandleBackup writes a consistent copy of the database using VACUUM INTO.
func (s *Server) HandleBackup(w http.ResponseWriter, r *http.Request) {
	dest := filepath.Join(s.backupDir(), "app-"+time.Now().UTC().Format("20060102-150405")+".db")
	if err := dbpkg.Backup(r.Context(), s.DB, dest); err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "backup written to %s\n", dest)
}

// HandleRestoreDrill opens the newest backup read-only and runs
// PRAGMA integrity_check, proving the backup is usable without touching the
// live database (ADR-0010).
func (s *Server) HandleRestoreDrill(w http.ResponseWriter, r *http.Request) {
	path, err := latestBackup(s.backupDir())
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	result, err := dbpkg.RestoreDrill(r.Context(), path)
	if err != nil {
		serverError(w, err)
		return
	}
	if result != "ok" {
		http.Error(w, "integrity check failed: "+result, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "restore drill ok: %s\n", path)
}

func (s *Server) backupDir() string {
	return filepath.Join(filepath.Dir(s.DBPath), "backups")
}

// latestBackup returns the most recent .db file in dir.
func latestBackup(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("no backups found in %s", dir)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".db") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no backups found in %s", dir)
	}
	sort.Strings(names)
	return filepath.Join(dir, names[len(names)-1]), nil
}
