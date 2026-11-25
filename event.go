package main

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// An event is a one-time takeover of the screens, set by a file like
// gala.event.toml at the top of the media folder:
//
//	folder = "gala"
//	dates  = ["2026-12-05"]
//	hours  = ["17:00-23:30"]  # leave out for the whole day
//
// During its hours every screen plays the subfolder gala/ instead of the
// main folder, starting on its first file at the window's start. On those
// dates its hours replace the schedule, so outside them the screen is off.
// Once its last window is over the file does nothing, so it never runs again.
// Slideshow keys (image_seconds, mute, fit, shuffle, [durations]) may be set
// too; they apply to the event only, over display.toml. tv_power stays
// display.toml's, so the player that put the TV in standby also wakes it.

const (
	eventSuffix  = ".event.toml"
	maxEventSize = 64 << 10
	maxEvents    = 64
	maxDates     = 366
	maxHours     = 24
)

type Event struct {
	File    string // e.g. gala.event.toml
	Folder  string
	Show    Show
	windows []span // when it plays, in order
	days    []span // midnight to midnight of each date, when it replaces the schedule
}

// span is [start, end).
type span struct{ start, end time.Time }

func (s span) contains(t time.Time) bool { return !t.Before(s.start) && t.Before(s.end) }

// eventFile is the file's layout. Show is embedded so the slideshow keys sit
// at the top level, next to folder, dates, and hours.
type eventFile struct {
	Folder string   `toml:"folder"`
	Dates  []string `toml:"dates"`
	Hours  []string `toml:"hours"`
	Show
}

// parseEvent reads one event file. Slideshow keys are layered over base,
// except [durations], which starts empty so a file in the event folder can't
// pick up a duration meant for a same-named file in the main folder. Dates
// and hours are in loc.
func parseEvent(name string, data []byte, base Show, loc *time.Location) (Event, error) {
	f := eventFile{Show: base}
	f.Durations, f.Schedule = nil, nil
	md, err := toml.Decode(string(data), &f)
	if err != nil {
		return Event{}, err
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return Event{}, fmt.Errorf("unknown key %q", undecoded[0].String())
	}
	if f.Schedule != nil {
		return Event{}, errors.New("an event takes dates and hours, not a schedule")
	}
	if md.IsDefined("tv_power") {
		return Event{}, errors.New("tv_power is set in display.toml, not in an event")
	}
	if !safeName(f.Folder) {
		return Event{}, fmt.Errorf(`folder must be the name of a subfolder, like "gala", got %q`, f.Folder)
	}
	if len(f.Dates) == 0 || len(f.Dates) > maxDates {
		return Event{}, fmt.Errorf(`dates needs 1 to %d days, like ["2026-12-05"]`, maxDates)
	}
	if err := f.Show.validate(); err != nil {
		return Event{}, err
	}
	if len(f.Hours) > maxHours {
		return Event{}, fmt.Errorf("hours takes at most %d windows", maxHours)
	}
	windows := make([]window, len(f.Hours))
	for i, text := range f.Hours {
		if windows[i], err = parseWindow(text); err != nil {
			return Event{}, fmt.Errorf("hours: %w", err)
		}
	}

	e := Event{File: name, Folder: f.Folder, Show: f.Show}
	for _, text := range f.Dates {
		d, err := time.ParseInLocation(time.DateOnly, text, loc)
		if err != nil {
			return Event{}, fmt.Errorf("dates: %q is not a date like 2026-12-05", text)
		}
		// time.Date rather than adding minutes, so clock changes don't
		// shift the hours.
		// A time the clocks skip going forward, like 02:30, comes back
		// from time.Date an hour early; it's moved an hour later instead.
		at := func(days int, c clock) time.Time {
			t := time.Date(d.Year(), d.Month(), d.Day()+days, int(c/60), int(c%60), 0, 0, loc)
			if t.Hour() != int(c/60) || t.Minute() != int(c%60) {
				t = t.Add(time.Hour)
			}
			return t
		}
		day := span{at(0, 0), at(1, 0)}
		e.days = append(e.days, day)
		if len(windows) == 0 {
			e.windows = append(e.windows, day)
		}
		for _, w := range windows {
			end := at(0, w.Off)
			if w.Off < w.On {
				end = at(1, w.Off)
			}
			// Moving a skipped time can leave nothing of a short window.
			if start := at(0, w.On); end.After(start) {
				e.windows = append(e.windows, span{start, end})
			}
		}
	}
	slices.SortFunc(e.windows, func(a, b span) int { return a.start.Compare(b.start) })
	return e, nil
}

