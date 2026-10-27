package main

import (
	"errors"
	"fmt"
	"os"
)

// startSlideshow handles `sudo pi-display start`: it restarts the slideshow,
// for example after Ctrl+Alt+T handed the screen to the console or
// `pi-display desktop` started the desktop.
func startSlideshow() error {
	if os.Geteuid() != 0 {
		return errors.New("run start with sudo")
	}
	fmt.Println("Slideshow starting.")
	// Without waiting: starting the slideshow stops a running desktop, which
	// may be where this is running.
	return runCmd("systemctl", "--no-block", "restart", "pi-display")
}

// openDesktop handles `sudo pi-display desktop`: it stops the slideshow and
// starts the desktop until the next reboot, for changing Wi-Fi or other
// settings with a mouse. The Pi still boots into the slideshow.
func openDesktop() error {
	if os.Geteuid() != 0 {
		return errors.New("run desktop with sudo")
	}
	if _, err := os.Stat("/etc/systemd/system/display-manager.service"); err != nil {
		return errors.New("this Pi has no desktop installed (Raspberry Pi OS Lite). Change Wi-Fi from the console with: sudo raspi-config")
	}
	// Starting the desktop stops the slideshow; the service conflicts with it.
	if err := runCmd("systemctl", "start", "display-manager"); err != nil {
		return err
	}
	fmt.Println("Desktop starting. Back to the slideshow: sudo pi-display start, or reboot.")
	return nil
}
