package main

import (
	"cmp"
	"hash/maphash"
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

// playOrder decides what plays after current. Position is always derived
// from the current file's name rather than an index, so files can be added or
// removed mid-show without any playlist bookkeeping. Shuffled, names are
// ordered by a seeded hash, and each pass through the folder gets a new seed.
type playOrder struct {
	shuffle bool
	seed    maphash.Seed
}

func newPlayOrder() *playOrder { return &playOrder{seed: maphash.MakeSeed()} }

func (o *playOrder) next(names []string, current string) string {
	next, wrapped := nextAfter(names, current, o.compare())
	if wrapped && o.shuffle {
		o.seed = maphash.MakeSeed()
		order := o.compare()
		next, _ = nextAfter(names, "", order)
		// Don't play the same file twice in a row across the reshuffle.
		if next == current && len(names) > 1 {
			next, _ = nextAfter(names, next, order)
		}
	}
	return next
}

func (o *playOrder) compare() func(a, b string) int {
	if !o.shuffle {
		return naturalCompare
	}
	seed := o.seed
	return func(a, b string) int {
		return cmp.Or(cmp.Compare(maphash.String(seed, a), maphash.String(seed, b)), naturalCompare(a, b))
	}
}

// nextAfter returns the first name after current in order, or the first name
// overall when current is "". wrapped reports that nothing came after current,
// so the show is starting over. names may be in any order.
func nextAfter(names []string, current string, order func(a, b string) int) (next string, wrapped bool) {
	if current != "" {
		found := false
		for _, name := range names {
			if order(name, current) > 0 && (!found || order(name, next) < 0) {
				next, found = name, true
			}
		}
		if found {
			return next, false
		}
		wrapped = true
	}
	return slices.MinFunc(names, order), wrapped
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
