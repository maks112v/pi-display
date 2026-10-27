package main

import (
	"bufio"
	"context"
	"encoding/base64"
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

// setup gets the media source, checks it works, and installs a service that
// starts the slideshow on every boot. Safe to re-run to change source.
//
// With a setup code from the web config generator it asks nothing: the code
// carries the source, login, and slideshow settings, and the desktop is
// turned off at boot if needed. Without one it prompts for the source.
func setup(ctx context.Context, code string) error {
	if os.Geteuid() != 0 {
		return errors.New("run setup with sudo")
	}
	account, err := serviceAccount()
	if err != nil {
		return err
	}
	in := bufio.NewReader(os.Stdin)

	var device Device
	if code != "" {
		device, err = decodeSetupCode(code)
	} else {
		device, err = askSource(in)
	}
	if err != nil {
		return err
	}
	if err := checkSource(ctx, device); err != nil {
		return err
	}

	if _, err := exec.LookPath("mpv"); err != nil {
		fmt.Println("Installing mpv...")
		if err := runCmd("apt-get", "update"); err != nil {
			return err
		}
		if err := runCmd("apt-get", "install", "-y", "mpv"); err != nil {
			return err
		}
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
		ok := code != ""
		if !ok {
			if ok, err = askYes(in, "The slideshow needs the screen, so the desktop must not start at boot. Boot to console instead? [Y/n] "); err != nil {
				return err
			}
		}
		if ok {
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

// decodeSetupCode reads the base64 code the web config generator produces:
// a Device as JSON, including its slideshow settings.
func decodeSetupCode(code string) (Device, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(code))
	if err != nil {
		return Device{}, fmt.Errorf("setup code is not valid base64: %w", err)
	}
	d, err := parseDevice(data)
	if err != nil {
		return d, fmt.Errorf("setup code: %w", err)
	}
	return d, nil
}

func askSource(in *bufio.Reader) (Device, error) {
	source, err := ask(in, "Media folder (smb://host/share/path, a public Google Drive folder link, or /local/path): ")
	if err != nil {
		return Device{}, err
	}
	d := Device{Source: source}
	if !isSMB(source) {
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
	return d, nil
}

// checkSource makes sure the source is well formed and reachable, so setup
// fails on a bad link or password instead of the service failing later.
func checkSource(ctx context.Context, d Device) error {
	switch {
	case isDrive(d.Source):
		if _, err := parseDriveFolder(d.Source); err != nil {
			return err
		}
	case isSMB(d.Source):
		if _, err := parseSMB(d.Source); err != nil {
			return err
		}
	default:
		if !filepath.IsAbs(d.Source) {
			return errors.New("local folder must be an absolute path")
		}
		names, err := listMedia(d.Source)
		if err != nil {
			return err
		}
		fmt.Printf("Found %d images and videos.\n", len(names))
		return nil
	}
	return checkRemote(ctx, d)
}

// checkRemote lists a remote source once so setup fails on a bad link or
// password instead of the service failing quietly later.
func checkRemote(ctx context.Context, d Device) error {
	fmt.Println("Connecting...")
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	src, closeFn, err := connect(ctx, d)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", d.Source, err)
	}
	defer closeFn()
	entries, err := fs.ReadDir(src, ".")
	if err != nil {
		return fmt.Errorf("list %s: %w", d.Source, err)
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && isMedia(e.Name()) {
			count++
		}
	}
	fmt.Printf("Connected. Found %d images and videos.\n", count)
	return nil
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
	return replaceBinary(data)
}

// replaceBinary writes a new executable next to binPath and renames it into
// place. The rename is atomic and safe while the old binary is running.
func replaceBinary(data []byte) error {
	tmp := binPath + ".new"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp, binPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
