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
	} {
		if _, err := parseShow([]byte(input)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
