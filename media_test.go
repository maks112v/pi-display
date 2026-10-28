package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNaturalOrder(t *testing.T) {
	names := []string{"10.jpg", "b.mp4", "2.jpg", "A.png", "02.jpg", "1.jpg", "img10.png", "img9.png"}
	slices.SortFunc(names, naturalCompare)
	want := []string{"1.jpg", "02.jpg", "2.jpg", "10.jpg", "A.png", "b.mp4", "img9.png", "img10.png"}
	if !slices.Equal(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}

func TestNextAfter(t *testing.T) {
	names := []string{"1.jpg", "2.jpg", "10.jpg"}
	for current, want := range map[string]string{
		"":       "1.jpg",
		"1.jpg":  "2.jpg",
		"2.jpg":  "10.jpg",
		"10.jpg": "1.jpg",  // wraps
		"5.jpg":  "10.jpg", // current was deleted mid-show
	} {
		if got := newPlayOrder().next(names, current); got != want {
			t.Errorf("nextAfter(%q) = %q, want %q", current, got, want)
		}
	}
}

func TestShuffle(t *testing.T) {
	names := []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg", "f.jpg", "g.jpg", "h.jpg"}
	order := newPlayOrder()
	order.shuffle = true

	// Group plays into passes by the seed in effect when each file was picked.
	var passes [][]string
	var pass []string
	seed, current := order.seed, ""
	for range 60 {
		next := order.next(names, current)
		if next == current {
			t.Fatalf("%s played twice in a row", next)
		}
		if order.seed != seed {
			passes = append(passes, pass)
			pass, seed = nil, order.seed
		}
		pass = append(pass, next)
		current = next
	}

	for i, pass := range passes {
		// A pass skips its first file only when that file just played.
		if len(pass) < len(names)-1 || len(slices.Compact(slices.Sorted(slices.Values(pass)))) != len(pass) {
			t.Fatalf("pass %d = %v, want each file once", i, pass)
		}
	}
	if len(passes) < 2 || slices.EqualFunc(passes[1:], passes[:len(passes)-1], slices.Equal) {
		t.Error("every pass had the same order; expected a reshuffle")
	}
}

func TestListMediaSkipsAnimatedWebP(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		// VP8X chunk with the animation flag (0x02) set, as GIPHY serves them.
		"giphy.webp": "RIFF\x00\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00\x12\x00\x00\x00",
		// VP8X without it: a still image with alpha or metadata.
		"still.webp": "RIFF\x00\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00\x10\x00\x00\x00",
		"photo.jpg":  "",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	names, unplayable, err := listMedia(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"photo.jpg", "still.webp"}; !slices.Equal(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
	if want := []string{"giphy.webp"}; !slices.Equal(unplayable, want) {
		t.Errorf("unplayable = %v, want %v", unplayable, want)
	}
}
