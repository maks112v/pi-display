package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
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
	"syscall"
	"time"

	"golang.org/x/term"
)

const (
	binPath  = "/usr/local/bin/pi-display"
	unitPath = "/etc/systemd/system/pi-display.service"
)

// The service runs as the account that ran setup (already in the video and
// render groups on Raspberry Pi OS) and is sandboxed to its state directory.
// It starts after the boot splash quits, like getty does: mpv started while
// Plymouth holds the screen loses it when the splash exits, leaving the
// console showing.
const unitTemplate = `[Unit]
Description=pi-display slideshow
After=local-fs.target plymouth-quit-wait.service

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
// With a setup code from the website's install page it asks nothing: the
// code carries the source and login, and the desktop is turned off at boot if
// needed. Without one it prompts for the source. Either way a Pi that was set
// up before keeps its slideshow settings unless the code carries new ones.
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
	if device.Show == nil {
		if current, err := loadDevice(devicePath); err == nil {
			device.Show = current.Show
		}
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
	stopDesktop := false
	if target, _ := exec.Command("systemctl", "get-default").Output(); strings.TrimSpace(string(target)) == "graphical.target" {
		ok := code != ""
		if !ok {
			if ok, err = askYes(in, "The slideshow needs the screen, so the desktop must not start at boot. Boot to console instead? [Y/n] ", true); err != nil {
				return err
			}
		}
		if ok {
			if err := runCmd("systemctl", "set-default", "multi-user.target"); err != nil {
				return err
			}
			fmt.Println("Desktop disabled at boot. Undo with: sudo systemctl set-default graphical.target")
			stopDesktop = exec.Command("systemctl", "is-active", "--quiet", "display-manager").Run() == nil
		} else {
			fmt.Println("Leaving the desktop on; the slideshow will fail until it is off.")
		}
	}

	for _, args := range [][]string{{"daemon-reload"}, {"enable", "pi-display"}, {"restart", "pi-display"}} {
		if err := runCmd("systemctl", args...); err != nil {
			return err
		}
	}
	fmt.Println("Done. The slideshow is starting now and on every boot. Logs: pi-display logs")

	// The running desktop still holds the screen until it stops. Stop it last
	// and without waiting, since setup may be running in one of its terminals.
	// The service's Restart=always picks up the screen once it is free.
	if stopDesktop {
		fmt.Println("Stopping the desktop so the slideshow can take the screen.")
		return runCmd("systemctl", "--no-block", "stop", "display-manager")
	}
	return nil
}

func serviceAccount() (*user.User, error) {
	name := os.Getenv("SUDO_USER")
	if name == "" || name == "root" {
		return nil, errors.New("run setup via sudo from the account the slideshow should run as")
	}
	return user.Lookup(name)
}

// decodeSetupCode reads the base64 code the website's install page produces:
// a Device as JSON. Older codes also carry slideshow settings.
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

// askYes asks a yes/no question; an empty answer means def.
func askYes(in *bufio.Reader, prompt string, def bool) (bool, error) {
	answer, err := ask(in, prompt)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(answer) {
	case "":
		return def, nil
	case "y", "yes":
		return true, nil
	}
	return false, nil
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

// applyConfig saves slideshow settings from a code made by the website's
// config generator ("Apply to this Pi only") and restarts the slideshow. The
// source and login stay as they are, and display.toml still applies on top.
func applyConfig(code string) error {
	if os.Geteuid() != 0 {
		return errors.New("run config with sudo")
	}
	if code == "" {
		return errors.New("usage: sudo pi-display config <code>")
	}
	show, err := decodeConfigCode(code)
	if err != nil {
		return err
	}
	device, err := loadDevice(devicePath)
	if err != nil {
		return fmt.Errorf("load %s (run `sudo pi-display setup` first): %w", devicePath, err)
	}
	device.Show = &show

	// Keep the file owned by the account the slideshow runs as.
	info, err := os.Stat(devicePath)
	if err != nil {
		return err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("can't read the owner of %s", devicePath)
	}
	if err := saveDevice(devicePath, device, int(owner.Uid), int(owner.Gid)); err != nil {
		return err
	}
	if err := runCmd("systemctl", "restart", "pi-display"); err != nil {
		return err
	}
	fmt.Println("Saved. The slideshow restarted with the new settings.")
	return nil
}

// decodeConfigCode reads the base64 code the config generator produces: a
// Show as JSON. Settings it leaves out keep their defaults.
func decodeConfigCode(code string) (Show, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(code))
	if err != nil {
		return Show{}, fmt.Errorf("config code is not valid base64: %w", err)
	}
	var s Show
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("config code: %w", err)
	}
	if err := s.validate(); err != nil {
		return s, fmt.Errorf("config code: %w", err)
	}
	return s, nil
}
