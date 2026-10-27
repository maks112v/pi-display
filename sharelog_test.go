package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// fakeShare records appends, failing while down is set.
type fakeShare struct {
	files map[string]string
	down  bool
}

func (f *fakeShare) appendFile(name string, data []byte, _ int64) error {
	if f.down {
		return errors.New("share offline")
	}
	f.files[name] += string(data)
	return nil
}

func TestShareLogName(t *testing.T) {
	for host, want := range map[string]string{
		"lobby-tv":     "lobby-tv.pi-display.log",
		"Lobby TV.lan": "Lobby-TV-lan.pi-display.log",
		"../../etc":    "etc.pi-display.log",
		"":             "unknown.pi-display.log",
	} {
		if got := shareLogName(host); got != want {
			t.Errorf("shareLogName(%q) = %q, want %q", host, got, want)
		}
	}
}

func TestShareLogFlush(t *testing.T) {
	share := &fakeShare{files: map[string]string{}, down: true}
	logs := newShareLog("lobby")

	fmt.Fprintln(logs, "sync: share offline")
	logs.flush(share) // fails, so the line is kept
	fmt.Fprintln(logs, "sync: recovered")
	share.down = false
	logs.flush(share)

	got := share.files["lobby.pi-display.log"]
	if !strings.Contains(got, "sync: share offline\n") || !strings.HasSuffix(got, "sync: recovered\n") {
		t.Errorf("log = %q, want both lines in order", got)
	}

	// Nothing new means nothing written.
	logs.flush(share)
	if share.files["lobby.pi-display.log"] != got {
		t.Error("flush with no new lines changed the file")
	}
}

func TestShareLogDropsOldest(t *testing.T) {
	share := &fakeShare{files: map[string]string{}}
	logs := newShareLog("lobby")
	for i := range maxPendingLines + 3 {
		fmt.Fprintf(logs, "line %d\n", i)
	}
	logs.flush(share)

	got := share.files["lobby.pi-display.log"]
	if !strings.HasPrefix(got, "(3 earlier lines dropped") || strings.Contains(got, "line 2\n") || !strings.Contains(got, "line 3\n") {
		t.Errorf("log starts %q", got[:min(len(got), 120)])
	}
}
