package main

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
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
`), defaultShow)
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
	s, err := parseShow(nil, defaultShow)
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
		if _, err := parseShow([]byte(input), defaultShow); err == nil {
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
		s, err := parseShow([]byte(input), defaultShow)
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

func TestDecodeSetupCode(t *testing.T) {
	encode := func(json string) string { return base64.StdEncoding.EncodeToString([]byte(json)) }

	d, err := decodeSetupCode(encode(`{"source":"smb://nas/screens","username":"pi","password":"pw",
		"show":{"fit":"cover","shuffle":true,"durations":{"intro.jpg":30},"schedule":{"on":"07:00","off":"22:00"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	s := d.baseShow()
	// Settings the code leaves out keep their defaults.
	if s.ImageSeconds != 10 || !s.Mute || s.Fit != "cover" || !s.Shuffle || s.Durations["intro.jpg"] != 30 {
		t.Errorf("got %+v", s)
	}
	if s.Schedule == nil || s.Schedule.On != 7*60 || s.Schedule.Off != 22*60 {
		t.Errorf("schedule = %+v", s.Schedule)
	}

	// What setup saves must load back the same.
	saved, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	again, err := parseDevice(saved)
	if err != nil {
		t.Fatalf("reload %s: %v", saved, err)
	}
	if !reflect.DeepEqual(again, d) {
		t.Errorf("round trip changed device:\n%+v\n%+v", d, again)
	}

	if d, err := decodeSetupCode(encode(`{"source":"/home/pi/media"}`)); err != nil || d.Show != nil {
		t.Errorf("no show: got %+v, %v", d, err)
	}

	for name, input := range map[string]string{
		"not base64":   "%%%",
		"no source":    encode(`{"show":{}}`),
		"unknown key":  encode(`{"source":"/x","sauce":"/y"}`),
		"unknown show": encode(`{"source":"/x","show":{"script":"evil.lua"}}`),
		"bad seconds":  encode(`{"source":"/x","show":{"image_seconds":0}}`),
		"bad fit":      encode(`{"source":"/x","show":{"fit":"stretch"}}`),
		"missing off":  encode(`{"source":"/x","show":{"schedule":{"on":"07:00"}}}`),
		"bad clock":    encode(`{"source":"/x","show":{"schedule":{"on":"7am","off":"22:00"}}}`),
	} {
		if _, err := decodeSetupCode(input); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestParseShowOverBase(t *testing.T) {
	base := Show{ImageSeconds: 20, Mute: false, Fit: "cover", Durations: map[string]int{"a.jpg": 5},
		Schedule: &Schedule{On: 7 * 60, Off: 22 * 60}}

	s, err := parseShow([]byte("image_seconds = 8\n[durations]\n\"b.jpg\" = 9"), base)
	if err != nil {
		t.Fatal(err)
	}
	// The file's keys win; everything else, including the schedule, comes from base.
	if s.ImageSeconds != 8 || s.Mute || s.Fit != "cover" || s.Schedule != base.Schedule {
		t.Errorf("got %+v", s)
	}
	if s.Durations["a.jpg"] != 5 || s.Durations["b.jpg"] != 9 {
		t.Errorf("durations = %v", s.Durations)
	}
	if len(base.Durations) != 1 {
		t.Errorf("base durations modified: %v", base.Durations)
	}

	if s, err := parseShow([]byte("fit = \"stretch\""), base); err == nil || !reflect.DeepEqual(s, base) {
		t.Errorf("bad file: got %+v, %v; want base", s, err)
	}
}
