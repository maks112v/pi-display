package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// screenFile is an optional one-line command in the media folder, like
	// `off 30m` or `on 2h`, that every Pi playing the folder applies once.
	screenFile = "screen.txt"
	// screenAppliedPath records which screen.txt this Pi last acted on, so
	// each saved version runs once, across restarts too.
	screenAppliedPath = "/var/lib/pi-display/screen-applied"
	maxScreenSize     = 1 << 10
)

// watchScreenFile checks dir for a new screen.txt every second while the
// slideshow runs and turns it into an override, which watchOverride then
// picks up.
func watchScreenFile(ctx context.Context, dir string) {
	for {
		if msg := applyScreenFile(dir, screenAppliedPath, overridePath, time.Now()); msg != "" {
			log.Print(msg)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// applyScreenFile acts on dir's screen.txt if this version hasn't been seen
// before, and returns a line for the log ("" when there was nothing new). A
// version is its save time plus its text, so saving the same command again
// sends it again.
//
// The duration counts from when the file was saved, not from when this Pi
// noticed it, so every screen switches back together, a Pi that was offline
// only gets the time that's left, and one that missed the whole window does
// nothing.
func applyScreenFile(dir, appliedPath, overridePath string, now time.Time) string {
	path := filepath.Join(dir, screenFile)
	info, err := os.Stat(path)
	if err != nil || info.Size() > maxScreenSize {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	command := strings.Join(strings.Fields(string(data)), " ")
	version := info.ModTime().UTC().Format(time.RFC3339Nano) + " " + command
	if applied, _ := os.ReadFile(appliedPath); string(applied) == version {
		return ""
	}
	msg := runScreenCommand(command, info.ModTime(), overridePath, now)
	if err := os.WriteFile(appliedPath, []byte(version), 0o644); err != nil {
		msg += fmt.Sprintf(" (couldn't record it, so it may run again: %v)", err)
	}
	return msg
}

// runScreenCommand applies one `on|off [duration]` command saved at saved.
func runScreenCommand(command string, saved time.Time, overridePath string, now time.Time) string {
	fields := strings.Fields(command)
	if len(fields) == 0 || len(fields) > 2 || (fields[0] != "on" && fields[0] != "off") {
		return fmt.Sprintf("%s: ignoring %q, write on or off and a duration, like off 30m", screenFile, command)
	}
	on := fields[0] == "on"
	arg := ""
	if len(fields) == 2 {
		arg = fields[1]
	}
	d, label, err := overrideDuration(arg)
	if err != nil {
		return fmt.Sprintf("%s: ignoring %q: %v", screenFile, command, err)
	}
	// A save time in the future means the share's clock is ahead; count
	// from now instead.
	start := saved
	if start.After(now) {
		start = now
	}
	until := start.Add(d)
	if d > 0 && !until.After(now) {
		return fmt.Sprintf("%s: skipping %q, it ended at %s", screenFile, command, until.Format("Mon 15:04"))
	}
	if err := writeOverride(overridePath, on, until, d == 0); err != nil {
		return fmt.Sprintf("%s: %v", screenFile, err)
	}
	return fmt.Sprintf("%s: %s", screenFile, describeOverride(on, label, until, d == 0))
}
