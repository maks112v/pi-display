package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// splashPNG is the brand icon (marketing/public/favicon.svg) drawn in white
// on transparent, rendered at 512px.
//
//go:embed splash.png
var splashPNG []byte

const splashDir = "/usr/share/plymouth/themes/pi-display"

const splashTheme = `[Plymouth Theme]
Name=Pi Display
Description=Pi Display logo on black
ModuleName=script

[script]
ImageDir=` + splashDir + `
ScriptFile=` + splashDir + `/pi-display.script
`

// The logo is centered and a sixth of the screen's height at any resolution.
const splashScript = `Window.SetBackgroundTopColor(0, 0, 0);
Window.SetBackgroundBottomColor(0, 0, 0);
size = Window.GetHeight() / 6;
logo.image = Image("splash.png").Scale(size, size);
logo.sprite = Sprite(logo.image);
logo.sprite.SetX(Window.GetX() + (Window.GetWidth() - size) / 2);
logo.sprite.SetY(Window.GetY() + (Window.GetHeight() - size) / 2);
`

// installSplash makes the Pi Display logo the boot splash. It does nothing
// without Plymouth (Raspberry Pi OS Lite), and only rebuilds the initramfs,
// which takes a minute, when the theme isn't already in place.
func installSplash() error {
	if _, err := exec.LookPath("plymouth-set-default-theme"); err != nil {
		return nil
	}
	if err := os.MkdirAll(splashDir, 0o755); err != nil {
		return err
	}
	changed := false
	for name, data := range map[string][]byte{
		"pi-display.plymouth": []byte(splashTheme),
		"pi-display.script":   []byte(splashScript),
		"splash.png":          splashPNG,
	} {
		path := filepath.Join(splashDir, name)
		if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, data) {
			continue
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		changed = true
	}
	if !changed && currentSplash() == "pi-display" {
		return nil
	}
	fmt.Println("Setting the boot logo. Rebuilding the initramfs takes a minute...")
	return setSplash("pi-display")
}

// removeSplash puts Raspberry Pi OS's own splash back if setup set ours.
func removeSplash() error {
	if _, err := os.Stat(splashDir); err != nil {
		return nil
	}
	if currentSplash() == "pi-display" {
		fmt.Println("Restoring the Raspberry Pi boot logo...")
		theme := "--reset"
		if _, err := os.Stat("/usr/share/plymouth/themes/pix"); err == nil {
			theme = "pix"
		}
		if err := setSplash(theme); err != nil {
			return err
		}
	}
	return os.RemoveAll(splashDir)
}

func currentSplash() string {
	out, _ := exec.Command("plymouth-set-default-theme").Output()
	return strings.TrimSpace(string(out))
}

// setSplash sets the Plymouth theme and rebuilds the initramfs it boots from.
// The rebuild is noisy, so its output only shows on failure.
func setSplash(theme string) error {
	if out, err := exec.Command("plymouth-set-default-theme", "-R", theme).CombinedOutput(); err != nil {
		return fmt.Errorf("plymouth-set-default-theme -R %s: %w\n%s", theme, err, out)
	}
	return nil
}
