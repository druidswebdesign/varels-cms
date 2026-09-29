package common

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
)

// ProductListLimit caps the number of products returned by list screens.
const ProductListLimit = 200

// Render writes a templ component with the given status code.
func Render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.Render(r.Context(), w); err != nil {
		log.Printf("Render: %v", err)
	}
}

// ServerError logs the error and renders a 500.
func ServerError(w http.ResponseWriter, err error) {
	log.Printf("handler error: %v", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

// URLID parses an integer path parameter.
func URLID(r *http.Request, key string) (int64, error) {
	raw := chi.URLParam(r, key)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q", key, raw)
	}
	return id, nil
}

// Itoa is a small helper for building redirect paths.
func Itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}

// IsHX reports whether the request came from HTMX.
func IsHX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
