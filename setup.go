package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

const (
	binPath  = "/usr/local/bin/pi-display"
	unitPath = "/etc/systemd/system/pi-display.service"
)

// The service runs as the account that ran setup (already in the video and
// render groups on Raspberry Pi OS) and is sandboxed to its state directory.
const unitTemplate = `[Unit]
Description=pi-display slideshow
After=local-fs.target

[Service]
User=%s
SupplementaryGroups=video render audio
ExecStart=` + binPath + ` run
Restart=always
RestartSec=3
StateDirectory=pi-display
RuntimeDirectory=pi-display
Environment=HOME=/var/lib/pi-display
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=read-only
PrivateTmp=yes

[Install]
WantedBy=multi-user.target
`

// setup asks for the media source, checks it works, and installs a service
// that starts the slideshow on every boot. Safe to re-run to change source.
func setup(ctx context.Context) error {
	if os.Geteuid() != 0 {
		return errors.New("run setup with sudo")
	}
	account, err := serviceAccount()
	if err != nil {
		return err
	}
	in := bufio.NewReader(os.Stdin)

	if _, err := exec.LookPath("mpv"); err != nil {
		fmt.Println("Installing mpv...")
		if err := runCmd("apt-get", "update"); err != nil {
			return err
		}
		if err := runCmd("apt-get", "install", "-y", "mpv"); err != nil {
			return err
		}
	}

	device, err := askSource(ctx, in)
	if err != nil {
		return err
	}
	uid, _ := strconv.Atoi(account.Uid)
	gid, _ := strconv.Atoi(account.Gid)
	if err := saveDevice(devicePath, device, uid, gid); err != nil {
		return err
	}
	if err := installBinary(); err != nil {
		return err
	}
	if err := os.WriteFile(unitPath, fmt.Appendf(nil, unitTemplate, account.Username), 0o644); err != nil {
		return err
	}

	// mpv draws straight to the display, which only works when no desktop
	// session holds the screen.
	if target, _ := exec.Command("systemctl", "get-default").Output(); strings.TrimSpace(string(target)) == "graphical.target" {
		if ok, err := askYes(in, "The slideshow needs the screen, so the desktop must not start at boot. Boot to console instead? [Y/n] "); err != nil {
			return err
		} else if ok {
			if err := runCmd("systemctl", "set-default", "multi-user.target"); err != nil {
				return err
			}
			fmt.Println("Desktop disabled at boot. Undo with: sudo systemctl set-default graphical.target")
		} else {
			fmt.Println("Leaving the desktop on; the slideshow will fail until it is off.")
		}
	}

	for _, args := range [][]string{{"daemon-reload"}, {"enable", "pi-display"}, {"restart", "pi-display"}} {
		if err := runCmd("systemctl", args...); err != nil {
			return err
		}
	}
	fmt.Println("Done. The slideshow starts on every boot. Logs: journalctl -u pi-display -f")
	return nil
}

func serviceAccount() (*user.User, error) {
	name := os.Getenv("SUDO_USER")
	if name == "" || name == "root" {
		return nil, errors.New("run setup via sudo from the account the slideshow should run as")
	}
	return user.Lookup(name)
}

func askSource(ctx context.Context, in *bufio.Reader) (Device, error) {
	source, err := ask(in, "Media folder (smb://host/share/path or /local/path): ")
	if err != nil {
		return Device{}, err
	}
	d := Device{Source: source}

	if !isSMB(source) {
		if !filepath.IsAbs(source) {
			return d, errors.New("local folder must be an absolute path")
		}
		names, err := listMedia(source)
		if err != nil {
			return d, err
		}
		fmt.Printf("Found %d images and videos.\n", len(names))
		return d, nil
	}

	if _, err := parseSMB(source); err != nil {
		return d, err
	}
	if d.Username, err = ask(in, "Username (blank for guest): "); err != nil {
		return d, err
	}
	if d.Username != "" {
		fmt.Print("Password: ")
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return d, err
		}
		d.Password = string(password)
	}

	fmt.Println("Connecting...")
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	src, closeFn, err := connectSMB(ctx, d)
	if err != nil {
		return d, fmt.Errorf("connect to %s: %w", source, err)
	}
	defer closeFn()
	entries, err := fs.ReadDir(src, ".")
	if err != nil {
		return d, fmt.Errorf("list %s: %w", source, err)
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && isMedia(e.Name()) {
			count++
		}
	}
	fmt.Printf("Connected. Found %d images and videos.\n", count)
	return d, nil
}

func ask(in *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	line, err := in.ReadString('\n')
	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func askYes(in *bufio.Reader, prompt string) (bool, error) {
	answer, err := ask(in, prompt)
	if err != nil {
		return false, err
	}
	answer = strings.ToLower(answer)
	return answer == "" || answer == "y" || answer == "yes", nil
}

// installBinary copies this executable to binPath so the service doesn't
// depend on wherever setup was run from.
func installBinary() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if self, err = filepath.EvalSymlinks(self); err != nil {
		return err
	}
	if self == binPath {
		return nil
	}
	data, err := os.ReadFile(self)
	if err != nil {
		return err
	}
	tmp := binPath + ".new"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, binPath)
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
