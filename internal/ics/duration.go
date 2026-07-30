package ics

import (
	"fmt"
	"strings"
	"time"
)

// Duration renders a non-negative duration as an RFC 5545 / ISO 8601 duration
// value, e.g. 0 -> "PT0S", 90m -> "PT1H30M", 25h -> "P1DT1H".
//
// Minutes are the smallest unit the callers need (alarm lead times, feed refresh
// intervals), so anything below a minute — including negative input — collapses
// to "PT0S". This is the only duration formatter in the tree; both the alarm
// rule's TRIGGER and the feed's REFRESH-INTERVAL go through it.
func Duration(d time.Duration) string {
	totalMinutes := int(d / time.Minute)
	if totalMinutes <= 0 {
		return "PT0S"
	}
	days := totalMinutes / (24 * 60)
	hours := (totalMinutes % (24 * 60)) / 60
	minutes := totalMinutes % 60

	var sb strings.Builder
	sb.WriteString("P")
	if days > 0 {
		fmt.Fprintf(&sb, "%dD", days)
	}
	if hours > 0 || minutes > 0 {
		sb.WriteString("T")
		if hours > 0 {
			fmt.Fprintf(&sb, "%dH", hours)
		}
		if minutes > 0 {
			fmt.Fprintf(&sb, "%dM", minutes)
		}
	}
	return sb.String()
}
