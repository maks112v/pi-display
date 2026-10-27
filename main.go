// pi-display turns a Raspberry Pi into a full-screen slideshow of the images
// and videos in a folder, either local or mirrored from an SMB share or a
// public Google Drive folder.
//
//	sudo pi-display setup [code]  # one-time: pick the source, install the boot service
//	                              # (code comes from the web config generator and skips the prompts)
//	pi-display run                # what the service runs
//	pi-display on [2h]            # stay on outside the schedule for a while (0 cancels)
//	sudo pi-display update        # install the latest release and restart
//	pi-display status             # version, source, service health, and screen state
//	pi-display logs               # follow the service's logs (journalctl -u pi-display -f)
//	pi-display version
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func main() {
	log.SetFlags(0) // journald adds timestamps

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch command := argAt(1); command {
	case "run":
		err = run(ctx)
	case "setup":
		err = setup(ctx, argAt(2))
	case "on":
		err = keepOn(wakePath, argAt(2))
	case "update":
		err = update(ctx)
	case "status":
		status()
	case "logs":
		err = followLogs()
	case "version":
		fmt.Println(version)
	default:
		fmt.Fprintln(os.Stderr, "usage: pi-display <setup [code]|run|on [duration]|update|status|logs|version>")
		os.Exit(2)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

// followLogs replaces this process with journalctl so Ctrl-C and paging
// behave exactly as they do when running it directly.
func followLogs() error {
	journalctl, err := exec.LookPath("journalctl")
	if err != nil {
		return err
	}
	return syscall.Exec(journalctl, []string{"journalctl", "-u", "pi-display", "-f"}, os.Environ())
}

func argAt(i int) string {
	if len(os.Args) > i {
		return os.Args[i]
	}
	return ""
}

// run plays the configured folder forever. Remote sources are mirrored into a
// local cache every 30s so playback starts at boot without waiting for the
// network and keeps going through outages.
func run(ctx context.Context) error {
	device, err := loadDevice(devicePath)
	if err != nil {
		return fmt.Errorf("load %s (run `sudo pi-display setup` first): %w", devicePath, err)
	}

	dir := device.Source
	if isRemote(device.Source) {
		dir = cacheDir
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		// SMB folders also collect each screen's log, so problems across
		// screens can be read from the share without SSH.
		var logs *shareLog
		if isSMB(device.Source) {
			host, _ := os.Hostname()
			logs = newShareLog(host)
			log.SetOutput(io.MultiWriter(os.Stderr, logs))
		}
		go syncLoop(ctx, device, dir, logs)
	}

	player, err := startMPV(ctx, socketPath)
	if err != nil {
		return err
	}
	defer player.close()

	return play(ctx, player, dir, device.baseShow())
}
