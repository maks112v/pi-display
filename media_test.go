package main

import (
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
		if got := nextAfter(names, current); got != want {
			t.Errorf("nextAfter(%q) = %q, want %q", current, got, want)
		}
	}
}
