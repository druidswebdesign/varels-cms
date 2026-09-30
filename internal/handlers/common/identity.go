package common

import (
	"database/sql"
	"net"
	"net/http"

	"github.com/druidswebdesign/varels-cms/internal/auth"
)

// CurrentUserID returns the authenticated user id for audit/ownership columns.
func CurrentUserID(r *http.Request) sql.NullInt64 {
	if u, ok := auth.UserFromContext(r.Context()); ok {
		return sql.NullInt64{Int64: u.ID, Valid: true}
	}
	return sql.NullInt64{}
}

// ClientIP returns the direct peer address. X-Forwarded-For is not trusted
// because no reverse proxy is configured (single-instance deployment).
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
