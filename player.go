package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// mpv is a running mpv process controlled over its JSON IPC socket. mpv does
// decoding and rendering (straight to the display via DRM, with hardware
// decoding); this program only decides what to show and for how long.
type mpv struct {
	cmd    *exec.Cmd
	conn   net.Conn
	events chan mpvEvent
}

type mpvEvent struct {
	Event  string          `json:"event"`
	Name   string          `json:"name"`
	Data   json.RawMessage `json:"data"`
	Reason string          `json:"reason"`
}

func startMPV(ctx context.Context, socket string) (*mpv, error) {
	_ = os.Remove(socket)
	cmd := exec.CommandContext(ctx, "mpv",
		"--no-config", // ignore any mpv.conf or scripts lying around
		"--load-scripts=no",
		"--ytdl=no",
		"--idle=yes",
		"--force-window=yes",
		"--fullscreen",
		"--no-terminal",
		"--osc=no",
		"--osd-level=0",
		"--cursor-autohide=always",
		"--no-input-default-bindings",
		"--input-vo-keyboard=no",
		"--hwdec=auto-safe",
		"--keep-open=yes", // hold a video's last frame until the next file replaces it
		"--image-display-duration=inf",
		"--input-ipc-server="+socket,
	)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mpv: %w", err)
	}

	var conn net.Conn
	var err error
	for range 100 {
		if conn, err = net.Dial("unix", socket); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("connect to mpv: %w", err)
	}

	m := &mpv{cmd: cmd, conn: conn, events: make(chan mpvEvent, 64)}
	go m.readEvents()
	if err := m.command("observe_property", 1, "eof-reached"); err != nil {
		m.close()
		return nil, err
	}
	return m, nil
}

// readEvents forwards mpv events and closes the channel when mpv goes away.
// Command replies are dropped; failures surface as end-file events instead.
func (m *mpv) readEvents() {
	defer close(m.events)
	scanner := bufio.NewScanner(m.conn)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		var e mpvEvent
		if json.Unmarshal(scanner.Bytes(), &e) == nil && e.Event != "" {
			m.events <- e
		}
	}
}

func (m *mpv) command(args ...any) error {
	data, err := json.Marshal(map[string]any{"command": args})
	if err != nil {
		return err
	}
	_, err = m.conn.Write(append(data, '\n'))
	return err
}

func (m *mpv) close() {
	_ = m.conn.Close()
	_ = m.cmd.Process.Kill()
	_ = m.cmd.Wait()
}

var errMPVExited = errors.New("mpv exited")

// show displays one file and returns once it is done: after the image
// duration, at the end of a video, or immediately if mpv can't load it.
func (m *mpv) show(ctx context.Context, path string, image bool, duration time.Duration) error {
	m.drain()
	// Loop images so animated GIFs keep moving for their full duration.
	loop := "no"
	if image {
		loop = "inf"
	}
	if err := m.command("set_property", "loop-file", loop); err != nil {
		return err
	}
	// Absolute paths keep mpv from treating a filename as a URL or protocol.
	if err := m.command("loadfile", path, "replace"); err != nil {
		return err
	}

	loaded := false
	loadTimeout := time.After(30 * time.Second)
	var imageDone <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-loadTimeout:
			log.Printf("timed out loading %s", filepath.Base(path))
			return nil
		case <-imageDone:
			return nil
		case e, ok := <-m.events:
			if !ok {
				return errMPVExited
			}
			switch {
			case e.Event == "file-loaded":
				loaded, loadTimeout = true, nil
				if image {
					imageDone = time.After(duration)
				}
			// Before file-loaded, an end-file is the previous file being
			// replaced, unless the new one failed to open.
			case e.Event == "end-file" && (loaded || e.Reason == "error"):
				if e.Reason == "error" {
					log.Printf("mpv could not play %s", filepath.Base(path))
				}
				return nil
			case loaded && !image && e.Event == "property-change" && e.Name == "eof-reached" && string(e.Data) == "true":
				return nil
			}
		}
	}
}

// drain discards events left over from the previous file.
func (m *mpv) drain() {
	for {
		select {
		case _, ok := <-m.events:
			if !ok {
				return
			}
		default:
			return
		}
	}
}

// play loops through dir forever, with base as the settings display.toml is
// layered over. The folder, display.toml, and any `pi-display on` override are
// re-read before every file, so changes show up without restarting anything.
func play(ctx context.Context, m *mpv, dir string, base Show) error {
	show := base
	current := ""
	order := newPlayOrder()
	screenOn := true
	var lastProblem string
	report := func(problem string) {
		if problem != "" && problem != lastProblem {
			log.Print(problem)
		}
		lastProblem = problem
	}

	for {
		problem := ""
		if s, err := loadShow(dir, base); err != nil {
			problem = fmt.Sprintf("ignoring %s, keeping previous settings: %v", showFile, err)
		} else {
			show = s
		}
		panscan := 0.0
		if show.Fit == "cover" {
			panscan = 1.0
		}
		if err := m.command("set_property", "mute", show.Mute); err != nil {
			return err
		}
		if err := m.command("set_property", "panscan", panscan); err != nil {
			return err
		}

		// Outside the schedule the screen goes black, unless `pi-display on`
		// is holding it on. The change lands at the next file boundary.
		now := time.Now()
		if on := show.Schedule.awake(now) || now.Before(awakeUntil(wakePath)); on != screenOn {
			screenOn = on
			if on {
				log.Print("screen on")
			} else {
				log.Print("screen off until the schedule turns it back on")
			}
		}

		var names []string
		if screenOn {
			var err error
			names, err = listMedia(dir)
			if err != nil {
				problem = fmt.Sprintf("read %s: %v", dir, err)
			} else if len(names) == 0 {
				problem = fmt.Sprintf("no images or videos in %s yet", dir)
			}
		}
		report(problem)

		if len(names) == 0 {
			if current != "" {
				current = ""
				if err := m.command("stop"); err != nil {
					return err
				}
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
			continue
		}

		order.shuffle = show.Shuffle
		next := order.next(names, current)
		image := isImage(next)
		if next == current && image {
			// A lone image: keep it on screen instead of reloading it.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(show.imageDuration(next)):
			}
			continue
		}
		current = next
		if err := m.show(ctx, filepath.Join(dir, next), image, show.imageDuration(next)); err != nil {
			return err
		}
	}
}
