package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// version is stamped by the release workflow with -ldflags "-X main.version=v1.2.3".
var version = "dev"

const latestReleaseURL = "https://api.github.com/repos/maks112v/pi-display/releases/latest"

type release struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func (r release) assetURL(name string) (string, error) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL, nil
		}
	}
	return "", fmt.Errorf("release %s has no %s", r.Tag, name)
}

// update replaces the installed binary with the latest GitHub release after
// checking its SHA-256, then restarts the service. Settings are untouched.
func update(ctx context.Context) error {
	if os.Geteuid() != 0 {
		return errors.New("run update with sudo")
	}
	asset, err := assetName(runtime.GOARCH)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	var latest release
	data, err := fetch(ctx, latestReleaseURL, 1<<20)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &latest); err != nil {
		return fmt.Errorf("read latest release: %w", err)
	}
	if latest.Tag == version {
		fmt.Printf("Already on the latest version, %s.\n", version)
		return nil
	}

	binURL, err := latest.assetURL(asset)
	if err != nil {
		return err
	}
	sumsURL, err := latest.assetURL("SHA256SUMS")
	if err != nil {
		return err
	}
	fmt.Printf("Downloading %s...\n", latest.Tag)
	bin, err := fetch(ctx, binURL, 128<<20)
	if err != nil {
		return err
	}
	sums, err := fetch(ctx, sumsURL, 64<<10)
	if err != nil {
		return err
	}
	if err := verifyChecksum(sums, asset, bin); err != nil {
		return err
	}

	if err := replaceBinary(bin); err != nil {
		return err
	}
	// Rewrite the service too, so changes to it reach Pis set up earlier.
	if username, err := unitUser(); err == nil {
		if err := writeUnit(username); err != nil {
			return err
		}
		if err := runCmd("systemctl", "restart", "pi-display"); err != nil {
			return err
		}
	}
	fmt.Printf("Updated %s to %s.\n", version, latest.Tag)
	return nil
}

// assetName maps Go's architecture to the release asset, which is named
// after Debian's architecture so the README's curl command can find it too.
func assetName(goarch string) (string, error) {
	switch goarch {
	case "arm64":
		return "pi-display-arm64", nil
	case "arm":
		return "pi-display-armhf", nil
	}
	return "", fmt.Errorf("no release build for %s", goarch)
}

// verifyChecksum checks data against its line in a `sha256sum` output file.
func verifyChecksum(sums []byte, name string, data []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(sums))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != strings.ToLower(fields[0]) {
			return fmt.Errorf("checksum mismatch for %s; not installing", name)
		}
		return nil
	}
	return fmt.Errorf("SHA256SUMS has no entry for %s", name)
}

// fetch GETs url and returns the body, refusing anything over limit bytes.
func fetch(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "pi-display/"+version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("GET %s: response larger than %d bytes", url, limit)
	}
	return data, nil
}
