package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestKeepOn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "awake-until")
	if !awakeUntil(path).IsZero() {
		t.Fatal("expected no override before keepOn")
	}

	if err := keepOn(path, "90m"); err != nil {
		t.Fatal(err)
	}
	if left := time.Until(awakeUntil(path)); left < 89*time.Minute || left > 90*time.Minute {
		t.Errorf("override ends in %v, want about 90m", left)
	}

	if err := keepOn(path, "0"); err != nil {
		t.Fatal(err)
	}
	if !awakeUntil(path).IsZero() {
		t.Error("expected override cleared")
	}

	for _, bad := range []string{"soon", "-1h"} {
		if err := keepOn(path, bad); err == nil {
			t.Errorf("keepOn(%q): expected error", bad)
		}
	}
}
