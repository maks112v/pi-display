package main

import (
	_ "embed"
	"os"
	"path/filepath"
)

// The screens shown while the folder has nothing to play, one per kind of
// source so whoever sees it knows where to add files. Designed in Paper (Pi
// Display file, "Empty state" page) and exported at 1920x1080.
var (
	//go:embed empty-drive.png
	emptyDrivePNG []byte
	//go:embed empty-smb.png
	emptySMBPNG []byte
	//go:embed empty-local.png
	emptyLocalPNG []byte
)

// writeEmptyScreen writes the empty screen for source into dir and returns
// its path, since mpv loads files, not bytes.
func writeEmptyScreen(dir, source string) (string, error) {
	png := emptyLocalPNG
	switch {
	case isDrive(source):
		png = emptyDrivePNG
	case isSMB(source):
		png = emptySMBPNG
	}
	path := filepath.Join(dir, "empty.png")
	return path, os.WriteFile(path, png, 0o644)
}
