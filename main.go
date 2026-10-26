// pi-display turns a Raspberry Pi into a full-screen slideshow of the images
// and videos in a folder, either local or mirrored from an SMB share.
//
//	sudo pi-display setup   # one-time: pick the source, install the boot service
//	pi-display run          # what the service runs
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
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
		err = setup(ctx)
	default:
		fmt.Fprintln(os.Stderr, "usage: pi-display <setup|run>")
		os.Exit(2)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

func argAt(i int) string {
	if len(os.Args) > i {
		return os.Args[i]
	}
	return ""
}

// run plays the configured folder forever. SMB sources are mirrored into a
// local cache every 30s so playback starts at boot without waiting for the
// network and keeps going through outages.
func run(ctx context.Context) error {
	device, err := loadDevice(devicePath)
	if err != nil {
		return fmt.Errorf("load %s (run `sudo pi-display setup` first): %w", devicePath, err)
	}

	dir := device.Source
	if isSMB(device.Source) {
		dir = cacheDir
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		go syncLoop(ctx, device, dir)
	}

	player, err := startMPV(ctx, socketPath)
	if err != nil {
		return err
	}
	defer player.close()

	return play(ctx, player, dir)
}
