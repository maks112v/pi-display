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
		"unquoted clock":    "[schedule]\non = 07:00\noff = \"22:00\"",
		"daily and days":    "[schedule]\non = \"07:00\"\noff = \"22:00\"\nsun = [\"09:00-17:00\"]",
		"unknown day":       "[schedule]\nsunday = [\"09:00-17:00\"]",
		"day not a list":    "[schedule]\nsun = \"09:00-17:00\"",
		"bad window":        "[schedule]\nsun = [\"09:00\"]",
		"empty window":      "[schedule]\nsun = [\"09:00-09:00\"]",
		"no hours":          "[schedule]\nsun = []",
	} {
		if _, err := parseShow([]byte(input), defaultShow); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestScheduleAwake(t *testing.T) {
	// 2026-10-25 is a Sunday.
	at := func(day int, hhmm string) time.Time {
		tm, _ := time.Parse("15:04", hhmm)
		return time.Date(2026, 10, 25+day, tm.Hour(), tm.Minute(), 0, 0, time.Local)
	}
	const sun, mon, fri, sat = 0, 1, 5, 6
	for input, want := range map[string]map[time.Time]bool{
		"[schedule]\non = \"07:00\"\noff = \"22:00\"": {
			at(mon, "06:59"): false, at(mon, "07:00"): true, at(sat, "21:59"): true, at(sat, "22:00"): false,
		},
		"[schedule]\non = \"18:00\"\noff = \"02:00\"": {
			at(mon, "17:59"): false, at(mon, "18:00"): true, at(sun, "00:30"): true, at(mon, "02:00"): false,
		},
		"[schedule]\nsun = [\"09:00-18:00\"]\nfri = [\"07:00-12:00\", \"20:00-01:00\"]": {
			at(sun, "08:59"): false, at(sun, "09:00"): true, at(sun, "17:59"): true, at(sun, "18:00"): false,
			at(mon, "12:00"): false,
			at(fri, "08:00"): true, at(fri, "13:00"): false, at(fri, "23:00"): true,
			at(sat, "00:30"): true, at(sat, "01:00"): false, at(sat, "10:00"): false,
		},
		"": {at(mon, "03:00"): true}, // no schedule means always on
	} {
		s, err := parseShow([]byte(input), defaultShow)
		if err != nil {
			t.Fatal(err)
		}
		for tm, on := range want {
			if got := s.Schedule.awake(tm); got != on {
				t.Errorf("%q at %s: awake = %v, want %v", input, tm.Format("Mon 15:04"), got, on)
			}
		}
	}
}

func TestScheduleString(t *testing.T) {
	for input, want := range map[string]string{
		"on = \"07:00\"\noff = \"22:00\"":                                   "daily 07:00-22:00",
		"fri = [\"17:00-22:00\"]\nsun = [\"09:00-12:00\", \"13:00-18:00\"]": "Sun 09:00-12:00 13:00-18:00, Fri 17:00-22:00",
	} {
		s, err := parseShow([]byte("[schedule]\n"+input), defaultShow)
		if err != nil {
			t.Fatal(err)
		}
		if got := s.Schedule.String(); got != want {
			t.Errorf("%q: got %q, want %q", input, got, want)
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
	if got := s.Schedule.String(); got != "daily 07:00-22:00" {
		t.Errorf("schedule = %s", got)
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

	weekly, err := decodeSetupCode(encode(`{"source":"/x","show":{"schedule":{"sun":["09:00-18:00"],"fri":["17:00-22:00"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := weekly.Show.Schedule.String(); got != "Sun 09:00-18:00, Fri 17:00-22:00" {
		t.Errorf("weekly schedule = %s", got)
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
		"unknown day":  encode(`{"source":"/x","show":{"schedule":{"sunday":["09:00-17:00"]}}}`),
	} {
		if _, err := decodeSetupCode(input); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestParseShowOverBase(t *testing.T) {
	base := Show{ImageSeconds: 20, Mute: false, Fit: "cover", Durations: map[string]int{"a.jpg": 5},
		Schedule: daily(7*60, 22*60)}

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

// daily is a schedule with the same window every day.
func daily(on, off clock) *Schedule {
	var s Schedule
	for day := range s {
		s[day] = []window{{on, off}}
	}
	return &s
}

func TestDecodeConfigCode(t *testing.T) {
	encode := func(json string) string { return base64.StdEncoding.EncodeToString([]byte(json)) }

	s, err := decodeConfigCode(encode(`{"image_seconds":5,"fit":"cover","schedule":{"sun":["09:00-18:00"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	// Settings the code leaves out keep their defaults.
	if s.ImageSeconds != 5 || s.Fit != "cover" || !s.Mute {
		t.Errorf("got %+v", s)
	}
	if got := s.Schedule.String(); got != "Sun 09:00-18:00" {
		t.Errorf("schedule = %s", got)
	}

	for name, input := range map[string]string{
		"not base64":  "!!!",
		"unknown key": encode(`{"script":"evil.lua"}`),
		"bad fit":     encode(`{"fit":"stretch"}`),
		"a device":    encode(`{"source":"smb://nas/screens"}`),
	} {
		if _, err := decodeConfigCode(input); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
