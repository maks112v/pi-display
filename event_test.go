package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func mustEvent(t *testing.T, name, text string) Event {
	t.Helper()
	e, err := parseEvent(name, []byte(text), defaultShow, time.UTC)
	if err != nil {
		t.Fatalf("parseEvent(%s): %v", name, err)
	}
	return e
}

func at(day, hour, minute int) time.Time {
	return time.Date(2026, 12, day, hour, minute, 0, 0, time.UTC)
}

func TestParseEvent(t *testing.T) {
	base := defaultShow
	base.ImageSeconds = 20
	base.Durations = map[string]int{"intro.jpg": 60}
	e, err := parseEvent("gala.event.toml", []byte(`
folder = "gala"
dates = ["2026-12-05", "2026-12-06"]
hours = ["17:00-18:00", "20:00-01:00"]
shuffle = true
[durations]
"01.jpg" = 30
`), base, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if e.Folder != "gala" || !e.Show.Shuffle || e.Show.ImageSeconds != 20 || e.Show.Fit != "contain" {
		t.Errorf("show = %+v, want display.toml's settings with shuffle on", e.Show)
	}
	if _, ok := e.Show.Durations["intro.jpg"]; ok || e.Show.Durations["01.jpg"] != 30 {
		t.Errorf("durations = %v, want only the event's own", e.Show.Durations)
	}
	want := []span{
		{at(5, 17, 0), at(5, 18, 0)},
		{at(5, 20, 0), at(6, 1, 0)},
		{at(6, 17, 0), at(6, 18, 0)},
		{at(6, 20, 0), at(7, 1, 0)},
	}
	if !slices.Equal(e.windows, want) {
		t.Errorf("windows = %v, want %v", e.windows, want)
	}

	// No hours means the whole day.
	whole := mustEvent(t, "a.event.toml", `folder = "a"
dates = ["2026-12-05"]`)
	if want := []span{{at(5, 0, 0), at(6, 0, 0)}}; !slices.Equal(whole.windows, want) {
		t.Errorf("whole day = %v, want %v", whole.windows, want)
	}
}

// What the event generator on the website downloads (marketing/src/lib/event.ts).
func TestParseGeneratedEvent(t *testing.T) {
	e := mustEvent(t, "gala.event.toml", `folder = "gala"
dates = ["2026-12-05", "2026-12-06"]
hours = ["17:00-01:00"]

# for this event only
image_seconds = 8
fit = "cover"
mute = false
`)
	if len(e.windows) != 2 || e.Show.ImageSeconds != 8 || e.Show.Fit != "cover" || e.Show.Mute {
		t.Errorf("got %+v", e)
	}
	mustEvent(t, "gala.event.toml", "folder = \"gala\"\ndates = [\"2026-12-05\"]\n# no hours: plays all day\n")
}

func TestParseEventRejects(t *testing.T) {
	for _, text := range []string{
		`dates = ["2026-12-05"]`,
		`folder = "../etc"` + "\n" + `dates = ["2026-12-05"]`,
		`folder = "a/b"` + "\n" + `dates = ["2026-12-05"]`,
		`folder = ".hidden"` + "\n" + `dates = ["2026-12-05"]`,
		`folder = "gala"`,
		`folder = "gala"` + "\n" + `dates = ["12/05/2026"]`,
		`folder = "gala"` + "\n" + `dates = ["2026-12-05"]` + "\n" + `hours = ["17:00"]`,
		`folder = "gala"` + "\n" + `dates = ["2026-12-05"]` + "\n" + `colour = "red"`,
		`folder = "gala"` + "\n" + `dates = ["2026-12-05"]` + "\n" + `fit = "stretch"`,
		`folder = "gala"` + "\n" + `dates = ["2026-12-05"]` + "\n" + "[schedule]\non = \"07:00\"\noff = \"09:00\"",
	} {
		if _, err := parseEvent("x.event.toml", []byte(text), defaultShow, time.UTC); err == nil {
			t.Errorf("parseEvent(%q): expected error", text)
		}
	}
}

func TestEventsDecideScreen(t *testing.T) {
	es := events{
		mustEvent(t, "a.event.toml", `folder = "a"
dates = ["2026-12-05"]
hours = ["17:00-23:30"]`),
		mustEvent(t, "b.event.toml", `folder = "b"
dates = ["2026-12-05"]
hours = ["16:00-18:00"]`),
		mustEvent(t, "late.event.toml", `folder = "late"
dates = ["2026-12-10"]
hours = ["20:00-01:00"]`),
	}
	weekdays := daily(9*60, 17*60)
	evening := daily(20*60, 2*60)

	tests := []struct {
		name     string
		schedule *Schedule
		o        override
		now      time.Time
		on       bool
		folder   string
	}{
		{"normal day follows schedule", weekdays, override{}, at(4, 10, 0), true, ""},
		{"event day before its hours is off", weekdays, override{}, at(5, 10, 0), false, ""},
		{"earlier start wins an overlap", weekdays, override{}, at(5, 17, 30), true, "b"},
		{"overlap over, other event plays", nil, override{}, at(5, 18, 0), true, "a"},
		{"after the event, rest of the day is off", nil, override{}, at(5, 23, 45), false, ""},
		{"day after follows schedule again", nil, override{}, at(6, 0, 0), true, ""},
		{"previous night's schedule is cut at midnight", evening, override{}, at(5, 1, 0), false, ""},
		{"overnight window runs into the next day", weekdays, override{}, at(11, 0, 30), true, "late"},
		{"next day isn't replaced after the tail", weekdays, override{}, at(11, 10, 0), true, ""},
		{"override off beats the event", nil, override{Until: at(5, 20, 0)}, at(5, 19, 0), false, "a"},
		{"override on beats the day off", nil, override{On: true, Until: at(5, 11, 0)}, at(5, 10, 0), true, ""},
	}
	for _, tt := range tests {
		if got := tt.o.screenOn(tt.schedule, es, tt.now); got != tt.on {
			t.Errorf("%s: screenOn = %v, want %v", tt.name, got, tt.on)
		}
		e, _, ok := es.active(tt.now)
		if folder := map[bool]string{true: e.Folder}[ok]; folder != tt.folder {
			t.Errorf("%s: active folder = %q, want %q", tt.name, folder, tt.folder)
		}
	}

	if got := es.liveFolders(at(5, 23, 45)); !slices.Equal(got, []string{"late"}) {
		t.Errorf("liveFolders = %v, want only late", got)
	}
	if e, w, ok := es.upcoming(at(6, 0, 0)); !ok || e.Folder != "late" || !w.start.Equal(at(10, 20, 0)) {
		t.Errorf("upcoming = %v %v %v", e.Folder, w, ok)
	}
}

func TestEventAcrossClockChange(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	// Clocks go back an hour at 02:00 on 2026-11-01.
	e, err := parseEvent("x.event.toml", []byte(`folder = "x"
dates = ["2026-11-01"]
hours = ["17:00-18:00"]`), defaultShow, loc)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.windows[0].start; got.Hour() != 17 || got.Minute() != 0 {
		t.Errorf("start = %v, want 17:00 local", got)
	}
	if got := e.days[0].end.Sub(e.days[0].start); got != 25*time.Hour {
		t.Errorf("day is %v long, want 25h", got)
	}
}

func TestLoadEvents(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "b.event.toml", "folder = \"b\"\ndates = [\"2026-12-05\"]\n")
	writeFile(t, dir, "a.event.toml", "folder = \"a\"\ndates = [\"2026-12-05\"]\n")
	writeFile(t, dir, "bad.event.toml", "folder = \"../x\"\ndates = [\"2026-12-05\"]\n")
	writeFile(t, dir, "display.toml", "image_seconds = 5\n")
	if err := os.Mkdir(filepath.Join(dir, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	es, problems := loadEvents(dir, defaultShow, time.UTC)
	if len(es) != 2 || es[0].File != "a.event.toml" || es[1].File != "b.event.toml" {
		t.Errorf("events = %+v, want a then b", es)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "bad.event.toml") {
		t.Errorf("problems = %v, want bad.event.toml", problems)
	}
}

func TestChoose(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	today := now.Format(time.DateOnly)
	writeFile(t, dir, "main.jpg", "x")
	writeFile(t, dir, "gala.event.toml", "folder = \"gala\"\ndates = [\""+today+"\"]\nimage_seconds = 3\n")

	// The folder isn't there yet: the main folder plays.
	c, _ := choose(dir, defaultShow, override{}, now)
	if !c.on || c.dir != dir || !c.waiting || c.event != "gala.event.toml" {
		t.Errorf("before the media arrives: %+v", c)
	}

	if err := os.Mkdir(filepath.Join(dir, "gala"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "gala"), "01.jpg", "x")
	c, _ = choose(dir, defaultShow, override{}, now)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	if c.dir != filepath.Join(dir, "gala") || c.waiting || c.show.ImageSeconds != 3 || !c.origin.Equal(midnight) {
		t.Errorf("with media: %+v", c)
	}
}
