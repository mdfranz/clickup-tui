package format

import (
	"strconv"
	"time"
)

// parseUnixMillis parses a Unix timestamp in milliseconds (as string) and returns a time.Time.
func parseUnixMillis(unixMillisStr string) (time.Time, error) {
	ms, err := strconv.ParseInt(unixMillisStr, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, ms*int64(time.Millisecond)), nil
}

// FormatTaskDate formats a Unix millisecond timestamp string as "01/02" (MM/DD).
// Returns empty string if parsing fails.
func FormatTaskDate(unixMillisStr string) string {
	if unixMillisStr == "" {
		return ""
	}
	t, err := parseUnixMillis(unixMillisStr)
	if err != nil {
		return ""
	}
	return t.Format("01/02")
}

// FormatCommentDate formats a Unix millisecond timestamp string as "01/02 15:04" (MM/DD HH:MM).
// Returns empty string if parsing fails.
func FormatCommentDate(unixMillisStr string) string {
	if unixMillisStr == "" {
		return ""
	}
	t, err := parseUnixMillis(unixMillisStr)
	if err != nil {
		return ""
	}
	return t.Format("01/02 15:04")
}
