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

	"go-msx-penguin/internal/game"
	"go-msx-penguin/internal/msxrender"
)

// stills are taken after the given number of interrupts from power-on.
var stills = []struct {
	name  string
	frame int
}{
	{"01-logo", 60},
	{"02-title", 300},
	{"03-stage-map", 600},
	{"04-demo-play", 1100},
	{"05-demo-play", 1700},
}

func main() {
	out := flag.String("out", "screenshots", "output directory")
	flag.Parse()
	check(os.MkdirAll(*out, 0o755))

	c := game.New()
	c.Boot(2)
	frame := 0
	for _, s := range stills {
		for ; frame < s.frame; frame++ {
			c.Interrupt()
		}
		p := filepath.Join(*out, s.name+".png")
		f, err := os.Create(p)
		check(err)
		check(png.Encode(f, msxrender.Render(c.VRAM[:], c.Regs)))
		check(f.Close())
		fmt.Println(p)
	}
	if c.Unported != "" {
		fmt.Println("note: the port reached unported", c.Unported)
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "screenshot:", err)
		os.Exit(1)
	}
}
