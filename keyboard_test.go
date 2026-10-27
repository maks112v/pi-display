package main

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestReadKeys(t *testing.T) {
	// keys builds a stream of key events as {code, value} pairs.
	keys := func(events ...[2]int) []byte {
		var b bytes.Buffer
		for _, e := range events {
			ev := make([]byte, inputEventSize)
			tail := ev[len(ev)-8:]
			binary.NativeEndian.PutUint16(tail, evKey)
			binary.NativeEndian.PutUint16(tail[2:], uint16(e[0]))
			binary.NativeEndian.PutUint32(tail[4:], uint32(e[1]))
			b.Write(ev)
		}
		return b.Bytes()
	}
	tests := []struct {
		name   string
		stream []byte
		want   bool
	}{
		{"ctrl+alt+t", keys([2]int{keyLeftCtrl, 1}, [2]int{keyLeftAlt, 1}, [2]int{keyT, 1}), true},
		{"right ctrl+alt+f2", keys([2]int{keyRightCtrl, 1}, [2]int{keyRightAlt, 1}, [2]int{keyF1 + 1, 1}), true},
		{"t alone", keys([2]int{keyT, 1}), false},
		{"ctrl released first", keys([2]int{keyLeftCtrl, 1}, [2]int{keyLeftAlt, 1}, [2]int{keyLeftCtrl, 0}, [2]int{keyT, 1}), false},
		{"ctrl+alt+other key", keys([2]int{keyLeftCtrl, 1}, [2]int{keyLeftAlt, 1}, [2]int{30, 1}), false},
	}
	for _, tt := range tests {
		got := false
		readKeys(bytes.NewReader(tt.stream), func() { got = true })
		if got != tt.want {
			t.Errorf("%s: exit = %v, want %v", tt.name, got, tt.want)
		}
	}
}
