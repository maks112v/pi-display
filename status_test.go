package main

import (
	"testing"
	"time"
)

func TestScreenState(t *testing.T) {
	now := time.Date(2026, 10, 26, 22, 0, 0, 0, time.Local)
	daytime := daily(8*60, 18*60)
	tests := []struct {
		name     string
		schedule *Schedule
		override time.Time
		want     string
	}{
		{"no schedule", nil, time.Time{}, "on, no schedule"},
		{"outside schedule", daytime, time.Time{}, "off, schedule daily 08:00-18:00"},
		{"inside overnight schedule", daily(20*60, 6*60), time.Time{}, "on, schedule daily 20:00-06:00"},
		{"override wins", daytime, now.Add(time.Hour), "on until Mon 23:00 (pi-display on)"},
		{"expired override", daytime, now.Add(-time.Hour), "off, schedule daily 08:00-18:00"},
	}
	for _, tt := range tests {
		if got := screenState(tt.schedule, tt.override, now); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
