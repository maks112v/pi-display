package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// maxPendingLines bounds what is held in memory while the share is
	// unreachable. The oldest lines are dropped first.
	maxPendingLines = 500
	// maxShareLogSize is when the log on the share is started over, so a
	// screen that errors forever can't fill the share.
	maxShareLogSize = 1 << 20
)

// shareLog copies this screen's log lines into <hostname>.pi-display.log in
// the SMB folder, so problems on every screen can be read in one place. It is
// installed as part of the standard logger's output; lines are buffered and
// appended to the share on each sync. The file isn't media, so players and
// syncs ignore it.
type shareLog struct {
	name string // file name inside the share folder

	mu      sync.Mutex
	pending []string
	dropped int
	lastErr string
}

func newShareLog(hostname string) *shareLog {
	return &shareLog{name: shareLogName(hostname)}
}

var unsafeHostChars = regexp.MustCompile(`[^A-Za-z0-9-]+`)

func shareLogName(hostname string) string {
	host := strings.Trim(unsafeHostChars.ReplaceAllString(hostname, "-"), "-")
	if host == "" {
		host = "unknown"
	}
	return host + ".pi-display.log"
}

// Write implements io.Writer for log.SetOutput. Each call is one log line.
func (l *shareLog) Write(p []byte) (int, error) {
	line := time.Now().Format("2006-01-02 15:04:05 ") + strings.TrimRight(string(p), "\n")
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pending = append(l.pending, line)
	if over := len(l.pending) - maxPendingLines; over > 0 {
		l.pending = l.pending[over:]
		l.dropped += over
	}
	return len(p), nil
}

// appender is a folder the log can be written to. shareFS implements it.
type appender interface {
	appendFile(name string, data []byte, maxSize int64) error
}

// flush appends pending lines to the share. On failure the lines are kept for
// the next sync, and the failure goes to stderr (not the logger, which would
// feed it back into the buffer).
func (l *shareLog) flush(dst appender) {
	l.mu.Lock()
	lines, dropped := l.pending, l.dropped
	l.pending, l.dropped = nil, 0
	l.mu.Unlock()
	if len(lines) == 0 && dropped == 0 {
		return
	}

	var b strings.Builder
	if dropped > 0 {
		fmt.Fprintf(&b, "(%d earlier lines dropped while the share was unreachable)\n", dropped)
	}
	for _, line := range lines {
		b.WriteString(line + "\n")
	}
	err := dst.appendFile(l.name, []byte(b.String()), maxShareLogSize)

	l.mu.Lock()
	defer l.mu.Unlock()
	if err == nil {
		l.lastErr = ""
		return
	}
	// Put the lines back ahead of anything logged during the write.
	l.pending = append(lines, l.pending...)
	l.dropped += dropped
	if over := len(l.pending) - maxPendingLines; over > 0 {
		l.pending = l.pending[over:]
		l.dropped += over
	}
	if msg := err.Error(); msg != l.lastErr {
		fmt.Fprintf(os.Stderr, "can't write %s to the share (read-only login?): %v\n", l.name, err)
		l.lastErr = msg
	}
}
