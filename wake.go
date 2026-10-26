package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"
)

// keepOn handles `pi-display on [duration]`: the screen stays on for that
// long (default 1h) even outside the schedule. A duration of 0 cancels.
func keepOn(path, arg string) error {
	d := time.Hour
	if arg != "" {
		var err error
		if d, err = time.ParseDuration(arg); err != nil || d < 0 {
			return fmt.Errorf("%q is not a duration like 90m or 2h", arg)
		}
	}
	if d == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		fmt.Println("Override cleared. Back on schedule.")
		return nil
	}

	until := time.Now().Add(d).Truncate(time.Second)
	tmp := path + ".new"
	if err := os.WriteFile(tmp, []byte(until.Format(time.RFC3339)+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	fmt.Printf("Screen on until %s, then back on schedule.\n", until.Format("Mon 15:04"))
	return nil
}

// awakeUntil returns when the current override ends, or the zero time if
// there is none. An unreadable file counts as no override.
func awakeUntil(path string) time.Time {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	until, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Time{}
	}
	return until
}
