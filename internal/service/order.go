package service

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

// NewOrderNumber returns a short random human-facing receipt id. It is not a
// sequence, so receipt ids do not leak order volume (docs/schema.md §8).
func NewOrderNumber() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "ORD-000000"
	}
	return "ORD-" + strings.ToUpper(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
}
