// Command refcompare runs the port in lockstep with a reference trace of
// the original and stops at the first differing byte.
//
//	go run ./cmd/refcompare -trace /tmp/attract.trace
//	go run ./cmd/refcompare -trace /tmp/play.trace -keys /tmp/play.keys
//
// For each record: the interrupts the original took since the previous
// tick start are replayed (all but the last nested in the previous tick,
// with the busy flag set), then the port's RAM, VRAM, VDP and PSG
// registers are compared with the record, then the port runs the tick.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go-msx-penguin/internal/game"
	"go-msx-penguin/internal/msxrender"
	"go-msx-penguin/internal/reftrace"
)

// The Z80 stack (SP starts at $E400) is not game state.
const stackLo, stackHi = 0xE380, 0xE3FF

func main() {
	tracePath := flag.String("trace", "", "reference trace from scripts/refdump.tcl")
	keysPath := flag.String("keys", "", "key schedule used for the trace (tick:row:mask:down|up)")
	outDir := flag.String("out", "/tmp/refcompare", "where to write ref/port PNGs of the first differing tick")
	maxDiffs := flag.Int("diffs", 16, "differing bytes to list")
	flag.Parse()

	r, err := reftrace.Open(*tracePath)
	check(err)
	defer r.Close()
	keys, err := loadKeys(*keysPath)
	check(err)

	c := game.New()
	c.Boot(int(r.InitIRQs))
	prev := r.InitIRQs
	var rec reftrace.Record
	n := 0
	for ; ; n++ {
		if err := r.Next(&rec); err == io.EOF {
			break
		} else {
			check(err)
		}
		d := int(rec.IRQs - prev)
		prev = rec.IRQs
		if d < 1 {
			fail(n, c, "record has %d interrupts since the previous tick", d)
		}
		for i := 0; i < d-1; i++ {
			c.PerInterrupt() // nested in the previous tick: busy is still set
		}
		c.EndTick()
		if !c.PerInterrupt() {
			fail(n, c, "port was busy at a tick start")
		}
		for _, k := range keys[n] {
			if k.down {
				c.Keys[k.row] &^= k.mask
			} else {
				c.Keys[k.row] |= k.mask
			}
		}
		if diffs := compare(&rec, c); len(diffs) > 0 {
			report(n, &rec, c, diffs, *maxDiffs, *outDir)
			os.Exit(1)
		}
		c.Tick()
	}
	fmt.Printf("refcompare: all %d ticks match", n)
	if c.Unported != "" {
		fmt.Printf(" (the port reached unported %s)", c.Unported)
	}
	fmt.Println()
}

type diff struct {
	area      string
	addr      int
	ref, port byte
}

func compare(rec *reftrace.Record, c *game.Console) []diff {
	var ds []diff
	for i := range rec.Regs {
		if rec.Regs[i] != c.Regs[i] {
			ds = append(ds, diff{"VDP R", i, rec.Regs[i], c.Regs[i]})
		}
	}
	for i := range rec.PSG {
		if i == 14 { // the joystick input port, not state
			continue
		}
		if rec.PSG[i] != c.PSG[i] {
			ds = append(ds, diff{"PSG R", i, rec.PSG[i], c.PSG[i]})
		}
	}
	for i := range rec.RAM {
		a := reftrace.RAMBase + i
		if a >= stackLo && a <= stackHi {
			continue
		}
		if rec.RAM[i] != c.RAM[i] {
			ds = append(ds, diff{"RAM", a, rec.RAM[i], c.RAM[i]})
		}
	}
	for i := range rec.VRAM {
		if rec.VRAM[i] != c.VRAM[i] {
			ds = append(ds, diff{"VRAM", i, rec.VRAM[i], c.VRAM[i]})
		}
	}
	return ds
}

func report(n int, rec *reftrace.Record, c *game.Console, ds []diff, max int, outDir string) {
	fmt.Printf("refcompare: ticks 0-%d match; tick %d differs in %d bytes (state $%02X, sub $%02X)\n",
		n-1, n, len(ds), rec.RAM[0], rec.RAM[1])
	if c.Unported != "" {
		fmt.Printf("  the port reached unported %s\n", c.Unported)
	}
	for i, d := range ds {
		if i == max {
			fmt.Printf("  ... %d more\n", len(ds)-max)
			break
		}
		fmt.Printf("  %-5s $%04X: ref $%02X port $%02X\n", d.area, d.addr, d.ref, d.port)
	}
	if err := os.MkdirAll(outDir, 0o755); err == nil {
		write(filepath.Join(outDir, fmt.Sprintf("tick%05d-ref.png", n)), rec.VRAM[:], rec.Regs)
		write(filepath.Join(outDir, fmt.Sprintf("tick%05d-port.png", n)), c.VRAM[:], c.Regs)
		fmt.Printf("  images in %s\n", outDir)
	}
}

func write(path string, vram []byte, regs [8]byte) {
	f, err := os.Create(path)
	check(err)
	check(png.Encode(f, msxrender.Render(vram, regs)))
	check(f.Close())
}

func fail(n int, c *game.Console, format string, args ...any) {
	fmt.Printf("refcompare: tick %d: %s\n", n, fmt.Sprintf(format, args...))
	if c.Unported != "" {
		fmt.Printf("  the port reached unported %s\n", c.Unported)
	}
	os.Exit(1)
}

type key struct {
	row  int
	mask byte
	down bool
}

// loadKeys reads the schedule format used by scripts/refdump.tcl.
func loadKeys(path string) (map[int][]key, error) {
	keys := map[int][]key{}
	if path == "" {
		return keys, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.SplitN(sc.Text(), "#", 2)[0])
		if line == "" {
			continue
		}
		p := strings.Split(line, ":")
		if len(p) != 4 {
			return nil, fmt.Errorf("keys: bad line %q", line)
		}
		t, err1 := strconv.Atoi(p[0])
		row, err2 := strconv.Atoi(p[1])
		mask, err3 := strconv.ParseUint(p[2], 0, 8)
		if err1 != nil || err2 != nil || err3 != nil || (p[3] != "down" && p[3] != "up") {
			return nil, fmt.Errorf("keys: bad line %q", line)
		}
		keys[t] = append(keys[t], key{row, byte(mask), p[3] == "down"})
	}
	return keys, sc.Err()
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "refcompare:", err)
		os.Exit(1)
	}
}
