package service

import "testing"

// TestIvaTaxMinor pins the IVA-inclusive derivation from ADR-0013:
// tax = total - round(total / 1.21), integer half-up.
func TestIvaTaxMinor(t *testing.T) {
	tests := []struct {
		name  string
		total int64
		want  int64
	}{
		{"zero", 0, 0},
		{"negative is clamped", -500, 0},
		{"one peso", 100, 17},            // 100 - 83 = 17
		{"exactly 121", 121, 21},         // 121 - 100 = 21
		{"rounds net half-up", 605, 105}, // 605*100+60=60560 /121 = 500, tax 105
		{"round price", 1_200_000, 208_264},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IvaTaxMinor(tt.total); got != tt.want {
				t.Errorf("IvaTaxMinor(%d) = %d, want %d", tt.total, got, tt.want)
			}
		})
	}
}

// TestIvaTaxMinorNeverExceedsTotal guards the invariant that the display-only
// tax is always a fraction of the total.
func TestIvaTaxMinorNeverExceedsTotal(t *testing.T) {
	for total := int64(1); total <= 100_000; total += 7 {
		tax := IvaTaxMinor(total)
		if tax < 0 || tax >= total {
			t.Fatalf("IvaTaxMinor(%d) = %d, out of range", total, tax)
		}
	}
}
