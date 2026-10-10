// Command keygen writes a pseudo-random key schedule for scripted traces
// (format tick:row:mask:down|up, as read by scripts/refdump.tcl and
// cmd/refcompare). It presses 2 on the title (keyboard play), then mostly
// accelerates while steering and jumping at random.
//
//	go run ./cmd/keygen -seed 1 -ticks 8000 > /tmp/play1.keys
package main

import (
	"flag"
	"fmt"
	"math/rand"
)

// Keyboard matrix bits used by the keyboard mode ($40A4).
const (
	row8  = 8
	space = 0x01
	left  = 0x10
	up    = 0x20
	down  = 0x40
	right = 0x80
)

func main() {
	seed := flag.Int64("seed", 1, "random seed")
	ticks := flag.Int("ticks", 8000, "schedule length")
	start := flag.Int("start", 200, "tick to press 2 (keyboard play)")
	flag.Parse()
	r := rand.New(rand.NewSource(*seed))

	fmt.Printf("# keygen -seed %d -ticks %d -start %d\n", *seed, *ticks, *start)
	fmt.Printf("%d:0:0x04:down\n%d:0:0x04:up\n", *start, *start+3)
	held := map[byte]bool{}
	set := func(t int, mask byte, on bool) {
		if held[mask] == on {
			return
		}
		held[mask] = on
		dir := "up"
		if on {
			dir = "down"
		}
		fmt.Printf("%d:%d:0x%02X:%s\n", t, row8, mask, dir)
	}
	for t := *start + 60; t < *ticks; t += 4 + r.Intn(20) {
		set(t, down, r.Intn(10) < 7)
		set(t, up, !held[down] && r.Intn(4) == 0)
		switch r.Intn(5) {
		case 0:
			set(t, left, true)
			set(t, right, false)
		case 1:
			set(t, right, true)
			set(t, left, false)
		case 2:
			set(t, left, false)
			set(t, right, false)
		}
		if r.Intn(6) == 0 {
			set(t, space, true)
			set(t+2, space, false) // a 2-tick press
			held[space] = false
		}
	}
}
