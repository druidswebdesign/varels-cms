package auth

import "testing"

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("hash equals plaintext")
	}
	if !CheckPassword(hash, "correct horse battery staple") {
		t.Error("correct password rejected")
	}
	if CheckPassword(hash, "wrong password") {
		t.Error("wrong password accepted")
	}
}

func TestHashPasswordIsSalted(t *testing.T) {
	a, err := HashPassword("same")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	b, err := HashPassword("same")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if a == b {
		t.Error("two hashes of the same password are identical; bcrypt salt missing")
	}
}

func TestCheckPasswordRejectsGarbageHash(t *testing.T) {
	if CheckPassword("not-a-bcrypt-hash", "anything") {
		t.Error("garbage hash accepted")
	}
}

func TestCheckDummyDoesNotPanic(t *testing.T) {
	CheckDummy("anything")
}

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		password string
		wantErr  bool
	}{
		{"", true},
		{"short", true},
		{"12345678901", true}, // 11 chars, below min
		{"123456789012", false},
		{"a long passphrase works", false},
	}
	for _, tt := range tests {
		err := ValidatePasswordStrength(tt.password)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidatePasswordStrength(%q) err = %v, wantErr %v", tt.password, err, tt.wantErr)
		}
	}
}
