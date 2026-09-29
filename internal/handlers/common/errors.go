package common

import (
	"database/sql"
	"errors"
	"strings"
)

// IsNotFound reports whether err is sql.ErrNoRows.
func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// IsUniqueViolation reports whether err is a SQLite UNIQUE constraint failure.
func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
