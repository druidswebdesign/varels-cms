package authweb

import (
	"database/sql"
	"log"
	"net/http"
	"strings"

	"github.com/druidswebdesign/varels-cms/internal/auth"
	"github.com/druidswebdesign/varels-cms/internal/db/sqlc"
	"github.com/druidswebdesign/varels-cms/internal/handlers/common"
	"github.com/druidswebdesign/varels-cms/internal/handlers/middleware"
	"github.com/druidswebdesign/varels-cms/internal/views/pages"
)

// HandleLoginPage renders GET /login. If already authenticated, skip to the
// dashboard.
func (s *Server) HandleLoginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.Sessions.AuthenticatedUser(r.Context()); ok {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	common.Render(w, r, http.StatusOK, pages.Login(middleware.CSRFToken(r), messageFor(r.URL.Query().Get("err"))))
}

// HandleLogin handles POST /login: a whitelisted employee email + bcrypt
// password, rate-limited and audit-logged. There is no public signup, so
// random people cannot create accounts (docs/auth.md).
func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	ip := common.ClientIP(r)
	if s.Limiter != nil && !s.Limiter.Allow(ip) {
		common.Render(w, r, http.StatusTooManyRequests,
			pages.Login(middleware.CSRFToken(r), "Too many attempts. Try again later."))
		return
	}

	_ = r.ParseForm()
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	user, err := s.Q.GetApprovedUserByEmail(r.Context(), email)
	valid := false
	if err == nil && user.IsActive == 1 && user.PasswordHash.Valid {
		valid = auth.CheckPassword(user.PasswordHash.String, password)
	} else {
		// Equalise timing and avoid revealing whether the account exists.
		auth.CheckDummy(password)
	}

	if !valid {
		if s.Limiter != nil {
			s.Limiter.Fail(ip)
		}
		common.Render(w, r, http.StatusUnauthorized,
			pages.Login(middleware.CSRFToken(r), "Invalid email or password."))
		return
	}

	if s.Limiter != nil {
		s.Limiter.Reset(ip)
	}
	if err := s.Sessions.Login(r.Context(), user); err != nil {
		log.Printf("login: session: %v", err)
		common.Render(w, r, http.StatusInternalServerError,
			pages.Login(middleware.CSRFToken(r), "Could not start session. Try again."))
		return
	}
	if err := s.Q.TouchApprovedUserLogin(r.Context(), user.ID); err != nil {
		log.Printf("login: touch login: %v", err)
	}
	s.auditLogin(r, user.ID, "local")

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// Google OAuth sign-in has been disabled in favour of email + bcrypt. The old
// whitelist flow is kept below for reference.
//
// // HandleAccessDenied renders the whitelist rejection page.
// func (s *Server) HandleAccessDenied(w http.ResponseWriter, r *http.Request) {
// 	common.Render(w, r, http.StatusOK, pages.Login(middleware.CSRFToken(r),
// 		"Access Denied: Your email is not authorized to view this system."))
// }
//
// // HandleGoogleBegin starts the Goth OAuth flow at GET /auth/google.
// func (s *Server) HandleGoogleBegin(w http.ResponseWriter, r *http.Request) {
// 	if !s.OAuthEnabled {
// 		http.Redirect(w, r, "/login?err=oauth", http.StatusSeeOther)
// 		return
// 	}
// 	gothic.BeginAuthHandler(w, auth.WithProvider(r, "google"))
// }
//
// // HandleGoogleCallback handles GET /auth/google/callback: exchange the code,
// // check the whitelist, then start a session.
// func (s *Server) HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
// 	if !s.OAuthEnabled {
// 		http.Redirect(w, r, "/login?err=oauth", http.StatusSeeOther)
// 		return
// 	}
//
// 	gu, err := gothic.CompleteUserAuth(w, auth.WithProvider(r, "google"))
// 	if err != nil {
// 		log.Printf("oauth: complete: %v", err)
// 		http.Redirect(w, r, "/auth/denied", http.StatusSeeOther)
// 		return
// 	}
//
// 	user, err := s.Sessions.FindActiveByEmail(r.Context(), gu.Email)
// 	if err != nil {
// 		http.Redirect(w, r, "/auth/denied", http.StatusSeeOther)
// 		return
// 	}
//
// 	if err := s.Sessions.Login(r.Context(), user); err != nil {
// 		log.Printf("oauth: session: %v", err)
// 		http.Redirect(w, r, "/auth/denied", http.StatusSeeOther)
// 		return
// 	}
// 	if err := s.Q.TouchApprovedUserLogin(r.Context(), user.ID); err != nil {
// 		log.Printf("oauth: touch login: %v", err)
// 	}
// 	s.auditLogin(r, user.ID, "google")
//
// 	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
// }
//
// // HandleAdminLoginPage renders the break-glass form at GET /admin-login. The
// // route is intentionally not linked anywhere in the UI (docs/auth.md).
// func (s *Server) HandleAdminLoginPage(w http.ResponseWriter, r *http.Request) {
// 	common.Render(w, r, http.StatusOK, pages.Login(middleware.CSRFToken(r), messageFor(r.URL.Query().Get("err"))))
// }

// HandleLogout destroys the session and returns to the login page.
func (s *Server) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.Sessions.Logout(r.Context()); err != nil {
		log.Printf("logout: %v", err)
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) auditLogin(r *http.Request, userID int64, method string) {
	_, err := s.Q.CreateAuditLog(r.Context(), sqlc.CreateAuditLogParams{
		UserID:     sql.NullInt64{Int64: userID, Valid: true},
		Action:     "login",
		EntityType: "approved_user",
		EntityID:   sql.NullInt64{Int64: userID, Valid: true},
		Note:       sql.NullString{String: method, Valid: true},
		Ip:         sql.NullString{String: common.ClientIP(r), Valid: true},
	})
	if err != nil {
		log.Printf("audit login: %v", err)
	}
}

func messageFor(code string) string {
	switch code {
	case "session":
		return "Your session expired. Please sign in again."
	default:
		return ""
	}
}
