package main

import (
	"testing"
	"time"
)

func TestParseShow(t *testing.T) {
	s, err := parseShow([]byte(`
image_seconds = 5
mute = false
fit = "cover"

[durations]
"intro.jpg" = 30
`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Mute || s.Fit != "cover" {
		t.Errorf("got %+v", s)
	}
	if got := s.imageDuration("intro.jpg"); got != 30*time.Second {
		t.Errorf("intro.jpg duration = %v", got)
	}
	if got := s.imageDuration("other.jpg"); got != 5*time.Second {
		t.Errorf("other.jpg duration = %v", got)
	}
}

func TestParseShowDefaults(t *testing.T) {
	s, err := parseShow(nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.ImageSeconds != 10 || !s.Mute || s.Fit != "contain" {
		t.Errorf("got %+v", s)
	}
}

func TestParseShowRejects(t *testing.T) {
	for name, input := range map[string]string{
		"unknown key":       `script = "evil.lua"`,
		"zero seconds":      `image_seconds = 0`,
		"huge duration":     "[durations]\n\"a.jpg\" = 999999",
		"bad fit":           `fit = "stretch"`,
		"wrong type":        `image_seconds = "10"`,
		"malformed":         `image_seconds =`,
		"unknown sub-table": "[mpv]\nvo = \"x\"",
		"bad clock":         "[schedule]\non = \"7am\"\noff = \"22:00\"",
		"missing off":       "[schedule]\non = \"07:00\"",
		"same on and off":   "[schedule]\non = \"07:00\"\noff = \"07:00\"",
	} {
		if _, err := parseShow([]byte(input)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestScheduleAwake(t *testing.T) {
	at := func(hhmm string) time.Time {
		tm, _ := time.Parse("15:04", hhmm)
		return tm
	}
	for input, want := range map[string]map[string]bool{
		"[schedule]\non = \"07:00\"\noff = \"22:00\"": {"06:59": false, "07:00": true, "21:59": true, "22:00": false},
		"[schedule]\non = \"18:00\"\noff = \"02:00\"": {"17:59": false, "18:00": true, "00:30": true, "02:00": false},
		"": {"03:00": true}, // no schedule means always on
	} {
		s, err := parseShow([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		for hhmm, on := range want {
			if got := s.Schedule.awake(at(hhmm)); got != on {
				t.Errorf("%q at %s: awake = %v, want %v", input, hhmm, got, on)
			}
		}
	}
}
