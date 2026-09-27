package auth

import "net/http"

// RequireLogin rejects unauthenticated requests and injects the user into the
// request context. It reloads the user from the database on every request, so
// disabling a whitelist row logs the user out on their next action.
func (m *Manager) RequireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := m.AuthenticatedUser(r.Context())
		if !ok {
			_ = m.Logout(r.Context())
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
	})
}

// RequireRole enforces a minimum role. It must run after RequireLogin, which
// puts the user on the context.
func (m *Manager) RequireRole(min Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := UserFromContext(r.Context())
			if !ok {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			if !u.Role.AtLeast(min) {
				if r.Header.Get("HX-Request") == "true" {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				http.Error(w, "forbidden: admin role required", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
