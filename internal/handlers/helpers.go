package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/yourname/varels_cms/internal/auth"
)

// render writes a templ component with the given status code.
func render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.Render(r.Context(), w); err != nil {
		log.Printf("render: %v", err)
	}
}

// urlID parses an integer path parameter.
func urlID(r *http.Request, key string) (int64, error) {
	raw := chi.URLParam(r, key)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q", key, raw)
	}
	return id, nil
}

// parsePesos converts a whole/decimal ARS amount from a form field to integer
// minor units (centavos) without using floats (ADR-0005). Accepts "12000" and
// "12000.50". Thousands separators are rejected rather than guessed.
func parsePesos(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if strings.ContainsAny(s, ", ") {
		return 0, errors.New("enter digits only, e.g. 12000 or 12000.50")
	}
	whole, frac, _ := strings.Cut(s, ".")
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, err
	}
	minor := w * 100

	if frac != "" {
		if len(frac) > 2 {
			frac = frac[:2]
		}
		for len(frac) < 2 {
			frac += "0"
		}
		f, err := strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, err
		}
		minor += f
	}
	return minor, nil
}

// nullString maps an empty form value to NULL.
func nullString(s string) sql.NullString {
	s = strings.TrimSpace(s)
	return sql.NullString{String: s, Valid: s != ""}
}

// nullInt64 maps an empty form value to NULL, otherwise parses it.
func nullInt64(s string) sql.NullInt64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullInt64{}
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: v, Valid: true}
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// slugify produces a URL-safe slug from a product name.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugPattern.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// isNotFound reports whether err is sql.ErrNoRows.
func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// currentUserID returns the authenticated user id for audit/ownership columns.
func currentUserID(r *http.Request) sql.NullInt64 {
	if u, ok := auth.UserFromContext(r.Context()); ok {
		return sql.NullInt64{Int64: u.ID, Valid: true}
	}
	return sql.NullInt64{}
}

// isHX reports whether the request came from HTMX.
func isHX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// nowUTC returns the current time in the ISO-8601 UTC form the schema uses.
func nowUTC() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// itoa is a small helper for building redirect paths.
func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint failure.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// serverError logs the error and renders a 500.
func serverError(w http.ResponseWriter, err error) {
	log.Printf("handler error: %v", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}
