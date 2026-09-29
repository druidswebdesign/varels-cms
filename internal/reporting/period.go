// Package reporting holds Argentina-local reporting time ranges (ADR-0014).
package reporting

import "time"

// ART is the Argentina reporting zone (UTC-3, no DST) per ADR-0014.
var ART = time.FixedZone("ART", -3*60*60)

// PeriodRange returns the inclusive UTC ISO bounds for an Argentina-local
// year+month. month == 0 means the whole year.
func PeriodRange(year, month int) (string, string) {
	if month == 0 {
		start := time.Date(year, 1, 1, 0, 0, 0, 0, ART)
		end := time.Date(year+1, 1, 1, 0, 0, 0, 0, ART)
		return ISOUTC(start), ISOUTC(end.Add(-time.Second))
	}
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, ART)
	end := start.AddDate(0, 1, 0)
	return ISOUTC(start), ISOUTC(end.Add(-time.Second))
}

// AllTimeRange covers everything up to now.
func AllTimeRange() (string, string) {
	return "1970-01-01T00:00:00Z", ISOUTC(time.Now().UTC())
}

// StaleSince returns the UTC timestamp N days before now, for deadstock.
func StaleSince(days int) string {
	return ISOUTC(time.Now().UTC().AddDate(0, 0, -days))
}

// ISOUTC formats a time in the ISO-8601 UTC form the schema uses.
func ISOUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}
