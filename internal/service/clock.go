// Package service holds business logic that is shared across handler packages:
// FIFO inventory costing, order numbering and tax derivation.
package service

import "time"

// NowUTC returns the current time in the ISO-8601 UTC form the schema uses.
func NowUTC() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}
