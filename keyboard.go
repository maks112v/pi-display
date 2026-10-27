package main

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"
)

// Event type and key codes from linux/input-event-codes.h.
const (
	evKey        = 1
	keyLeftCtrl  = 29
	keyRightCtrl = 97
	keyLeftAlt   = 56
	keyRightAlt  = 100
	keyT         = 20
	keyF1        = 59
	keyF10       = 68
	keyF11       = 87
	keyF12       = 88
)

// inputEventSize is sizeof(struct input_event): a timeval of two C longs,
// then a u16 type, u16 code, and s32 value.
var inputEventSize = 2*int(unsafe.Sizeof(uint(0))) + 8

// watchKeys calls onExit once someone presses Ctrl+Alt+T, or Ctrl+Alt+F1 to
// F12 (the usual console keys), on any keyboard plugged into the Pi,
// including ones plugged in later. It runs for the life of the process. The
// service account reads /dev/input through the input group, which Raspberry
// Pi OS puts every user in.
func watchKeys(onExit func()) {
	var once sync.Once
	exit := func() { once.Do(onExit) }
	open := map[string]bool{}
	closed := make(chan string)
	rescan := time.NewTicker(2 * time.Second)
	for {
		paths, _ := filepath.Glob("/dev/input/event*")
		for _, p := range paths {
			if open[p] {
				continue
			}
			f, err := os.Open(p)
			if err != nil {
				continue
			}
			open[p] = true
			go func() {
				defer f.Close()
				readKeys(f, exit)
				closed <- p
			}()
		}
		select {
		case p := <-closed:
			delete(open, p)
		case <-rescan.C:
		}
	}
}

// readKeys reads one input device until it fails, such as when the keyboard
// is unplugged, and calls exit on an exit combo. Modifiers are tracked per
// device, so the combo has to come from one keyboard.
func readKeys(r io.Reader, exit func()) {
	held := map[uint16]bool{}
	buf := make([]byte, inputEventSize)
	for {
		if _, err := io.ReadFull(r, buf); err != nil {
			return
		}
		ev := buf[len(buf)-8:]
		typ := binary.NativeEndian.Uint16(ev)
		code := binary.NativeEndian.Uint16(ev[2:])
		value := int32(binary.NativeEndian.Uint32(ev[4:])) // 0 release, 1 press, 2 repeat
		if typ != evKey {
			continue
		}
		held[code] = value != 0
		ctrl := held[keyLeftCtrl] || held[keyRightCtrl]
		alt := held[keyLeftAlt] || held[keyRightAlt]
		if value == 1 && ctrl && alt && isExitKey(code) {
			exit()
		}
	}
}

func isExitKey(code uint16) bool {
	return code == keyT || (code >= keyF1 && code <= keyF10) || code == keyF11 || code == keyF12
}
