package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"
	"time"
)

func TestMirror(t *testing.T) {
	dst := t.TempDir()
	modTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	src := fstest.MapFS{
		"1.jpg":        {Data: []byte("one"), ModTime: modTime},
		"clip.mp4":     {Data: []byte("video"), ModTime: modTime},
		"display.toml": {Data: []byte("image_seconds = 5"), ModTime: modTime},
		"notes.txt":    {Data: []byte("skip"), ModTime: modTime},
		".hidden.jpg":  {Data: []byte("skip"), ModTime: modTime},
		"sub/2.jpg":    {Data: []byte("skip"), ModTime: modTime},
	}
	writeFile(t, dst, "stale.jpg", "old")
	writeFile(t, dst, ".partial-1.jpg", "interrupted")

	if err := mirror(src, dst); err != nil {
		t.Fatal(err)
	}
	if got, want := dirNames(t, dst), []string{"1.jpg", "clip.mp4", "display.toml"}; !slices.Equal(got, want) {
		t.Fatalf("dst = %v, want %v", got, want)
	}

	// A changed file is re-downloaded; an unchanged one is left alone.
	src["1.jpg"] = &fstest.MapFile{Data: []byte("one, edited"), ModTime: modTime.Add(time.Hour)}
	// Downloads rename a new file into place, so an unchanged inode means no download.
	untouched := filepath.Join(dst, "clip.mp4")
	before, _ := os.Stat(untouched)

	if err := mirror(src, dst); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dst, "1.jpg")); string(data) != "one, edited" {
		t.Errorf("1.jpg = %q", data)
	}
	if after, _ := os.Stat(untouched); !os.SameFile(before, after) {
		t.Error("clip.mp4 was re-downloaded")
	}
}

func TestParseSMB(t *testing.T) {
	got, err := parseSMB("smb://nas.local/media/lobby/screen")
	if err != nil {
		t.Fatal(err)
	}
	want := smbTarget{addr: "nas.local:445", share: `\\nas.local\media`, dir: "lobby/screen"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	for _, bad := range []string{"smb://nas.local", "smb:///media", "http://nas/media"} {
		if _, err := parseSMB(bad); err == nil {
			t.Errorf("parseSMB(%q): expected error", bad)
		}
	}
}

func writeFile(t *testing.T, dir, name, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
