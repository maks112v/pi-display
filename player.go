package main

import (
	"bufio"
	"cmp"
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
	device, connector, err := waitForDisplay(ctx)
	if err != nil {
		return nil, err
	}
	m, err := launchMPV(ctx, socket,
		"--force-window=yes",
		"--fullscreen",
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
		"--drm-device="+device,
		"--drm-connector="+connector,
	)
	if err != nil {
		return nil, err
	}
	// eof-reached ends videos; the rest feed the playback report in show.
	for i, name := range []string{"eof-reached", "hwdec-current", "frame-drop-count", "decoder-frame-drop-count", "vo-delayed-frame-count"} {
		if err := m.command("observe_property", i+1, name); err != nil {
			m.close()
			return nil, err
		}
	}
	return m, nil
}

// launchMPV starts an idle mpv with args on top of the settings every mpv
// here shares, and connects to it over socket.
func launchMPV(ctx context.Context, socket string, args ...string) (*mpv, error) {
	_ = os.Remove(socket)
	args = append([]string{
		"--no-config", // ignore any mpv.conf or scripts lying around
		"--load-scripts=no",
		"--ytdl=no",
		"--idle=yes",
		"--no-terminal",
		"--input-ipc-server=" + socket,
	}, args...)
	cmd := exec.CommandContext(ctx, "mpv", args...)
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
		_ = cmd.Wait()
		return nil, fmt.Errorf("connect to mpv: %w", err)
	}

	m := &mpv{cmd: cmd, conn: conn, events: make(chan mpvEvent, 64)}
	go m.readEvents()
	return m, nil
}

// probeDuration reads how long the video at path runs, using a separate
// paused mpv with no video or audio output so the screen is untouched. It is
// rounded to the millisecond so every Pi gets the same timeline.
func probeDuration(ctx context.Context, path string) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	m, err := launchMPV(ctx, filepath.Join(filepath.Dir(socketPath), "probe.sock"), "--vo=null", "--ao=null", "--pause")
	if err != nil {
		return 0, err
	}
	defer m.close()
	if err := m.command("observe_property", 1, "duration"); err != nil {
		return 0, err
	}
	if err := m.command("loadfile", path, "replace"); err != nil {
		return 0, err
	}
	for {
		select {
		case <-ctx.Done():
			return 0, errors.New("timed out")
		case e, ok := <-m.events:
			if !ok {
				return 0, errMPVExited
			}
			var seconds float64
			switch {
			case e.Event == "end-file" && e.Reason == "error":
				return 0, errors.New("mpv could not open it")
			case e.Event == "property-change" && e.Name == "duration" && json.Unmarshal(e.Data, &seconds) == nil && seconds > 0:
				return time.Duration(seconds * float64(time.Second)).Round(time.Millisecond), nil
			}
		}
	}
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

