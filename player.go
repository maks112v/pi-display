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
	"strings"
	"time"
)

// mpv is a running mpv process controlled over its JSON IPC socket. mpv does
// decoding and rendering (straight to the display via DRM, with hardware
// decoding); this program only decides what to show and for how long.
type mpv struct {
	cmd    *exec.Cmd
	conn   net.Conn
	events chan mpvEvent
	vf     string // the video filter set for the current file
}

// imageVF shrinks images so neither side is over 2048px, the largest texture
// a Pi 3 or Zero 2 GPU can draw. Bigger photos (straight off a camera or from
// a stock site) otherwise decode fine and then show as a black screen, even
// on a Pi 5. Smaller images pass through untouched, and videos get no filter
// so they keep hardware decoding.
const imageVF = "lavfi=[scale=w='min(iw,2048)':h='min(ih,2048)':force_original_aspect_ratio=decrease]"

type mpvEvent struct {
	Event  string          `json:"event"`
	Name   string          `json:"name"`
	Data   json.RawMessage `json:"data"`
	Reason string          `json:"reason"`
}

// drmOutput finds the first connected display under sysDRM (normally
// /sys/class/drm), e.g. ("/dev/dri/card1", "HDMI-A-1"). mpv otherwise opens
// card0, which on a Pi 5 is the 3D-only GPU with no outputs, leaving the
// console on screen. ok is false when no output reports connected.
func drmOutput(sysDRM string) (device, connector string, ok bool) {
	paths, _ := filepath.Glob(filepath.Join(sysDRM, "card*-*"))
	for _, p := range paths {
		status, err := os.ReadFile(filepath.Join(p, "status"))
		if err != nil || strings.TrimSpace(string(status)) != "connected" {
			continue
		}
		card, connector, _ := strings.Cut(filepath.Base(p), "-")
		return "/dev/dri/" + card, connector, true
	}
	return "", "", false
}

// waitForDisplay returns the connected display, waiting for one if the
// screen is unplugged or HDMI hasn't come up yet at boot. Starting mpv before
// then would leave it drawing to the wrong card.
func waitForDisplay(ctx context.Context) (device, connector string, err error) {
	for logged := false; ; logged = true {
		if device, connector, ok := drmOutput("/sys/class/drm"); ok {
			if logged {
				log.Printf("display connected on %s %s", device, connector)
			}
			return device, connector, nil
		}
		if !logged {
			log.Print("waiting for a display to be connected")
		}
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func startMPV(ctx context.Context, socket string) (*mpv, error) {
	_ = os.Remove(socket)
	args := []string{
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
		// mpv's default high-quality scaling drops most frames of a 1080p
		// video on a Pi 5's GPU. Bilinear scaling plays smoothly.
		"--profile=fast",
		"--keep-open=yes", // hold a video's last frame until the next file replaces it
		"--image-display-duration=inf",
		"--input-ipc-server=" + socket,
	}
	device, connector, err := waitForDisplay(ctx)
	if err != nil {
		return nil, err
	}
	args = append(args, "--drm-device="+device, "--drm-connector="+connector)
	cmd := exec.CommandContext(ctx, "mpv", args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mpv: %w", err)
	}

	var conn net.Conn
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
	vf := ""
	if image {
		vf = imageVF
	}
	if vf != m.vf {
		if err := m.command("set_property", "vf", vf); err != nil {
			return err
		}
		m.vf = vf
	}
	// --keep-open pauses mpv at the end of a video, and the pause carries
	// over to the next file, which would leave the next video stuck on its
	// first frame.
	if err := m.command("set_property", "pause", false); err != nil {
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
// layered over. While the folder has nothing to play, the image at empty is
// shown instead. The folder, display.toml, and any `pi-display on|off`
// override are re-read before every file, so changes show up without
// restarting anything.
// booted means the Pi just powered on, so the TV gets one wake over HDMI-CEC
// if the screen should be on, even without tv_power, in case it came back up
// in standby.
func play(ctx context.Context, m *mpv, dir, empty string, base Show, booted bool) error {
	show := base
	current := ""
	showingEmpty := false
	order := newPlayOrder()
	screenOn := true
	bootWake := booted
	var seen override // the last override read, to spot a new on or off command
	var lastProblem, lastSkip string
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

		// Outside the schedule the screen goes black, and `pi-display on|off`
		// overrides the schedule for a while. With tv_power the TV is also put
		// in standby over HDMI-CEC, and woken when the screen comes back on.
		// A new on or off command switches the TV even if the screen was
		// already in that state, so `on` wakes a TV someone turned off with
		// the remote. The change lands at the next file boundary.
		now := time.Now()
		o := readOverride(overridePath)
		fresh := o != seen && o.active(now)
		seen = o
		if on := o.screenOn(show.Schedule, now); on != screenOn || fresh || bootWake {
			wake := bootWake && on
			bootWake = false
			screenOn = on
			switch until := o.Until.Format("Mon 15:04"); {
			case o.active(now) && on:
				log.Printf("screen on until %s (pi-display on)", until)
			case o.active(now):
				log.Printf("screen off until %s (pi-display off)", until)
			case on:
				log.Print("screen on")
			default:
				log.Print("screen off until the schedule turns it back on")
			}
			if show.TVPower || wake {
				if err := setTVPower(ctx, on); err != nil {
					log.Printf("couldn't switch the TV over HDMI-CEC: %v", err)
				}
			}
		}

		var names []string
		if screenOn {
			var unplayable []string
			var err error
			names, unplayable, err = listMedia(dir)
			if err != nil {
				problem = fmt.Sprintf("read %s: %v", dir, err)
			} else if len(names) == 0 {
				problem = fmt.Sprintf("no images or videos in %s yet", dir)
			}
			skip := ""
			if len(unplayable) > 0 {
				skip = unplayableNote(unplayable)
			}
			if skip != "" && skip != lastSkip {
				log.Print(skip)
			}
			lastSkip = skip
		}
		report(problem)

		// With nothing to play the screen shows the empty screen, or goes
		// black while it is off.
		if len(names) == 0 {
			if current != "" || showingEmpty != screenOn {
				current = ""
				showingEmpty = screenOn
				var err error
				if screenOn {
					err = m.command("loadfile", empty, "replace")
				} else {
					err = m.command("stop")
				}
				if err != nil {
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
		current, showingEmpty = next, false
		if err := m.show(ctx, filepath.Join(dir, next), image, show.imageDuration(next)); err != nil {
			return err
		}
	}
}
