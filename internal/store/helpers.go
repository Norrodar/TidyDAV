package store

import "time"

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// parseTime parses an RFC3339 timestamp, returning the zero time on failure.
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
