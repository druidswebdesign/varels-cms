package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/druidswebdesign/varels-cms/internal/db/sqlc"
)

const (
	sessionKeyUserID = "uid"
	sessionKeyRole   = "role"
	sessionKeyEmail  = "email"

	sessionKeyFlashMsg  = "flash_msg"
	sessionKeyFlashTone = "flash_tone"
)

// Manager owns the SCS session manager and exposes auth helpers. The session
// data lives in the app's own `sessions` table through sessionStore.
type Manager struct {
	sm *scs.SessionManager
	q  *sqlc.Queries
}

// NewManager builds a session manager backed by the `sessions` table.
func NewManager(q *sqlc.Queries, lifetime time.Duration, secure bool) *Manager {
	sm := scs.New()
	sm.Store = &sessionStore{q: q}
	sm.Lifetime = lifetime
	sm.Cookie.Name = "varels_session"
	sm.Cookie.Path = "/"
	sm.Cookie.HttpOnly = true
	sm.Cookie.Secure = secure
	sm.Cookie.SameSite = http.SameSiteLaxMode

	return &Manager{sm: sm, q: q}
}

// LoadAndSave must wrap the whole handler tree before RequireLogin.
func (m *Manager) LoadAndSave(next http.Handler) http.Handler {
	return m.sm.LoadAndSave(next)
}

// Login records the user in the session and rotates the session token to
// prevent session fixation. LoadAndSave has already loaded the session for the
// request; SCS commits it at the end of the request.
func (m *Manager) Login(ctx context.Context, u sqlc.ApprovedUser) error {
	m.sm.Put(ctx, sessionKeyUserID, u.ID)
	m.sm.Put(ctx, sessionKeyRole, string(u.Role))
	m.sm.Put(ctx, sessionKeyEmail, u.Email)
	return m.sm.RenewToken(ctx)
}

// Logout destroys the session.
func (m *Manager) Logout(ctx context.Context) error {
	return m.sm.Destroy(ctx)
}

// CleanupExpiredSessions deletes session rows that have expired. Call
// periodically (e.g. hourly) so the sessions table does not grow without bound.
func (m *Manager) CleanupExpiredSessions(ctx context.Context) error {
	return m.q.DeleteExpiredSessions(ctx, time.Now().UTC().Format(time.RFC3339))
}

// SetFlash stores a one-shot message for the next rendered page.
func (m *Manager) SetFlash(ctx context.Context, message, tone string) {
	m.sm.Put(ctx, sessionKeyFlashMsg, message)
	m.sm.Put(ctx, sessionKeyFlashTone, tone)
}

// PopFlash returns and clears any pending flash message.
func (m *Manager) PopFlash(ctx context.Context) (message, tone string, ok bool) {
	message = m.sm.GetString(ctx, sessionKeyFlashMsg)
	if message == "" {
		return "", "", false
	}
	tone = m.sm.GetString(ctx, sessionKeyFlashTone)
	m.sm.Remove(ctx, sessionKeyFlashMsg)
	m.sm.Remove(ctx, sessionKeyFlashTone)
	return message, tone, true
}

// AuthenticatedUser returns the current user, re-checking the DB on every call
// so disabling a whitelist row revokes access immediately (docs/auth.md).
func (m *Manager) AuthenticatedUser(ctx context.Context) (User, bool) {
	id := m.sm.GetInt64(ctx, sessionKeyUserID)
	if id <= 0 {
		return User{}, false
	}
	au, err := m.q.GetApprovedUser(ctx, id)
	if err != nil || au.IsActive != 1 {
		return User{}, false
	}
	return User{
		ID:          au.ID,
		Email:       au.Email,
		DisplayName: au.DisplayName.String,
		Role:        Role(au.Role),
	}, true
}

// sessionStore adapts the `sessions` table to the scs.Store interface. The
// window between requests has no context, so it uses context.Background().
type sessionStore struct {
	q *sqlc.Queries
}

func (s *sessionStore) Find(token string) ([]byte, bool, error) {
	sess, err := s.q.GetSession(context.Background(), token)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if expired(sess.ExpiresAt) {
		return nil, false, nil
	}
	return sess.Data, true, nil
}

func (s *sessionStore) Commit(token string, b []byte, expiry time.Time) error {
	ctx := context.Background()
	// The schema keys sessions by token; replace any existing row.
	if err := s.q.DeleteSession(ctx, token); err != nil {
		return err
	}
	return s.q.CreateSession(ctx, sqlc.CreateSessionParams{
		Token:     token,
		Data:      b,
		ExpiresAt: expiry.UTC().Format(time.RFC3339),
	})
}

func (s *sessionStore) Delete(token string) error {
	return s.q.DeleteSession(context.Background(), token)
}

func expired(ts string) bool {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return false
	}
	return time.Now().UTC().After(t)
}
