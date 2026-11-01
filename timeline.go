package main

import (
	"cmp"
	"context"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// Screens on the same folder stay in step by playing a timeline laid against
// the wall clock instead of timing each file from when it loaded. The folder
// loops back to back from the Unix epoch, each file taking its image duration
// or video length, so every Pi with the same files, the same settings, and a
// synced clock (Raspberry Pi OS keeps it synced over the network) works out
// the same file at the same moment, with no talking between Pis. A Pi that
// boots, loads slowly, or falls behind lands back in step at the next file.

// slot is one file's turn in the timeline.
type slot struct {
	name       string
	start, end time.Time
}

// slotAt returns the slot playing at now. length gives each file's running
// time; files it reports as 0 are left out. ok is false when nothing can
// play. Shuffled, each loop gets its own order from hashing the loop number
// with the names, and the last file of one loop never plays first in the
// next.
func slotAt(names []string, length func(name string) time.Duration, shuffle bool, now time.Time) (s slot, ok bool) {
	var total time.Duration
	lengths := map[string]time.Duration{}
	var playable []string
	for _, name := range names {
		if d := length(name); d > 0 {
			lengths[name] = d
			playable = append(playable, name)
			total += d
		}
	}
	if total == 0 {
		return slot{}, false
	}

	at := now.UnixNano()
	loop, pos := at/int64(total), time.Duration(at%int64(total))
	order := loopOrder(playable, shuffle, loop)
	start := now.Add(-pos)
	for _, name := range order {
		end := start.Add(lengths[name])
		if pos < lengths[name] {
			return slot{name: name, start: start, end: end}, true
		}
		pos -= lengths[name]
		start = end
	}
	panic("unreachable: pos is less than the sum of the lengths")
}

// loopOrder is the play order for one pass through names.
func loopOrder(names []string, shuffle bool, loop int64) []string {
	if !shuffle {
		return slices.SortedFunc(slices.Values(names), naturalCompare)
	}
	order := shuffled(names, loop)
	if len(order) > 1 && order[0] == shuffled(names, loop-1)[len(order)-1] {
		order[0], order[1] = order[1], order[0]
	}
	return order
}

// shuffled orders names by a hash of each name with loop, the same on every Pi.
func shuffled(names []string, loop int64) []string {
	key := func(name string) uint64 {
		h := fnv.New64a()
		_ = binary.Write(h, binary.LittleEndian, loop)
		h.Write([]byte(name))
		return h.Sum64()
	}
	return slices.SortedFunc(slices.Values(names), func(a, b string) int {
		return cmp.Or(cmp.Compare(key(a), key(b)), naturalCompare(a, b))
	})
}

// settleTime is how old a synced file must be before it joins the timeline,
// and how long a deleted one stays (see mirror). Pis sync on their own 30
// second cycles, so without it each would add or drop a file at a different
// moment and play different files until all had caught up. Both are timed
// from the source's clock, so every Pi agrees on when a change lands.
const settleTime = time.Minute

// settled drops files in dir saved within settle of now. If every file is
// that new (a freshly filled folder) they all play rather than none. A
// modified time in the future means the source's clock is ahead, so those
// files play right away instead of waiting for it.
func settled(dir string, names []string, settle time.Duration, now time.Time) []string {
	if settle == 0 {
		return names
	}
	var old []string
	for _, name := range names {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if age := now.Sub(info.ModTime()); age < 0 || age >= settle {
			old = append(old, name)
		}
	}
	if len(old) == 0 {
		return names
	}
	return old
}

// videoLengths measures each video once and remembers it, keyed by name,
// size, and modified time so a replaced file is measured again. A video that
// can't be measured is remembered as 0 and left out of the timeline, rather
// than probed again before every file.
type videoLengths struct {
	probe func(ctx context.Context, path string) (time.Duration, error)
	known map[string]time.Duration
}

func newVideoLengths() *videoLengths {
	return &videoLengths{probe: probeDuration, known: map[string]time.Duration{}}
}

func (v *videoLengths) length(ctx context.Context, dir, name string) time.Duration {
	path := filepath.Join(dir, name)
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	key := fmt.Sprintf("%s\x00%d\x00%d", name, info.Size(), info.ModTime().UnixNano())
	if d, ok := v.known[key]; ok {
		return d
	}
	d, err := v.probe(ctx, path)
	if err != nil {
		if ctx.Err() != nil {
			return 0 // shutting down; measure it next time
		}
		log.Printf("skipping %s: couldn't read its length: %v", name, err)
	}
	v.known[key] = d
	return d
}
