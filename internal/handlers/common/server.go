package common

import (
	"database/sql"

	"github.com/yourname/varels_cms/internal/auth"
	"github.com/yourname/varels_cms/internal/db/sqlc"
)

// Server holds the shared dependencies every handler needs. The sqlc queries
// are safe to reuse across requests; transaction-scoped sets are created with
// Q.WithTx inside multi-write handlers.
type Server struct {
	DB     *sql.DB
	Q      *sqlc.Queries
	DBPath string
	Env    string

	// Auth dependencies, wired in main after the DB is open.
	Sessions     *auth.Manager
	OAuthEnabled bool
	Limiter      *auth.LoginLimiter
}

// NewServer wires a handler set around an open database.
func NewServer(db *sql.DB, dbPath, env string) *Server {
	return &Server{
		DB:     db,
		Q:      sqlc.New(db),
		DBPath: dbPath,
		Env:    env,
	}
}
