package service

import (
	"strings"
	"testing"
)

func TestNewOrderNumberShape(t *testing.T) {
	got := NewOrderNumber()
	if !strings.HasPrefix(got, "ORD-") {
		t.Errorf("order number %q missing ORD- prefix", got)
	}
	// 6 random bytes -> 10 base32 chars without padding.
	if body := strings.TrimPrefix(got, "ORD-"); len(body) != 10 {
		t.Errorf("order number body %q length = %d, want 10", body, len(body))
	}
	if got != strings.ToUpper(got) {
		t.Errorf("order number %q not upper-case", got)
	}
}

func TestNewOrderNumberIsNotSequential(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		n := NewOrderNumber()
		if seen[n] {
			t.Fatalf("duplicate order number %q after %d draws", n, i)
		}
		seen[n] = true
	}
}