// show displays one file and returns once it is done: at until, at the end
// of a video, immediately if mpv can't load it, as soon as interrupt fires,
// or when moved, checked every second, reports something else should be on
// screen. A video starts offset into it, so a screen that joins partway
// through matches the others.
func (m *mpv) show(ctx context.Context, interrupt <-chan struct{}, moved func() bool, path string, image bool, offset time.Duration, until time.Time) error {
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
	start := "none"
	if offset > 0 {
		start = fmt.Sprintf("%.3f", offset.Seconds())
	}
	if err := m.command("set_property", "start", start); err != nil {
		return err
	}
	// Absolute paths keep mpv from treating a filename as a URL or protocol.
	if err := m.command("loadfile", path, "replace"); err != nil {
		return err
	}

	loaded := false
	var report playback
	if !image {
		defer func() { report.log(filepath.Base(path)) }()
	}
	loadTimeout := time.After(30 * time.Second)
	var done <-chan time.Time
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-interrupt:
			return nil
		case <-tick.C:
			if moved() {
				return nil
			}
		case <-loadTimeout:
			log.Printf("timed out loading %s", filepath.Base(path))
			return nil
		case <-done:
			return nil
		case e, ok := <-m.events:
			if !ok {
				return errMPVExited
			}
			// mpv resets these for each file, so the last values are this one's.
			if e.Event == "property-change" {
				report.update(e)
			}
			switch {
			case e.Event == "file-loaded":
				loaded, loadTimeout = true, nil
				done = time.After(time.Until(until))
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

// playback is what mpv reported about one video, to spot why playback
// stutters: frames dropped because the Pi couldn't keep up, and whether
// hardware decoding is on.
type playback struct {
	hwdec                     string // "no" means the CPU is decoding
	dropped, decoder, delayed int    // dropped by the display, by the decoder, and shown late
}

func (p *playback) update(e mpvEvent) {
	switch e.Name {
	case "hwdec-current":
		_ = json.Unmarshal(e.Data, &p.hwdec)
	case "frame-drop-count":
		_ = json.Unmarshal(e.Data, &p.dropped)
	case "decoder-frame-drop-count":
		_ = json.Unmarshal(e.Data, &p.decoder)
	case "vo-delayed-frame-count":
		_ = json.Unmarshal(e.Data, &p.delayed)
	}
}

// log notes a video that played with trouble or without hardware decoding,
// and stays quiet for one that played cleanly.
func (p playback) log(name string) {
	if p.dropped == 0 && p.decoder == 0 && p.delayed == 0 && p.hwdec != "no" {
		return
	}
	log.Printf("played %s: %d frames dropped, %d dropped by the decoder, %d late, hardware decoding %s",
		name, p.dropped, p.decoder, p.delayed, cmp.Or(p.hwdec, "unknown"))
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
// layered over, following the wall-clock timeline in timeline.go so every
// screen on the folder shows the same file at once. While the folder has
// nothing to play, the image at empty is shown instead. The folder,
// display.toml, and any `pi-display on|off` override are re-read before every
// file, so changes show up without restarting anything. A new
// `pi-display on|off` cuts the current file short so it takes effect right
// away, and so does a change to the folder or display.toml that moves what
// should be on screen: the screen jumps straight to the new spot in the
// timeline, cutting off whatever was playing.
// settle is how old a file must be before it plays (see settled), and booted
// means the Pi just powered on, so the TV gets one wake over HDMI-CEC if the
// screen should be on, even without tv_power, in case it came back up in
// standby.
func play(ctx context.Context, m *mpv, dir, empty string, base Show, settle time.Duration, booted bool) error {
	show := base
	var current slot // what's on screen; its name is "" when nothing is
	showingEmpty := false
	videos := newVideoLengths()
	screenOn := true
	bootWake := booted
	var seen override // the last override read, to spot a new on or off command
	interrupt := watchOverride(ctx, overridePath)
	go watchScreenFile(ctx, dir)
	// lineup is what should be on screen at now, given show and the files
	// in the folder.
	lineup := func(show Show, names []string, now time.Time) (slot, bool) {
		length := func(name string) time.Duration {
			if isImage(name) {
				return show.imageDuration(name)
			}
			return videos.length(ctx, dir, name)
		}
		return slotAt(settled(dir, names, settle, now), length, show.Shuffle, now)
	}
	// moved re-reads the folder and display.toml while a file plays and
	// reports whether they now put something else on screen.
	var slowest time.Duration
	moved := func() bool {
		defer func(start time.Time) {
			if took := time.Since(start); took > 250*time.Millisecond && took > slowest {
				slowest = took
				log.Printf("checking %s for changes took %s, which can make video stutter", dir, took.Round(time.Millisecond))
			}
		}(time.Now())
		s, err := loadShow(dir, base)
		if err != nil {
			s = show
		}
		names, _, err := listMedia(dir)
		if err != nil {
			return false
		}
		want, ok := lineup(s, names, time.Now())
		return !ok || want.name != current.name || !want.start.Equal(current.start)
	}
	// hold keeps what's on screen until t, a new on or off command, or a
	// change that moves it.
	hold := func(t time.Time) error {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		deadline := time.After(time.Until(t))
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-interrupt:
				return nil
			case <-deadline:
				return nil
			case <-tick.C:
				if moved() {
					return nil
				}
			}
		}
	}
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
		// the remote. Schedule changes and expired overrides land at the next
		// file boundary.
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
		var playing slot
		if screenOn {
			var unplayable []string
			var err error
			names, unplayable, err = listMedia(dir)
			if err != nil {
				problem = fmt.Sprintf("read %s: %v", dir, err)
			} else if len(names) == 0 {
				problem = fmt.Sprintf("no images or videos in %s yet", dir)
			}
			var ok bool
			if playing, ok = lineup(show, names, time.Now()); !ok {
				names = nil
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
			if current.name != "" || showingEmpty != screenOn {
				current = slot{}
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
			case <-interrupt:
			case <-time.After(5 * time.Second):
			}
			continue
		}

		// The file on screen keeps its turn: an image that comes round again
		// (a lone image) stays up instead of reloading, and a video that
		// ended a moment early holds its last frame until the next turn.
		image := isImage(playing.name)
		if playing.name == current.name && (image || playing.start.Equal(current.start)) {
			current = playing
			if err := hold(playing.end); err != nil {
				return err
			}
			continue
		}
		// Joining a video more than a second in seeks to where the others
		// are. Any less and the seek would cost more than it saves.
		var offset time.Duration
		if late := time.Since(playing.start); !image && late > time.Second {
			offset = late
		}
		current, showingEmpty = playing, false
		if err := m.show(ctx, interrupt, moved, filepath.Join(dir, playing.name), image, offset, playing.end); err != nil {
			return err
		}
	}
}
