package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
// because it holds the SMB password.
type Device struct {
	// Source is smb://host/share[/path], a public Google Drive folder link, or
	// an absolute local folder.
	Source   string `json:"source"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func loadDevice(path string) (Device, error) {
	var d Device
	data, err := os.ReadFile(path)
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return d, err
	}
	if d.Source == "" {
		return d, errors.New("source is empty")
	}
	return d, nil
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

// Show is the slideshow config read from display.toml in the media folder.
// The folder may come from a share other people can write to, so the file is
// parsed strictly: unknown keys or out-of-range values reject the whole file,
// and nothing in it is ever passed through to mpv.
type Show struct {
	ImageSeconds int            `toml:"image_seconds"`
	Mute         bool           `toml:"mute"`
	Fit          string         `toml:"fit"`
	Durations    map[string]int `toml:"durations"` // per-image seconds, keyed by filename
	Schedule     *Schedule      `toml:"schedule"`  // nil means always on
}

// Schedule is the daily window the screen is on, in the Pi's local time. An
// off time earlier than the on time spans midnight.
type Schedule struct {
	On  clock `toml:"on"`
	Off clock `toml:"off"`
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

func parseShow(data []byte) (Show, error) {
	s := defaultShow
	md, err := toml.Decode(string(data), &s)
	if err != nil {
		return defaultShow, err
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return defaultShow, fmt.Errorf("unknown key %q", undecoded[0].String())
	}
	if err := checkSeconds("image_seconds", s.ImageSeconds); err != nil {
		return defaultShow, err
	}
	for name, seconds := range s.Durations {
		if err := checkSeconds("durations."+name, seconds); err != nil {
			return defaultShow, err
		}
	}
	if s.Fit != "contain" && s.Fit != "cover" {
		return defaultShow, fmt.Errorf(`fit must be "contain" or "cover", got %q`, s.Fit)
	}
	if s.Schedule != nil {
		if !md.IsDefined("schedule", "on") || !md.IsDefined("schedule", "off") {
			return defaultShow, errors.New("schedule needs both on and off")
		}
		if s.Schedule.On == s.Schedule.Off {
			return defaultShow, errors.New("schedule on and off must differ")
		}
	}
	return s, nil
}

func checkSeconds(key string, v int) error {
	if v < 1 || v > maxSeconds {
		return fmt.Errorf("%s must be between 1 and %d, got %d", key, maxSeconds, v)
	}
	return nil
}

// loadShow reads display.toml from dir, falling back to defaults when absent.
func loadShow(dir string) (Show, error) {
	f, err := os.Open(filepath.Join(dir, showFile))
	if errors.Is(err, fs.ErrNotExist) {
		return defaultShow, nil
	}
	if err != nil {
		return defaultShow, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxShowSize+1))
	if err != nil {
		return defaultShow, err
	}
	if len(data) > maxShowSize {
		return defaultShow, fmt.Errorf("%s is larger than %d bytes", showFile, maxShowSize)
	}
	return parseShow(data)
}

func (s Show) imageDuration(name string) time.Duration {
	if seconds, ok := s.Durations[name]; ok {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(s.ImageSeconds) * time.Second
}
