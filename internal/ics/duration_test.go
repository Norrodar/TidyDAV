package ics

import (
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{0, "PT0S"},
		{-3600, "PT0S"},
		{30, "PT0S"}, // below a minute: no sub-minute precision
		{60, "PT1M"},
		{900, "PT15M"},
		{3600, "PT1H"},
		{5400, "PT1H30M"},
		{21600, "PT6H"},
		{86400, "P1D"},
		{90000, "P1DT1H"},
		{93600, "P1DT2H"},
		{93720, "P1DT2H2M"},
	}
	for _, tt := range tests {
		got := Duration(time.Duration(tt.seconds) * time.Second)
		if got != tt.want {
			t.Errorf("Duration(%ds) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}
