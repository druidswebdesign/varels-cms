package admin

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/yourname/varels_cms/internal/auth"
	"github.com/yourname/varels_cms/internal/db/sqlc"
	"github.com/yourname/varels_cms/internal/db/types"
	"github.com/yourname/varels_cms/internal/handlers/common"
	"github.com/yourname/varels_cms/internal/handlers/middleware"
	"github.com/yourname/varels_cms/internal/views/pages"
)

// HandleStaffList renders GET /staff.
func (s *Server) HandleStaffList(w http.ResponseWriter, r *http.Request) {
	users, err := s.Q.ListApprovedUsers(r.Context())
	if err != nil {
		common.ServerError(w, err)
		return
	}
	common.Render(w, r, http.StatusOK, pages.Staff(users, middleware.CSRFToken(r)))
}

// HandleStaffInvite handles POST /staff/invite: adds an email to the whitelist.
func (s *Server) HandleStaffInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()

	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	if email == "" || !strings.Contains(email, "@") {
		s.SetFlash(r, "Enter a valid email address.", "error")
		http.Redirect(w, r, "/staff", http.StatusSeeOther)
		return
	}

	role := types.UserRole(strings.TrimSpace(r.FormValue("role")))
	if role != types.UserRoleAdmin && role != types.UserRoleStaff {
		role = types.UserRoleStaff
	}

	if _, err := s.Q.CreateApprovedUser(ctx, sqlc.CreateApprovedUserParams{
		Email:       email,
		DisplayName: common.NullString(r.FormValue("display_name")),
		Role:        role,
		Provider:    types.AuthProviderGoogle,
		InvitedBy:   common.CurrentUserID(r),
	}); err != nil {
		if common.IsUniqueViolation(err) {
			s.SetFlash(r, "That email is already on the whitelist.", "error")
			http.Redirect(w, r, "/staff", http.StatusSeeOther)
			return
		}
		common.ServerError(w, err)
		return
	}

	s.SetFlash(r, "Staff invited.", "success")
	http.Redirect(w, r, "/staff", http.StatusSeeOther)
}

// HandleStaffRole handles POST /staff/{id}/role.
func (s *Server) HandleStaffRole(w http.ResponseWriter, r *http.Request) {
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	role := types.UserRole(strings.TrimSpace(r.FormValue("role")))
	if role != types.UserRoleAdmin && role != types.UserRoleStaff {
		s.SetFlash(r, "Invalid role.", "error")
		http.Redirect(w, r, "/staff", http.StatusSeeOther)
		return
	}
	if err := s.Q.SetApprovedUserRole(r.Context(), sqlc.SetApprovedUserRoleParams{Role: role, ID: id}); err != nil {
		common.ServerError(w, err)
		return
	}
	s.SetFlash(r, "Role updated.", "success")
	http.Redirect(w, r, "/staff", http.StatusSeeOther)
}

// HandleStaffDisable handles POST /staff/{id}/disable (instant offboarding).
func (s *Server) HandleStaffDisable(w http.ResponseWriter, r *http.Request) {
	s.setStaffActive(w, r, 0)
}

// HandleStaffEnable handles POST /staff/{id}/enable.
func (s *Server) HandleStaffEnable(w http.ResponseWriter, r *http.Request) {
	s.setStaffActive(w, r, 1)
}

func (s *Server) setStaffActive(w http.ResponseWriter, r *http.Request, active int64) {
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.Q.SetApprovedUserActive(r.Context(), sqlc.SetApprovedUserActiveParams{IsActive: active, ID: id}); err != nil {
		common.ServerError(w, err)
		return
	}
	if active == 1 {
		s.SetFlash(r, "Access enabled.", "success")
	} else {
		s.SetFlash(r, "Access disabled — offboarding is instant.", "warning")
	}
	http.Redirect(w, r, "/staff", http.StatusSeeOther)
}

// HandleStaffResetPassword handles POST /staff/{id}/reset-password. Only the
// break-glass local accounts carry a password (provider = 'local').
func (s *Server) HandleStaffResetPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := common.URLID(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	u, err := s.Q.GetApprovedUser(ctx, id)
	if err != nil {
		if common.IsNotFound(err) {
			http.NotFound(w, r)
			return
		}
		common.ServerError(w, err)
		return
	}
	if u.Provider != types.AuthProviderLocal {
		s.SetFlash(r, "Only break-glass local accounts have a password.", "error")
		http.Redirect(w, r, "/staff", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()

	password := r.FormValue("password")
	if len(password) < 8 {
		s.SetFlash(r, "Password must be at least 8 characters.", "error")
		http.Redirect(w, r, "/staff", http.StatusSeeOther)
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		common.ServerError(w, err)
		return
	}
	if err := s.Q.SetApprovedUserPassword(ctx, sqlc.SetApprovedUserPasswordParams{
		PasswordHash: sql.NullString{String: hash, Valid: true},
		ID:           id,
	}); err != nil {
		common.ServerError(w, err)
		return
	}
	s.Audit(r, "reset_password", "approved_user", id, "break-glass password reset")
	s.SetFlash(r, "Password reset.", "success")
	http.Redirect(w, r, "/staff", http.StatusSeeOther)
}
