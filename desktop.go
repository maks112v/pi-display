package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// startSlideshow handles `sudo pi-display start`: it restarts the slideshow,
// for example after Ctrl+Alt+T handed the screen to the console or
// `pi-display desktop` started the desktop.
func startSlideshow() error {
	if os.Geteuid() != 0 {
		return errors.New("run start with sudo")
	}
	if err := runCmd("systemctl", "restart", "pi-display"); err != nil {
		return err
	}
	fmt.Println("Slideshow starting.")

	// Like setup: a running desktop holds the screen until it stops. Stop it
	// last and without waiting, since this may be running in one of its
	// terminals. The service's Restart=always takes the screen once it's free.
	if exec.Command("systemctl", "is-active", "--quiet", "display-manager").Run() == nil {
		fmt.Println("Stopping the desktop so the slideshow can take the screen.")
		return runCmd("systemctl", "--no-block", "stop", "display-manager")
	}
	return nil
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
	if err := runCmd("systemctl", "stop", "pi-display"); err != nil {
		return err
	}
	if err := runCmd("systemctl", "start", "display-manager"); err != nil {
		return err
	}
	fmt.Println("Desktop starting. Back to the slideshow: sudo pi-display start, or reboot.")
	return nil
}
