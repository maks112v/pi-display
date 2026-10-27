package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// uninstall undoes setup: it removes the service, the program, its settings
// (including any SMB password), the mirrored media, and the boot logo. Setup
// may also have turned off the desktop and installed mpv, but it doesn't
// record whether it did, so those are offered rather than done unasked.
func uninstall() error {
	if os.Geteuid() != 0 {
		return errors.New("run uninstall with sudo")
	}
	in := bufio.NewReader(os.Stdin)

	if _, err := os.Stat(unitPath); err == nil {
		fmt.Println("Stopping the slideshow...")
		if err := runCmd("systemctl", "disable", "--now", "pi-display"); err != nil {
			return err
		}
	}
	// The program goes last: removing a running executable is fine on Linux.
	for _, path := range []string{unitPath, filepath.Dir(devicePath), filepath.Dir(cacheDir), binPath} {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	if err := runCmd("systemctl", "daemon-reload"); err != nil {
		return err
	}
	fmt.Println("Removed the service, program, settings, and mirrored media.")
	if err := removeSplash(); err != nil {
		fmt.Println("Couldn't restore the boot logo:", err)
	}

	// A display manager on a console-boot Pi means the desktop is installed
	// but switched off, most likely by setup.
	target, _ := exec.Command("systemctl", "get-default").Output()
	if _, err := os.Stat("/etc/systemd/system/display-manager.service"); err == nil && strings.TrimSpace(string(target)) == "multi-user.target" {
		ok, err := askYes(in, "Start the desktop at boot again? [Y/n] ", true)
		if err != nil {
			return err
		}
		if ok {
			if err := runCmd("systemctl", "set-default", "graphical.target"); err != nil {
				return err
			}
			fmt.Println("The desktop starts on the next boot: sudo reboot")
		}
	}

	if _, err := exec.LookPath("mpv"); err == nil {
		ok, err := askYes(in, "Remove mpv too? Keep it if anything else uses it. [y/N] ", false)
		if err != nil {
			return err
		}
		if ok {
			if err := runCmd("apt-get", "remove", "-y", "mpv"); err != nil {
				return err
			}
		}
	}

	fmt.Println("Done. With an SMB folder, delete <hostname>.pi-display.log from the share; your media is untouched.")
	return nil
}
