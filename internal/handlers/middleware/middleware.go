// Package middleware holds the HTTP middleware shared by the handler packages.
package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

type contextKey int

const (
	requestIDKey contextKey = iota
	csrfTokenKey
)

// RequestID attaches a request id to the context and the X-Request-Id header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := NewToken(8)
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// statusRecorder captures the response status for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Logger logs method, path, status and duration for every request.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}

// SecurityHeaders sets baseline hardening headers for a private app. The CSP
// allows the inline theme script and the ApexCharts CDN the templates already
// use; everything else is locked to same-origin.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"font-src 'self'; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

// Recover turns panics into 500 responses instead of crashing the server.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v\n%s", rec, debug.Stack())
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CSRF implements the double-submit cookie pattern: a random token is stored in
// a cookie and must be echoed back on state-changing requests, either in the
// X-CSRF-Token header (set globally via hx-headers) or the csrf_token form
// field. It needs no server-side session store.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if c, err := r.Cookie(csrfCookieName); err == nil {
			token = c.Value
		}
		if token == "" {
			token = NewToken(32)
			http.SetCookie(w, &http.Cookie{
				Name:     csrfCookieName,
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   r.TLS != nil,
			})
		}

		ctx := context.WithValue(r.Context(), csrfTokenKey, token)

		if isStateChanging(r.Method) {
			sent := r.Header.Get("X-CSRF-Token")
			if sent == "" {
				sent = r.PostFormValue("csrf_token")
			}
			if sent == "" || subtle.ConstantTimeCompare([]byte(sent), []byte(token)) != 1 {
				http.Error(w, "invalid CSRF token", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

const csrfCookieName = "csrf_token"

func isStateChanging(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// CSRFToken returns the token created by CSRF for the request.
func CSRFToken(r *http.Request) string {
	if v, ok := r.Context().Value(csrfTokenKey).(string); ok {
		return v
	}
	return ""
}

// RequestIDFromContext returns the request id attached by RequestID.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// NewToken returns n random bytes hex-encoded.
func NewToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is unrecoverable; fall back to a fixed value so the
		// server keeps running, and log loudly.
		log.Printf("crypto/rand: %v", err)
		return strings.Repeat("0", n*2)
	}
	return hex.EncodeToString(b)
}
