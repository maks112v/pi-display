package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"
)

// override is a `pi-display on` or `pi-display off` in effect: the screen is
// held on (or off) until Until, whatever the schedule says.
type override struct {
	On    bool
	Until time.Time
}

func (o override) active(now time.Time) bool { return now.Before(o.Until) }

// screenOn reports whether the screen should be on at now: the override while
// it lasts, otherwise the schedule.
func (o override) screenOn(s *Schedule, now time.Time) bool {
	if o.active(now) {
		return o.On
	}
	return s.awake(now)
}

// setOverride handles `pi-display on|off [duration]`: the screen stays on or
// off for that long (default 1h), whatever the schedule says. A duration of 0
// cancels either.
func setOverride(path string, on bool, arg string) error {
	d := time.Hour
	if arg == "" {
		arg = "1h"
	} else {
		var err error
		if d, err = time.ParseDuration(arg); err != nil || d < 0 {
			return fmt.Errorf("%q is not a duration like 90m or 2h", arg)
		}
	}
	if d == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		fmt.Println("Override cleared. Back on schedule.")
		return nil
	}

	state := "off"
	if on {
		state = "on"
	}
	until := time.Now().Add(d).Truncate(time.Second)
	tmp := path + ".new"
	if err := os.WriteFile(tmp, []byte(state+" "+until.Format(time.RFC3339)+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	fmt.Printf("Screen %s for %s, until %s, then back on schedule.\n", state, arg, until.Format("Mon 15:04"))
	return nil
}

// watchOverride signals on the returned channel when a `pi-display on|off`
// changes the override at path. It checks every second, so the slideshow can
// act on the command right away instead of after the current file.
func watchOverride(ctx context.Context, path string) <-chan struct{} {
	changed := make(chan struct{}, 1)
	go func() {
		last := readOverride(path)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			if o := readOverride(path); o != last {
				last = o
				select {
				case changed <- struct{}{}:
				default: // a signal is already waiting
				}
			}
		}
	}()
	return changed
}

// readOverride returns the override saved at path, or the zero override
// (never active) if there is none. An unreadable file counts as none.
func readOverride(path string) override {
	data, err := os.ReadFile(path)
	if err != nil {
		return override{}
	}
	state, at, _ := strings.Cut(strings.TrimSpace(string(data)), " ")
	until, err := time.Parse(time.RFC3339, at)
	if err != nil || (state != "on" && state != "off") {
		return override{}
	}
	return override{On: state == "on", Until: until}
}
