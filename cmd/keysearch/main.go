// Command keysearch finds a keyboard schedule that plays well, by trying
// random input chunks on the port and keeping the best. It exists to
// reach paths random play never does (stage clears, later stages, wind).
// The schedule must then be recorded on the original with
// scripts/refdump.sh and checked with cmd/refcompare: the port is only
// the search engine here, never the judge.
//
//	go run ./cmd/keysearch -seed 1 -stages 3 > /tmp/win.keys
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"

	"go-msx-penguin/internal/game"
)

const (
	row8  = 8
	space = 0x01
	left  = 0x10
	up    = 0x20
	right = 0x80
	chunk = 32
)

type press struct {
	tick int
	row  int
	mask byte
	down bool
}

func main() {
	seed := flag.Int64("seed", 1, "random seed")
	tries := flag.Int("tries", 24, "candidates per chunk")
	stages := flag.Int("stages", 2, "stop after this many stage clears")
	maxTicks := flag.Int("ticks", 40000, "give up after this many ticks")
	flag.Parse()
	r := rand.New(rand.NewSource(*seed))

	c := game.New()
	c.Boot(2)
	var sched []press
	tick := 0
	step := func(c *game.Console, t int, ps []press) {
		for _, p := range ps {
			if p.tick == t {
				if p.down {
					c.Keys[p.row] &^= p.mask
				} else {
					c.Keys[p.row] |= p.mask
				}
			}
		}
		c.Interrupt()
	}
	start := []press{{200, 0, 0x04, true}, {203, 0, 0x04, false}}
	for ; tick < 260; tick++ {
		step(c, tick, start)
	}
	sched = append(sched, start...)
	// Hold up (accelerate: $E100 is a period) from now on.
	sched = append(sched, press{tick, row8, up, true})
	step(c, tick, sched[len(sched)-1:])
	tick++

	cleared, lastStage := 0, c.RAM[0x0E0]
	for tick < *maxTicks && cleared < *stages {
		best, bestScore := []press(nil), -1<<30
		var bestC *game.Console
		for i := 0; i < *tries; i++ {
			cand := randomChunk(r, tick, c.Keys[row8])
			sim := *c
			if msg := simulate(&sim, tick, cand, step); msg != "" {
				// An engine panic is either a port bug or ROM data the
				// manifest lacks: stop and hand over the schedule that
				// reaches it, to be checked against the original.
				fmt.Fprintf(os.Stderr, "candidate panicked: %s\n", msg)
				sched = append(sched, cand...)
				tick += chunk
				cleared = *stages
				break
			}
			if s := score(c, &sim); s > bestScore {
				best, bestScore, bestC = cand, s, &sim
			}
		}
		if cleared >= *stages {
			break
		}
		sched = append(sched, best...)
		*c = *bestC
		tick += chunk
		if os.Getenv("KS_DEBUG") != "" && tick%640 < chunk {
			fmt.Fprintf(os.Stderr, "tick %d state %d/%d dist %02X%02X time %02X%02X speed %d score %d\n",
				tick, c.RAM[0], c.RAM[1], c.RAM[0x0E6], c.RAM[0x0E5], c.RAM[0x0E4], c.RAM[0x0E3], c.RAM[0x100], bestScore)
		}
		if c.RAM[0x0E0] != lastStage {
			lastStage = c.RAM[0x0E0]
			cleared++
			fmt.Fprintf(os.Stderr, "tick %d: stage %02X\n", tick, lastStage)
		}
	}
	fmt.Printf("# keysearch -seed %d -tries %d -stages %d (%d ticks)\n", *seed, *tries, *stages, tick)
	for _, p := range sched {
		dir := "up"
		if p.down {
			dir = "down"
		}
		fmt.Printf("%d:%d:0x%02X:%s\n", p.tick, p.row, p.mask, dir)
	}
	fmt.Fprintf(os.Stderr, "%d ticks, %d stages cleared\n", tick, cleared)
}

// simulate runs one chunk, returning a panic message if the engine
// panics.
func simulate(c *game.Console, t0 int, ps []press, step func(*game.Console, int, []press)) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint(r)
		}
	}()
	for t := t0; t < t0+chunk; t++ {
		step(c, t, ps)
	}
	return ""
}

// randomChunk returns presses for one chunk: steering changes and short
// jumps, keeping up held. row8 is the current key row (clear = held).
func randomChunk(r *rand.Rand, t0 int, row8now byte) []press {
	var ps []press
	held := map[byte]bool{left: row8now&left == 0, right: row8now&right == 0}
	set := func(t int, m byte, on bool) {
		if held[m] != on {
			held[m] = on
			ps = append(ps, press{t, row8, m, on})
		}
	}
	for t := t0; t < t0+chunk; t += 4 + r.Intn(8) {
		switch r.Intn(4) {
		case 0:
			set(t, left, true)
			set(t, right, false)
		case 1:
			set(t, right, true)
			set(t, left, false)
		default:
			set(t, left, false)
			set(t, right, false)
		}
		if r.Intn(5) == 0 && t+2 < t0+chunk {
			ps = append(ps, press{t, row8, space, true}, press{t + 2, row8, space, false})
		}
	}
	// Leave steering released at the chunk end.
	set(t0+chunk-1, left, false)
	set(t0+chunk-1, right, false)
	return ps
}

// score rewards distance covered (BCD $E0E5/$E0E6 counts down) and a
// stage change, and punishes falls, knocks and lost speed.
func score(before, after *game.Console) int {
	dist := func(c *game.Console) int {
		b := func(v byte) int { return int(v>>4)*10 + int(v&0x0F) }
		return b(c.RAM[0x0E6])*100 + b(c.RAM[0x0E5])
	}
	s := (dist(before) - dist(after)) * 10
	if after.RAM[0x0E0] != before.RAM[0x0E0] || after.RAM[0x000] == 14 {
		s += 100000
	}
	if after.RAM[0x140] != 0 {
		s -= 5000
	}
	if after.RAM[0x142] != 0 {
		s -= 2000
	}
	if after.RAM[0x000] == 12 || after.RAM[0x000] == 13 {
		s -= 50000
	}
	s -= int(after.RAM[0x100]) * 5 // lower period is faster
	return s
}
