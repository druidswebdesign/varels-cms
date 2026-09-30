package auth

import (
	"testing"
	"time"
)

func TestLoginLimiterBlocksAfterMaxFailures(t *testing.T) {
	l := NewLoginLimiter(3, time.Minute)
	ip := "10.0.0.1"

	for i := 0; i < 3; i++ {
		if !l.Allow(ip) {
			t.Fatalf("attempt %d blocked before reaching max", i+1)
		}
		l.Fail(ip)
	}
	if l.Allow(ip) {
		t.Error("4th attempt allowed, want blocked at max=3")
	}
	if !l.Allow("10.0.0.2") {
		t.Error("one IP's failures must not block another IP")
	}
}

func TestLoginLimiterResetClearsFailures(t *testing.T) {
	l := NewLoginLimiter(1, time.Minute)
	ip := "10.0.0.1"

	l.Fail(ip)
	if l.Allow(ip) {
		t.Fatal("attempt allowed after max failures")
	}
	l.Reset(ip)
	if !l.Allow(ip) {
		t.Error("Reset did not clear failures")
	}
}

func TestLoginLimiterWindowExpiry(t *testing.T) {
	l := NewLoginLimiter(1, time.Minute)
	base := time.Now()
	l.now = func() time.Time { return base }
	ip := "10.0.0.1"

	l.Fail(ip)
	if l.Allow(ip) {
		t.Fatal("attempt allowed within window")
	}
	l.now = func() time.Time { return base.Add(2 * time.Minute) }
	if !l.Allow(ip) {
		t.Error("attempt still blocked after window elapsed")
	}
}
