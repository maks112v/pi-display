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
	if s.Schedule != nil && s.Schedule.On == s.Schedule.Off {
		return errors.New("schedule on and off must differ")
	}
	return nil
}

// Schedule is the daily window the screen is on, in the Pi's local time. An
// off time earlier than the on time spans midnight.
type Schedule struct {
	On  clock `toml:"on" json:"on"`
	Off clock `toml:"off" json:"off"`
}

// UnmarshalJSON requires both times, matching parseShow's check for TOML.
func (s *Schedule) UnmarshalJSON(data []byte) error {
	var v struct {
		On  *clock `json:"on"`
		Off *clock `json:"off"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	if v.On == nil || v.Off == nil {
		return errors.New("schedule needs both on and off")
	}
	*s = Schedule{On: *v.On, Off: *v.Off}
	return nil
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

func (c clock) MarshalText() ([]byte, error) {
	return []byte(c.String()), nil
}

func (c clock) String() string {
	return fmt.Sprintf("%02d:%02d", c/60, c%60)
}

// awake reports whether the schedule has the screen on at t.
func (s *Schedule) awake(t time.Time) bool {
	if s == nil {
		return true
	}
	now := clock(t.Hour()*60 + t.Minute())
	if s.On < s.Off {
		return s.On <= now && now < s.Off
	}
	return now >= s.On || now < s.Off
}

var defaultShow = Show{ImageSeconds: 10, Mute: true, Fit: "contain"}

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
	} else if !md.IsDefined("schedule", "on") || !md.IsDefined("schedule", "off") {
		return base, errors.New("schedule needs both on and off")
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
