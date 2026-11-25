package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

	if err := mirror(src, dst, plentyFree, map[string]time.Time{}, time.Time{}, time.Now()); err != nil {
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

	if err := mirror(src, dst, plentyFree, map[string]time.Time{}, time.Time{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dst, "1.jpg")); string(data) != "one, edited" {
		t.Errorf("1.jpg = %q", data)
	}
	if after, _ := os.Stat(untouched); !os.SameFile(before, after) {
		t.Error("clip.mp4 was re-downloaded")
	}
}

func TestMirrorSkipsWhatDoesNotFit(t *testing.T) {
	dst := t.TempDir()
	src := fstest.MapFS{
		"small.jpg": {Data: make([]byte, 10)},
		"huge.mp4":  {Data: make([]byte, 100)},
	}
	writeFile(t, dst, "stale.jpg", "old")

	// Only room for the small file. The stale file must be gone before any
	// download is attempted, so its space counts.
	free := func(dir string) (uint64, error) {
		if _, err := os.Stat(filepath.Join(dir, "stale.jpg")); err == nil {
			t.Error("free space checked before stale files were removed")
		}
		return diskReserve + 50, nil
	}
	err := mirror(src, dst, free, map[string]time.Time{}, time.Time{}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "skipping huge.mp4") {
		t.Errorf("err = %v, want huge.mp4 skipped", err)
	}
	if got, want := dirNames(t, dst), []string{"small.jpg"}; !slices.Equal(got, want) {
		t.Errorf("dst = %v, want %v", got, want)
	}
}

func TestMirrorDelaysDeletions(t *testing.T) {
	dst := t.TempDir()
	deleted := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	src := fstest.MapFS{
		".":        {Mode: fs.ModeDir, ModTime: deleted}, // folders change when a file is deleted
		"keep.jpg": {Data: []byte("keep")},
	}
	writeFile(t, dst, "gone.jpg", "gone")
	writeFile(t, dst, "restored.jpg", "back")
	writeFile(t, dst, ".partial-keep.jpg", "interrupted")
	removing := map[string]time.Time{}

	if err := mirror(src, dst, plentyFree, removing, time.Time{}, deleted.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	if got, want := dirNames(t, dst), []string{"gone.jpg", "keep.jpg", "restored.jpg"}; !slices.Equal(got, want) {
		t.Fatalf("dst = %v, want deleted media kept for now and the partial file removed", got)
	}
	if due := deleted.Add(settleTime); !removing["gone.jpg"].Equal(due) {
		t.Errorf("gone.jpg due %v, want %v", removing["gone.jpg"], due)
	}

	// Later changes to the folder don't move the time, and a file put back
	// is no longer due.
	src["."].ModTime = deleted.Add(40 * time.Second)
	src["restored.jpg"] = &fstest.MapFile{Data: []byte("back")}
	if err := mirror(src, dst, plentyFree, removing, time.Time{}, deleted.Add(50*time.Second)); err != nil {
		t.Fatal(err)
	}
	if due := deleted.Add(settleTime); len(removing) != 1 || !removing["gone.jpg"].Equal(due) {
		t.Errorf("removing = %v, want only gone.jpg due %v", removing, due)
	}

	if err := mirror(src, dst, plentyFree, removing, time.Time{}, deleted.Add(settleTime)); err != nil {
		t.Fatal(err)
	}
	if got, want := dirNames(t, dst), []string{"keep.jpg", "restored.jpg"}; !slices.Equal(got, want) || len(removing) != 0 {
		t.Errorf("dst = %v, removing = %v; want gone.jpg removed once due", got, removing)
	}
}

// Drive folders don't record when they changed, so a deletion is timed from
// the sync tick that noticed it, the same tick on every Pi.
func TestMirrorTimesDeletionsByTick(t *testing.T) {
	dst := t.TempDir()
	writeFile(t, dst, "gone.jpg", "gone")
	tick := time.Date(2026, 1, 1, 12, 0, 30, 0, time.UTC)
	removing := map[string]time.Time{}

	if err := mirror(fstest.MapFS{}, dst, plentyFree, removing, tick, tick.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if due := tick.Add(settleTime); !removing["gone.jpg"].Equal(due) || len(dirNames(t, dst)) != 1 {
		t.Errorf("removing = %v, dst = %v; want gone.jpg kept until %v", removing, dirNames(t, dst), due)
	}
}

func plentyFree(string) (uint64, error) { return 1 << 40, nil }

func TestMirrorEventFolders(t *testing.T) {
	dst := t.TempDir()
	now := time.Date(2026, 12, 5, 12, 0, 0, 0, time.Local)
	modTime := now.Add(-time.Hour)
	src := fstest.MapFS{
		"main.jpg":        {Data: []byte("main"), ModTime: modTime},
		"gala.event.toml": {Data: []byte("folder = \"gala\"\ndates = [\"2026-12-05\"]\nhours = [\"17:00-23:00\"]\n"), ModTime: modTime},
		"old.event.toml":  {Data: []byte("folder = \"old\"\ndates = [\"2026-11-01\"]\n"), ModTime: modTime},
		"gala/01.jpg":     {Data: []byte("one"), ModTime: modTime},
		"gala/notes.txt":  {Data: []byte("skip"), ModTime: modTime},
		"old/01.jpg":      {Data: []byte("past"), ModTime: modTime},
		"other/01.jpg":    {Data: []byte("unused"), ModTime: modTime},
	}
	writeFile(t, dst, "main.jpg", "main")
	if err := os.MkdirAll(filepath.Join(dst, "stale"), 0o755); err != nil {
		t.Fatal(err)
	}
	removing := map[string]time.Time{"stale/x.jpg": now}

	// One sync brings in the event file and its folder; ended and unused
	// folders are left out, and a folder no event uses is removed.
	if err := mirrorAll(src, dst, plentyFree, removing, time.Time{}, now); err != nil {
		t.Fatal(err)
	}
	if got, want := dirNames(t, dst), []string{"gala", "gala.event.toml", "main.jpg", "old.event.toml"}; !slices.Equal(got, want) {
		t.Fatalf("dst = %v, want %v", got, want)
	}
	if got, want := dirNames(t, filepath.Join(dst, "gala")), []string{"01.jpg"}; !slices.Equal(got, want) {
		t.Errorf("gala = %v, want %v", got, want)
	}
	if _, ok := removing["stale/x.jpg"]; ok {
		t.Error("removing still holds a file from a removed folder")
	}

	// A file deleted from the event folder waits its turn, keyed by its path.
	delete(src, "gala/01.jpg")
	src["gala/02.jpg"] = &fstest.MapFile{Data: []byte("two"), ModTime: modTime}
	if err := mirrorAll(src, dst, plentyFree, removing, now, now); err != nil {
		t.Fatal(err)
	}
	if _, ok := removing["gala/01.jpg"]; !ok {
		t.Errorf("removing = %v, want gala/01.jpg", removing)
	}

	// Once the event is over its folder goes.
	if err := mirrorAll(src, dst, plentyFree, removing, time.Time{}, now.Add(12*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "gala")); !os.IsNotExist(err) {
		t.Errorf("gala still there after the event: %v", err)
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
