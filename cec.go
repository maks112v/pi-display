package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// setTVPower turns the TV on or off over HDMI-CEC with cec-ctl, which setup
// installs. It tries every CEC adapter (a Pi 5 has one per HDMI port), since
// only the port with a TV answers, and succeeds if any of them does.
func setTVPower(ctx context.Context, on bool) error {
	devices, _ := filepath.Glob("/dev/cec*")
	if len(devices) == 0 {
		return errors.New("no CEC adapters in /dev")
	}
	var errs []error
	for _, dev := range devices {
		err := tvPower(ctx, dev, on)
		if err == nil {
			return nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", dev, err))
	}
	return errors.Join(errs...)
}

// tvPower sends one adapter's commands. The Pi registers as a playback
// device first, which also reports its physical address. Waking sends Image
// View On, then Active Source so the TV switches to the Pi's input.
func tvPower(ctx context.Context, dev string, on bool) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := cecCtl(ctx, dev, "--playback")
	if err != nil {
		return err
	}
	if !on {
		_, err := cecCtl(ctx, dev, "--to", "0", "--standby")
		return err
	}
	if _, err := cecCtl(ctx, dev, "--to", "0", "--image-view-on"); err != nil {
		return err
	}
	if addr, ok := physicalAddress(out); ok {
		_, err = cecCtl(ctx, dev, "--to", "15", "--active-source", "phys-addr="+addr)
	}
	return err
}

func cecCtl(ctx context.Context, dev string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "cec-ctl", append([]string{"-d", dev}, args...)...).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return "", fmt.Errorf("cec-ctl %s: %w: %s", strings.Join(args, " "), err, lastLine(msg))
		}
		return "", fmt.Errorf("cec-ctl %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// physicalAddress reads the adapter's HDMI address, like 1.0.0.0, from
// cec-ctl's status output. f.f.f.f means the TV isn't reporting one.
func physicalAddress(out string) (string, bool) {
	for line := range strings.Lines(out) {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "Physical Address" {
			continue
		}
		addr := strings.TrimSpace(value)
		return addr, addr != "" && addr != "f.f.f.f"
	}
	return "", false
}

func lastLine(s string) string {
	return s[strings.LastIndex(s, "\n")+1:]
}
