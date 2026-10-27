package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"time"
)

// status handles `pi-display status`: a one-screen summary for debugging a Pi
// over SSH. Each line reports its own problem instead of failing, so one
// broken piece never hides the rest.
func status() {
	row := func(label, format string, args ...any) {
		fmt.Printf("%-10s %s\n", label, fmt.Sprintf(format, args...))
	}

	row("version", "%s", version)
	// A copy run from elsewhere (like ~/pi-display) can differ from what the
	// service runs.
	if exe, _ := os.Executable(); exe != binPath {
		installed := "not installed"
		if out, err := exec.Command(binPath, "version").Output(); err == nil {
			installed = strings.TrimSpace(string(out))
		}
		row("installed", "%s (%s)", installed, binPath)
	}

	switch svc := systemdUnit("pi-display"); {
	case svc == nil:
		row("service", "unknown, systemctl is unavailable")
	case svc["LoadState"] == "not-found":
		row("service", "not installed, run `sudo pi-display setup`")
	default:
		state := fmt.Sprintf("%s (%s), %s", svc["ActiveState"], svc["SubState"], svc["UnitFileState"])
		if since := svc["ActiveEnterTimestamp"]; svc["ActiveState"] == "active" && since != "" {
			state += ", since " + since
		}
		if n := svc["NRestarts"]; n != "" && n != "0" {
			state += ", restarted " + n + " times (see pi-display logs)"
		}
		row("service", "%s", state)
	}
	if dm := systemdUnit("display-manager"); dm["ActiveState"] == "active" {
		row("desktop", "running, so the slideshow can't take the screen (re-run setup)")
	}

	device, err := loadDevice(devicePath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		row("source", "not set up, run `sudo pi-display setup`")
		return
	case errors.Is(err, fs.ErrPermission):
		row("source", "can't read %s, run with sudo", devicePath)
		return
	case err != nil:
		row("source", "bad %s: %v", devicePath, err)
		return
	}
	source := device.Source
	if device.Username != "" {
		source += " as " + device.Username
	}
	row("source", "%s", source)

	dir := device.Source
	if isRemote(device.Source) {
		dir = cacheDir
	}
	if names, err := listMedia(dir); err != nil {
		row("media", "%v", err)
	} else {
		row("media", "%d files in %s", len(names), dir)
	}

	show, err := loadShow(dir, device.baseShow())
	if err != nil {
		row("show", "%s ignored: %v", showFile, err)
	}
	row("screen", "%s", screenState(show.Schedule, readOverride(overridePath), time.Now()))
}

// screenState describes whether the screen should be on at now and why.
func screenState(s *Schedule, o override, now time.Time) string {
	switch {
	case o.active(now) && o.On:
		return "on until " + o.Until.Format("Mon 15:04") + " (pi-display on)"
	case o.active(now):
		return "off until " + o.Until.Format("Mon 15:04") + " (pi-display off)"
	case s == nil:
		return "on, no schedule"
	case s.awake(now):
		return "on, schedule " + s.String()
	default:
		return "off, schedule " + s.String()
	}
}

// systemdUnit returns a few properties of a systemd unit, or nil when
// systemctl can't be run.
func systemdUnit(name string) map[string]string {
	out, err := exec.Command("systemctl", "show", name,
		"--property=LoadState,ActiveState,SubState,UnitFileState,ActiveEnterTimestamp,NRestarts").Output()
	if err != nil {
		return nil
	}
	props := map[string]string{}
	for line := range strings.Lines(string(out)) {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			props[k] = v
		}
	}
	return props
}
