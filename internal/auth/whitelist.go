package auth

import (
	"context"

	"github.com/druidswebdesign/varels-cms/internal/db/sqlc"
)

// Whitelist policy (docs/auth.md, ADR-0004): a login is allowed only when the
// email is present in approved_users with is_active = 1. There is no public
// signup; offboarding is disabling the row.
//
// FindActiveByEmail is the single lookup used by the Google OAuth callback.
func (m *Manager) FindActiveByEmail(ctx context.Context, email string) (sqlc.ApprovedUser, error) {
	return m.q.GetActiveApprovedUserByEmail(ctx, email)
}
