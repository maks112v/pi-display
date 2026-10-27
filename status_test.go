package main

import (
	"testing"
	"time"
)

func TestScreenState(t *testing.T) {
	now := time.Date(2026, 10, 26, 22, 0, 0, 0, time.Local)
	daytime := daily(8*60, 18*60)
	evening := daily(20*60, 6*60)
	tests := []struct {
		name     string
		schedule *Schedule
		override override
		want     string
	}{
		{"no schedule", nil, override{}, "on, no schedule"},
		{"outside schedule", daytime, override{}, "off, schedule daily 08:00-18:00"},
		{"inside overnight schedule", evening, override{}, "on, schedule daily 20:00-06:00"},
		{"on wins", daytime, override{On: true, Until: now.Add(time.Hour)}, "on until Mon 23:00 (pi-display on)"},
		{"off wins", evening, override{Until: now.Add(time.Hour)}, "off until Mon 23:00 (pi-display off)"},
		{"off with no schedule", nil, override{Until: now.Add(time.Hour)}, "off until Mon 23:00 (pi-display off)"},
		{"expired override", daytime, override{On: true, Until: now.Add(-time.Hour)}, "off, schedule daily 08:00-18:00"},
	}
	for _, tt := range tests {
		if got := screenState(tt.schedule, tt.override, now); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
		if got, want := tt.override.screenOn(tt.schedule, now), tt.want[:3] != "off"; got != want {
			t.Errorf("%s: screenOn = %v, want %v", tt.name, got, want)
		}
	}
}
