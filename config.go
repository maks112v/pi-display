package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	devicePath = "/etc/pi-display/config.json"
	cacheDir   = "/var/lib/pi-display/media"
	socketPath = "/run/pi-display/mpv.sock"
	// wakePath holds the time a `pi-display on` override ends. It lives in
	// the state directory so an override survives a reboot mid-event.
	wakePath = "/var/lib/pi-display/awake-until"

	// showFile is the optional slideshow config inside the media folder.
	showFile    = "display.toml"
	maxShowSize = 64 << 10
)

// Device is the per-Pi config written by `pi-display setup`. The file is 0600
// because it holds the SMB password. It is also the payload of a setup code
// from the web config generator, base64-encoded.
type Device struct {
	// Source is smb://host/share[/path], a public Google Drive folder link, or
	// an absolute local folder.
	Source   string `json:"source"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// Show is this screen's slideshow settings. nil means defaults. A
	// display.toml in the folder is applied on top.
	Show *Show `json:"show,omitempty"`
}

func loadDevice(path string) (Device, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Device{}, err
	}
	return parseDevice(data)
}

// parseDevice decodes and checks a Device. Unknown keys are rejected so a
// typo'd setup code fails loudly instead of being half applied.
func parseDevice(data []byte) (Device, error) {
	var d Device
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, err
	}
	if d.Source == "" {
		return d, errors.New("source is empty")
	}
	if d.Show != nil {
		if err := d.Show.validate(); err != nil {
			return d, fmt.Errorf("show: %w", err)
		}
	}
	return d, nil
}

// baseShow is what display.toml is layered over.
func (d Device) baseShow() Show {
	if d.Show == nil {
		return defaultShow
	}
	return *d.Show
}

func saveDevice(path string, d Device, uid, gid int) error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Chown(path, uid, gid)
}

// Show is the slideshow config. It comes from the device config (set by a
// setup code) with display.toml in the media folder layered on top. The
// folder may come from a share other people can write to, so the file is
// parsed strictly: unknown keys or out-of-range values reject the whole file,
// and nothing in it is ever passed through to mpv.
type Show struct {
	ImageSeconds int            `toml:"image_seconds" json:"image_seconds"`
	Mute         bool           `toml:"mute" json:"mute"`
	Fit          string         `toml:"fit" json:"fit"`
	Shuffle      bool           `toml:"shuffle" json:"shuffle"`
	Durations    map[string]int `toml:"durations" json:"durations,omitempty"` // per-image seconds, keyed by filename
	Schedule     *Schedule      `toml:"schedule" json:"schedule,omitempty"`   // nil means always on
	TVPower      bool           `toml:"tv_power" json:"tv_power"`             // turn the TV off and on over HDMI-CEC with the schedule
}

// UnmarshalJSON starts from the defaults so a setup code only needs the
// settings it changes, and rejects unknown keys like display.toml does.
func (s *Show) UnmarshalJSON(data []byte) error {
	type plain Show // drops this method so Decode doesn't recurse
	v := plain(defaultShow)
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	*s = Show(v)
	return nil
}

// validate checks everything parseShow can't express as types.
func (s Show) validate() error {
	if err := checkSeconds("image_seconds", s.ImageSeconds); err != nil {
		return err
	}
	for name, seconds := range s.Durations {
		if err := checkSeconds("durations."+name, seconds); err != nil {
			return err
		}
	}
	if s.Fit != "contain" && s.Fit != "cover" {
		return fmt.Errorf(`fit must be "contain" or "cover", got %q`, s.Fit)
	}
	return nil
}

// Schedule is when the screen is on, in the Pi's local time: a list of
// windows for each day, indexed by time.Weekday. A window whose off time is
// earlier than its on time runs past midnight into the next day. It is
// written either as one window for every day:
//
//	on = "07:00"
//	off = "22:00"
//
// or per weekday, with days left out staying off:
//
//	sun = ["09:00-18:00"]
//	fri = ["07:00-12:00", "17:00-22:00"]
type Schedule [7][]window

// window is one stretch of on time, written "HH:MM-HH:MM".
type window struct{ On, Off clock }

var dayKeys = [7]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// UnmarshalTOML and UnmarshalJSON share parseSchedule, so display.toml and
// setup codes accept exactly the same shapes.
func (s *Schedule) UnmarshalTOML(data any) error {
	m, ok := data.(map[string]any)
	if !ok {
		return errors.New("schedule must be a table")
	}
	v, err := parseSchedule(m)
	*s = v
	return err
}

func (s *Schedule) UnmarshalJSON(data []byte) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	v, err := parseSchedule(m)
	*s = v
	return err
}

// MarshalJSON writes the per-day form, which round-trips through
// UnmarshalJSON whichever form was read.
func (s Schedule) MarshalJSON() ([]byte, error) {
	m := map[string][]string{}
	for day, windows := range s {
		for _, w := range windows {
			m[dayKeys[day]] = append(m[dayKeys[day]], w.String())
		}
	}
	return json.Marshal(m)
}

func parseSchedule(m map[string]any) (Schedule, error) {
	var s Schedule
	on, hasOn := m["on"]
	off, hasOff := m["off"]
	if hasOn || hasOff {
		if !hasOn || !hasOff {
			return s, errors.New("schedule needs both on and off")
		}
		if len(m) > 2 {
			return s, errors.New("schedule takes on and off, or days, not both")
		}
		var w window
		if err := w.On.parse(on); err != nil {
			return s, err
		}
		if err := w.Off.parse(off); err != nil {
			return s, err
		}
		if w.On == w.Off {
			return s, errors.New("schedule on and off must differ")
		}
		for day := range s {
			s[day] = []window{w}
		}
		return s, nil
	}

	for key, v := range m {
		day := slices.Index(dayKeys[:], key)
		if day < 0 {
			return s, fmt.Errorf("unknown schedule key %q, want on, off, or %s", key, strings.Join(dayKeys[:], ", "))
		}
		list, ok := v.([]any)
		if !ok {
			return s, fmt.Errorf(`schedule.%s must be a list like ["09:00-17:00"]`, key)
		}
		for _, item := range list {
			text, ok := item.(string)
			if !ok {
				return s, fmt.Errorf(`schedule.%s: %v is not a quoted window like "09:00-17:00"`, key, item)
			}
			w, err := parseWindow(text)
			if err != nil {
				return s, fmt.Errorf("schedule.%s: %w", key, err)
			}
			s[day] = append(s[day], w)
		}
	}
	if !slices.ContainsFunc(s[:], func(ws []window) bool { return len(ws) > 0 }) {
		return s, errors.New("schedule has no hours; leave it out to stay on all the time")
	}
	return s, nil
}

func parseWindow(text string) (window, error) {
	var w window
	on, off, ok := strings.Cut(text, "-")
	if !ok {
		return w, fmt.Errorf("%q is not a window like 09:00-17:00", text)
	}
	if err := w.On.UnmarshalText([]byte(on)); err != nil {
		return w, err
	}
	if err := w.Off.UnmarshalText([]byte(off)); err != nil {
		return w, err
	}
	if w.On == w.Off {
		return w, fmt.Errorf("%q starts and ends at the same time", text)
	}
	return w, nil
}

func (w window) String() string { return w.On.String() + "-" + w.Off.String() }

// String summarizes the schedule for `pi-display status`, like
// "daily 07:00-22:00" or "Sun 09:00-18:00, Fri 17:00-22:00".
func (s Schedule) String() string {
	if !slices.ContainsFunc(s[1:], func(ws []window) bool { return !slices.Equal(ws, s[0]) }) {
		return "daily " + joinWindows(s[0])
	}
	var days []string
	for day, windows := range s {
		if len(windows) > 0 {
			days = append(days, time.Weekday(day).String()[:3]+" "+joinWindows(windows))
		}
	}
	return strings.Join(days, ", ")
}

func joinWindows(ws []window) string {
	parts := make([]string, len(ws))
	for i, w := range ws {
		parts[i] = w.String()
	}
	return strings.Join(parts, " ")
}

// clock is a time of day in minutes after midnight, written "HH:MM".
type clock int

func (c *clock) UnmarshalText(text []byte) error {
	t, err := time.Parse("15:04", string(text))
	if err != nil {
		return fmt.Errorf("%q is not a 24-hour HH:MM time", text)
	}
	*c = clock(t.Hour()*60 + t.Minute())
	return nil
}

// parse reads a clock from a decoded TOML or JSON value, which must be a string.
func (c *clock) parse(v any) error {
	text, ok := v.(string)
	if !ok {
		return fmt.Errorf(`%v is not a quoted time like "07:00"`, v)
	}
	return c.UnmarshalText([]byte(text))
}

func (c clock) String() string {
	return fmt.Sprintf("%02d:%02d", c/60, c%60)
}

// awake reports whether the schedule has the screen on at t: inside one of
// today's windows, or the overnight tail of one of yesterday's.
func (s *Schedule) awake(t time.Time) bool {
	if s == nil {
		return true
	}
	now := clock(t.Hour()*60 + t.Minute())
	today := t.Weekday()
	for _, w := range s[today] {
		if w.On <= now && (now < w.Off || w.Off < w.On) {
			return true
		}
	}
	for _, w := range s[(today+6)%7] {
		if w.Off < w.On && now < w.Off {
			return true
		}
	}
	return false
}

var defaultShow = Show{ImageSeconds: 10, Mute: true, Fit: "contain", TVPower: true}

const maxSeconds = 24 * 60 * 60

// parseShow applies display.toml over base: keys the file sets win, the rest
// keep base's values, and [durations] entries add to base's. On error it
// returns base unchanged.
func parseShow(data []byte, base Show) (Show, error) {
	s := base
	// Copy what Decode would write through, so base is never modified.
	s.Durations = maps.Clone(base.Durations)
	s.Schedule = nil
	md, err := toml.Decode(string(data), &s)
	if err != nil {
		return base, err
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return base, fmt.Errorf("unknown key %q", undecoded[0].String())
	}
	if s.Schedule == nil {
		s.Schedule = base.Schedule
	}
	if err := s.validate(); err != nil {
		return base, err
	}
	return s, nil
}

func checkSeconds(key string, v int) error {
	if v < 1 || v > maxSeconds {
		return fmt.Errorf("%s must be between 1 and %d, got %d", key, maxSeconds, v)
	}
	return nil
}

// loadShow applies display.toml from dir over base, or returns base when the
// folder has none.
func loadShow(dir string, base Show) (Show, error) {
	f, err := os.Open(filepath.Join(dir, showFile))
	if errors.Is(err, fs.ErrNotExist) {
		return base, nil
	}
	if err != nil {
		return base, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxShowSize+1))
	if err != nil {
		return base, err
	}
	if len(data) > maxShowSize {
		return base, fmt.Errorf("%s is larger than %d bytes", showFile, maxShowSize)
	}
	return parseShow(data, base)
}

func (s Show) imageDuration(name string) time.Duration {
	if seconds, ok := s.Durations[name]; ok {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(s.ImageSeconds) * time.Second
}
