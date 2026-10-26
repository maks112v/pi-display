package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var (
	imageExts = []string{".jpg", ".jpeg", ".png", ".webp", ".bmp", ".gif"}
	videoExts = []string{".mp4", ".m4v", ".mov", ".mkv", ".webm", ".avi"}
)

func isImage(name string) bool {
	return slices.Contains(imageExts, strings.ToLower(filepath.Ext(name)))
}

func isMedia(name string) bool {
	return isImage(name) || slices.Contains(videoExts, strings.ToLower(filepath.Ext(name)))
}

// listMedia returns the playable files directly inside dir, in natural order.
// Hidden files are skipped, which also hides in-progress sync downloads.
func listMedia(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && !strings.HasPrefix(e.Name(), ".") && isMedia(e.Name()) {
			names = append(names, e.Name())
		}
	}
	slices.SortFunc(names, naturalCompare)
	return names, nil
}

// nextAfter picks the first name that sorts after current, wrapping to the
// start. Deriving position from the name instead of an index means files can
// be added or removed mid-show without any playlist bookkeeping.
func nextAfter(names []string, current string) string {
	for _, name := range names {
		if naturalCompare(name, current) > 0 {
			return name
		}
	}
	return names[0]
}

// naturalCompare orders names the way people number them: "2.jpg" before
// "10.jpg", case-insensitively. Exact ties fall back to byte order.
func naturalCompare(a, b string) int {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	for la != "" && lb != "" {
		ca, cb := leadingChunk(la), leadingChunk(lb)
		if ca != cb {
			if isDigit(ca[0]) && isDigit(cb[0]) {
				na, nb := strings.TrimLeft(ca, "0"), strings.TrimLeft(cb, "0")
				if len(na) != len(nb) {
					return len(na) - len(nb)
				}
				if c := strings.Compare(na, nb); c != 0 {
					return c
				}
			}
			return strings.Compare(ca, cb)
		}
		la, lb = la[len(ca):], lb[len(cb):]
	}
	if c := len(la) - len(lb); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

// leadingChunk returns the leading run of all-digit or all-non-digit bytes.
func leadingChunk(s string) string {
	digit := isDigit(s[0])
	i := 1
	for i < len(s) && isDigit(s[i]) == digit {
		i++
	}
	return s[:i]
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }
