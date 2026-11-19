package ui

import "time"

const dateColumnWidth = 156

func displayDateTime(value string) string {
	if stamp, err := time.Parse(time.RFC3339, value); err == nil {
		return formatLocalTime(stamp)
	}
	// Older Agent responses may contain only a calendar date.
	return value
}

func formatLocalTime(stamp time.Time) string {
	return stamp.Local().Format(time.DateTime)
}
