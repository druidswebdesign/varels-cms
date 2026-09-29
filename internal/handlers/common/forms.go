package common

import (
	"database/sql"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// ParsePesos converts a whole/decimal ARS amount from a form field to integer
// minor units (centavos) without using floats (ADR-0005). Accepts "12000" and
// "12000.50". Thousands separators are rejected rather than guessed.
func ParsePesos(s string) (int64, error) {
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

// NullString maps an empty form value to NULL.
func NullString(s string) sql.NullString {
	s = strings.TrimSpace(s)
	return sql.NullString{String: s, Valid: s != ""}
}

// NullInt64 maps an empty form value to NULL, otherwise parses it.
func NullInt64(s string) sql.NullInt64 {
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

// ParseInt64 parses a form value, returning ok=false when empty/invalid.
func ParseInt64(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify produces a URL-safe slug from a product name.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugPattern.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
