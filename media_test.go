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
