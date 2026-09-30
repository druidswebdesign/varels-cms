package reporting

import (
	"strings"
	"testing"
	"time"
)

func TestPeriodRangeMonth(t *testing.T) {
	start, end := PeriodRange(2026, 9)
	// 2026-09-01 00:00 ART (UTC-3) -> 03:00Z.
	if start != "2026-09-01T03:00:00Z" {
		t.Errorf("start = %q, want 2026-09-01T03:00:00Z", start)
	}
	// End is the last ART second of September: 2026-09-30 23:59:59 ART -> 2026-10-01 02:59:59Z.
	if end != "2026-10-01T02:59:59Z" {
		t.Errorf("end = %q, want 2026-10-01T02:59:59Z", end)
	}
}

func TestPeriodRangeWholeYear(t *testing.T) {
	start, end := PeriodRange(2026, 0)
	if start != "2026-01-01T03:00:00Z" {
		t.Errorf("start = %q, want 2026-01-01T03:00:00Z", start)
	}
	if end != "2027-01-01T02:59:59Z" {
		t.Errorf("end = %q, want 2027-01-01T02:59:59Z", end)
	}
}

func TestPeriodRangeBoundariesAreOrdered(t *testing.T) {
	for month := 1; month <= 12; month++ {
		start, end := PeriodRange(2026, month)
		s, err := time.Parse(time.RFC3339, start)
		if err != nil {
			t.Fatalf("month %d start not RFC3339: %v", month, err)
		}
		e, err := time.Parse(time.RFC3339, end)
		if err != nil {
			t.Fatalf("month %d end not RFC3339: %v", month, err)
		}
		if !e.After(s) {
			t.Errorf("month %d: end %s not after start %s", month, end, start)
		}
	}
}

func TestISOUTCFormatsSchemaShape(t *testing.T) {
	if got := ISOUTC(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)); got != "2026-01-02T03:04:05Z" {
		t.Errorf("ISOUTC = %q", got)
	}
}

func TestAllTimeRangeAndStaleSince(t *testing.T) {
	start, end := AllTimeRange()
	if start != "1970-01-01T00:00:00Z" {
		t.Errorf("all-time start = %q", start)
	}
	if !strings.HasSuffix(end, "Z") {
		t.Errorf("all-time end = %q, want UTC", end)
	}
	if got := StaleSince(30); !strings.HasSuffix(got, "Z") {
		t.Errorf("StaleSince = %q, want UTC", got)
	}
}
