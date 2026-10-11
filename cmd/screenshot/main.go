// Command screenshot drives the port headless, one tick per interrupt,
// and writes stills through the shared renderer. The committed stills in
// screenshots/ are the regression baseline: after a change not meant to
// alter behaviour, regenerate into /tmp and byte-compare.
//
//	go run ./cmd/screenshot -out /tmp/shots && \
//	  for f in screenshots/*.png; do cmp "$f" "/tmp/shots/$(basename "$f")"; done
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go-msx-penguin/internal/game"
	"go-msx-penguin/internal/msxrender"
)

// stills are taken after the given number of interrupts (one tick each)
// from power-on, with the key schedule replayed if one is named.
var stills = []struct {
	name  string
	frame int
	keys  string
}{
	{"01-logo", 60, ""},
	{"02-title", 300, ""},
	{"03-stage-map", 600, ""},
	{"04-demo-play", 1100, ""},
	{"05-demo-play", 1700, ""},
	// The South Pole station after clearing stage 5, flag at the top
	// (keys found by cmd/keysearch, verified against the original).
	{"06-stage5-clear", 30175, "testdata/clear5.keys"},
}

func main() {
	out := flag.String("out", "screenshots", "output directory")
	flag.Parse()
	check(os.MkdirAll(*out, 0o755))

	c := game.New()
	c.Boot(2)
	frame := 0
	for _, s := range stills {
		if s.keys != "" {
			continue
		}
		for ; frame < s.frame; frame++ {
			c.Interrupt()
		}
		write(filepath.Join(*out, s.name+".png"), c)
	}
	for _, s := range stills {
		if s.keys == "" {
			continue
		}
		keys, err := loadKeys(s.keys)
		check(err)
		k := game.New()
		k.Boot(2)
		for n := 0; n < s.frame; n++ {
			for _, p := range keys[n] {
				if p.down {
					k.Keys[p.row] &^= p.mask
				} else {
					k.Keys[p.row] |= p.mask
				}
			}
			k.Interrupt()
		}
		write(filepath.Join(*out, s.name+".png"), k)
	}
	if c.Unported != "" {
		fmt.Println("note: the port reached unported", c.Unported)
	}
}

func write(p string, c *game.Console) {
	f, err := os.Create(p)
	check(err)
	check(png.Encode(f, msxrender.Render(c.VRAM[:], c.Regs)))
	check(f.Close())
	fmt.Println(p)
}

type press struct {
	row  int
	mask byte
	down bool
}

// loadKeys reads a key schedule (tick:row:mask:down|up, '#' comments).
func loadKeys(path string) (map[int][]press, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	keys := map[int][]press{}
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(strings.SplitN(l, "#", 2)[0])
		if l == "" {
			continue
		}
		p := strings.Split(l, ":")
		if len(p) != 4 {
			return nil, fmt.Errorf("%s: bad line %q", path, l)
		}
		t, err1 := strconv.Atoi(p[0])
		row, err2 := strconv.Atoi(p[1])
		m, err3 := strconv.ParseUint(p[2], 0, 8)
		if err1 != nil || err2 != nil || err3 != nil {
			return nil, fmt.Errorf("%s: bad line %q", path, l)
		}
		keys[t] = append(keys[t], press{row, byte(m), p[3] == "down"})
	}
	return keys, nil
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "screenshot:", err)
		os.Exit(1)
	}
}
