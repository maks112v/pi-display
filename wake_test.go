package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSetOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "override")
	if readOverride(path).active(time.Now()) {
		t.Fatal("expected no override before setOverride")
	}

	for _, on := range []bool{true, false} {
		if err := setOverride(path, on, "90m"); err != nil {
			t.Fatal(err)
		}
		o := readOverride(path)
		if left := time.Until(o.Until); o.On != on || left < 89*time.Minute || left > 90*time.Minute {
			t.Errorf("setOverride(on=%v): got %+v, want on=%v ending in about 90m", on, o, on)
		}
	}

	if err := setOverride(path, true, "0"); err != nil {
		t.Fatal(err)
	}
	if readOverride(path).active(time.Now()) {
		t.Error("expected override cleared")
	}

	for _, bad := range []string{"soon", "-1h"} {
		if err := setOverride(path, false, bad); err == nil {
			t.Errorf("setOverride(%q): expected error", bad)
		}
	}
}
