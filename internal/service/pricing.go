package service

// IvaTaxMinor derives the display-only IVA portion of an IVA-inclusive total
// (ADR-0013): total − round(total / 1.21), rounded half-up in integers.
func IvaTaxMinor(total int64) int64 {
	if total <= 0 {
		return 0
	}
	net := (total*100 + 60) / 121
	return total - net
}
