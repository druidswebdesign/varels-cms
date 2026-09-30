package common

import (
	"net/http"

	"github.com/druidswebdesign/varels-cms/internal/views/flash"
)

// FlashFromSession moves any pending session flash into the request context so
// the layout renders it exactly once. Must run after Sessions.LoadAndSave.
func (s *Server) FlashFromSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Sessions == nil {
			next.ServeHTTP(w, r)
			return
		}
		if msg, tone, ok := s.Sessions.PopFlash(r.Context()); ok {
			r = r.WithContext(flash.With(r.Context(), flash.Flash{Message: msg, Tone: tone}))
		}
		next.ServeHTTP(w, r)
	})
}

// SetFlash stores a one-shot message for the next rendered page.
func (s *Server) SetFlash(r *http.Request, message, tone string) {
	if s.Sessions != nil {
		s.Sessions.SetFlash(r.Context(), message, tone)
	}
}