// safeName reports whether name is a single, visible entry of a folder.
func safeName(name string) bool {
	return name != "" && fs.ValidPath(name) && !strings.ContainsAny(name, `/\`) && !strings.HasPrefix(name, ".")
}

func isEventFile(name string) bool {
	return strings.HasSuffix(name, eventSuffix) && len(name) > len(eventSuffix)
}

// ended reports whether every window is over at now.
func (e Event) ended(now time.Time) bool {
	_, ok := e.next(now)
	return !ok
}

// next is the first window, by start, that hasn't ended at now. Windows are
// sorted by start, not end, so an overnight window can end after a later one.
func (e Event) next(now time.Time) (span, bool) {
	for _, w := range e.windows {
		if now.Before(w.end) {
			return w, true
		}
	}
	return span{}, false
}

type events []Event

// loadEvents reads every *.event.toml at the top of dir, in filename order,
// with slideshow keys layered over base. Files that don't parse are left out
// and described in problems.
func loadEvents(dir string, base Show, loc *time.Location) (es events, problems []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isEventFile(name) || strings.HasPrefix(name, ".") {
			continue
		}
		if len(es) == maxEvents {
			problems = append(problems, fmt.Sprintf("ignoring %s and later events, %d is the most a folder can hold", name, maxEvents))
			break
		}
		e, err := readEvent(filepath.Join(dir, name), base, loc)
		if err != nil {
			problems = append(problems, fmt.Sprintf("ignoring %s: %v", name, err))
			continue
		}
		es = append(es, e)
	}
	return es, problems
}

func readEvent(path string, base Show, loc *time.Location) (Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return Event{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxEventSize+1))
	if err != nil {
		return Event{}, err
	}
	if len(data) > maxEventSize {
		return Event{}, fmt.Errorf("larger than %d bytes", maxEventSize)
	}
	return parseEvent(filepath.Base(path), data, base, loc)
}

// active returns the event playing at now and its current window. When two
// overlap, the window that started first wins, then the first filename.
func (es events) active(now time.Time) (Event, span, bool) {
	var best Event
	var bestWindow span
	found := false
	for _, e := range es {
		for _, w := range e.windows {
			if w.contains(now) && (!found || w.start.Before(bestWindow.start)) {
				best, bestWindow, found = e, w, true
			}
		}
	}
	return best, bestWindow, found
}

// dayOff reports whether now falls on an event's date, where the event's
// hours replace the schedule.
func (es events) dayOff(now time.Time) bool {
	for _, e := range es {
		for _, d := range e.days {
			if d.contains(now) {
				return true
			}
		}
	}
	return false
}

// upcoming is the next window to start after now, across all events.
func (es events) upcoming(now time.Time) (Event, span, bool) {
	var best Event
	var bestWindow span
	found := false
	for _, e := range es {
		for _, w := range e.windows {
			if w.start.After(now) && (!found || w.start.Before(bestWindow.start)) {
				best, bestWindow, found = e, w, true
			}
		}
	}
	return best, bestWindow, found
}

// liveFolders are the folders of events that haven't ended, which sync keeps
// mirrored ahead of time so the media is on the Pi when the event starts.
func (es events) liveFolders(now time.Time) []string {
	var folders []string
	for _, e := range es {
		if !e.ended(now) && !slices.Contains(folders, e.Folder) {
			folders = append(folders, e.Folder)
		}
	}
	slices.SortFunc(folders, cmp.Compare)
	return folders
}
