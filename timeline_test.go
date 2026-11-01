package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// plays walks the timeline from from for n turns.
func plays(names []string, length func(string) time.Duration, shuffle bool, from time.Time, n int) []slot {
	var out []slot
	at := from
	for range n {
		s, _ := slotAt(names, length, shuffle, at)
		out = append(out, s)
		at = s.end
	}
	return out
}

func TestSlotAt(t *testing.T) {
	lengths := map[string]time.Duration{"10.mp4": 3 * time.Second, "2.jpg": 5 * time.Second, "1.jpg": 2 * time.Second, "broken.mp4": 0}
	length := func(name string) time.Duration { return lengths[name] }
	names := []string{"10.mp4", "2.jpg", "1.jpg", "broken.mp4"}

	// The loop is 10s long and starts on a multiple of 10s from the epoch.
	at := time.Unix(1_000_000_003, 0)
	s, ok := slotAt(names, length, false, at)
	if !ok || s.name != "2.jpg" || !s.start.Equal(time.Unix(1_000_000_002, 0)) || !s.end.Equal(time.Unix(1_000_000_007, 0)) {
		t.Fatalf("slotAt = %+v, %v; want 2.jpg from :02 to :07", s, ok)
	}

	var got []string
	for _, s := range plays(names, length, false, at, 5) {
		got = append(got, s.name)
	}
	if want := []string{"2.jpg", "10.mp4", "1.jpg", "2.jpg", "10.mp4"}; !slices.Equal(got, want) {
		t.Errorf("plays = %v, want %v", got, want)
	}

	if _, ok := slotAt([]string{"broken.mp4"}, length, false, at); ok {
		t.Error("found a slot with nothing playable")
	}
}

func TestShuffledLoops(t *testing.T) {
	names := []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg", "f.jpg", "g.jpg", "h.jpg"}
	length := func(string) time.Duration { return time.Second }

	var loops [][]string
	for _, s := range plays(names, length, true, time.Unix(800, 0), 8*20) {
		if s.start.Unix()%8 == 0 {
			loops = append(loops, nil)
		}
		loops[len(loops)-1] = append(loops[len(loops)-1], s.name)
	}
	for i, loop := range loops {
		if !slices.Equal(slices.Sorted(slices.Values(loop)), names) {
			t.Fatalf("loop %d = %v, want each file once", i, loop)
		}
		if i > 0 && loop[0] == loops[i-1][len(names)-1] {
			t.Fatalf("%s played twice in a row across loops %d and %d", loop[0], i-1, i)
		}
	}
	if slices.EqualFunc(loops[1:], loops[:len(loops)-1], slices.Equal) {
		t.Error("every loop had the same order; expected a reshuffle")
	}
}

func TestSettled(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for name, saved := range map[string]time.Time{
		"old.jpg":    now.Add(-2 * settleTime),
		"new.jpg":    now.Add(-settleTime / 2),
		"future.jpg": now.Add(time.Hour), // the source's clock is ahead
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, saved, saved); err != nil {
			t.Fatal(err)
		}
	}

	if got, want := settled(dir, []string{"future.jpg", "new.jpg", "old.jpg"}, settleTime, now), []string{"future.jpg", "old.jpg"}; !slices.Equal(got, want) {
		t.Errorf("settled = %v, want %v", got, want)
	}
	if got := settled(dir, []string{"new.jpg"}, settleTime, now); !slices.Equal(got, []string{"new.jpg"}) {
		t.Errorf("settled = %v, want the new file when nothing else can play", got)
	}
}

func TestVideoLengthsProbeOnce(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.mp4", "bad.mp4"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	probes := 0
	v := newVideoLengths()
	v.probe = func(_ context.Context, path string) (time.Duration, error) {
		probes++
		if filepath.Base(path) == "bad.mp4" {
			return 0, errors.New("unreadable")
		}
		return 4 * time.Second, nil
	}

	for range 3 {
		if d := v.length(t.Context(), dir, "a.mp4"); d != 4*time.Second {
			t.Fatalf("length(a.mp4) = %v, want 4s", d)
		}
		if d := v.length(t.Context(), dir, "bad.mp4"); d != 0 {
			t.Fatalf("length(bad.mp4) = %v, want 0", d)
		}
	}
	if probes != 2 {
		t.Errorf("probed %d times, want once per file", probes)
	}

	// Replacing a file measures it again.
	if err := os.WriteFile(filepath.Join(dir, "a.mp4"), []byte("longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	v.length(t.Context(), dir, "a.mp4")
	if probes != 3 {
		t.Errorf("probed %d times after replacing a.mp4, want 3", probes)
	}
}
