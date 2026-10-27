package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDRMOutput(t *testing.T) {
	// Pi 5 layout: card0 has no outputs, the screen is on card1's first HDMI.
	dir := t.TempDir()
	for name, status := range map[string]string{
		"card1-HDMI-A-1":    "connected\n",
		"card1-HDMI-A-2":    "disconnected\n",
		"card1-Writeback-1": "unknown\n",
	} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "status"), []byte(status), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "card0"), 0o755); err != nil {
		t.Fatal(err)
	}

	device, connector, ok := drmOutput(dir)
	if !ok || device != "/dev/dri/card1" || connector != "HDMI-A-1" {
		t.Errorf("got (%q, %q, %v), want (/dev/dri/card1, HDMI-A-1, true)", device, connector, ok)
	}
	if _, _, ok := drmOutput(t.TempDir()); ok {
		t.Error("found an output in an empty dir")
	}
}
