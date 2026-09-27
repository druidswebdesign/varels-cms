// Package format holds display-only helpers used by the templ views. All
// conversions from storage representation to human output live here so the
// templates stay free of arithmetic (docs/schema.md §6).
package format

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// Money renders ARS minor units (centavos) as "$1.234,56" (es-AR). Money is
// always an integer on the wire (ADR-0005); formatting happens only here.
func Money(minor int64) string {
	neg := minor < 0
	if neg {
		minor = -minor
	}
	whole := minor / 100
	cents := minor % 100

	var b strings.Builder
	s := strconv.FormatInt(whole, 10)
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}

	out := fmt.Sprintf("$%s,%02d", b.String(), cents)
	if neg {
		return "-" + out
	}
	return out
}

// PercentFromBps renders integer basis points as a percentage (2000 -> "20.00%").
func PercentFromBps(bps int64) string {
	return fmt.Sprintf("%.2f%%", float64(bps)/100.0)
}

// PercentOf renders part/whole as a percentage for display only.
func PercentOf(part, whole int64) string {
	if whole == 0 {
		return "0.00%"
	}
	return fmt.Sprintf("%.2f%%", float64(part)/float64(whole)*100)
}

// MinorToPlain renders minor units as a plain decimal string for form inputs,
// e.g. 1200050 -> "12000.50" (no separators, no currency symbol).
func MinorToPlain(minor int64) string {
	return fmt.Sprintf("%d.%02d", minor/100, minor%100)
}

// NullString returns the string value or "" when NULL.
func NullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// NullInt64 returns the value or 0 when NULL.
func NullInt64(ni sql.NullInt64) int64 {
	if ni.Valid {
		return ni.Int64
	}
	return 0
}

// GrossMarginPct computes unit gross margin % from retail and cost minor units.
// Returns 0 when retail is zero to avoid divide-by-zero.
func GrossMarginPct(retailMinor, costMinor int64) float64 {
	if retailMinor <= 0 {
		return 0
	}
	return float64(retailMinor-costMinor) / float64(retailMinor) * 100
}
