package repository

import "strconv"

// AsInt64 normalises the interface{} sqlc returns for aggregate projections.
func AsInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case []byte:
		return parseIntBytes(n)
	case string:
		if parsed, err := strconv.ParseInt(n, 10, 64); err == nil {
			return parsed
		}
	}
	return 0
}

// AsInt64Or wraps an (interface{}, error) aggregate result.
func AsInt64Or(v interface{}, _ error) int64 {
	return AsInt64(v)
}

func parseIntBytes(b []byte) int64 {
	v, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return 0
	}
	return v
}
