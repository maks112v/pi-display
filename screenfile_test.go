package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApplyScreenFile(t *testing.T) {
	dir := t.TempDir()
	applied := filepath.Join(dir, "applied")
	override := filepath.Join(dir, "override")
	now := time.Date(2026, 10, 28, 19, 0, 0, 0, time.Local)
	save := func(command string, at time.Time) {
		t.Helper()
		path := filepath.Join(dir, screenFile)
		if err := os.WriteFile(path, []byte(command+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	apply := func() string { return applyScreenFile(dir, applied, override, now) }

	if msg := apply(); msg != "" {
		t.Errorf("no screen.txt: got %q", msg)
	}

	// Saved 10 minutes ago: this Pi gets the 20 minutes that are left, once.
	save("off 30m", now.Add(-10*time.Minute))
	if msg := apply(); !strings.Contains(msg, "Screen off for 30m") {
		t.Errorf("first apply: got %q", msg)
	}
	if o := readOverride(override); o.On || !o.Until.Equal(now.Add(20*time.Minute)) {
		t.Errorf("override = %+v, want off until %v", o, now.Add(20*time.Minute))
	}
	if msg := apply(); msg != "" {
		t.Errorf("second apply of the same version: got %q", msg)
	}

	// A local `pi-display on` afterwards isn't undone by the old file.
	if err := writeOverride(override, true, now.Add(time.Hour), false); err != nil {
		t.Fatal(err)
	}
	if apply(); !readOverride(override).On {
		t.Error("an already-applied screen.txt replaced a newer local override")
	}

	// Saving the same command again sends it again.
	save("off 30m", now)
	if apply(); readOverride(override).On {
		t.Error("re-saved screen.txt wasn't applied")
	}

	// A command whose time is already up does nothing.
	save("on 2h", now.Add(-3*time.Hour))
	if msg := apply(); !strings.Contains(msg, "skipping") || readOverride(override).On {
		t.Errorf("expired command: got %q, override %+v", msg, readOverride(override))
	}

	// on 0 cancels, and a typo is reported without touching the override.
	save("on 0", now)
	if apply(); readOverride(override).active(now) {
		t.Error("on 0 didn't clear the override")
	}
	save("of 30m", now.Add(time.Second))
	if msg := apply(); !strings.Contains(msg, "ignoring") {
		t.Errorf("typo: got %q", msg)
	}
}
