package db

import (
	"testing"
	"time"

	"github.com/druidswebdesign/varels-cms/internal/db/sqlc"
)

func TestDeleteExpiredSessions(t *testing.T) {
	ctx, db, _ := openTestDB(t)
	q := sqlc.New(db)

	now := time.Now().UTC()
	insert := func(token string, expires time.Time) {
		t.Helper()
		if _, err := db.Exec(
			"INSERT INTO sessions (token, data, expires_at) VALUES (?, ?, ?)",
			token, []byte("x"), expires.Format(time.RFC3339)); err != nil {
			t.Fatalf("insert session: %v", err)
		}
	}

	insert("expired", now.Add(-time.Hour))
	insert("expiring-soon", now.Add(time.Hour))
	insert("future", now.Add(24*time.Hour))

	if err := q.DeleteExpiredSessions(ctx, now.Format(time.RFC3339)); err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}

	remaining := map[string]bool{}
	rows, err := db.Query("SELECT token FROM sessions")
	if err != nil {
		t.Fatalf("query sessions: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			t.Fatalf("scan session: %v", err)
		}
		remaining[token] = true
	}

	if remaining["expired"] {
		t.Error("expired session was not deleted")
	}
	if !remaining["expiring-soon"] || !remaining["future"] {
		t.Errorf("unexpired sessions were wrongly deleted: %v", remaining)
	}
}
