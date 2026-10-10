// Command vramview renders VRAM from a reference trace through the shared
// renderer: the composed screen, the Screen 2 tile sheet, or the sprite
// pattern sheet. Output is ROM-derived: write it outside the repo.
//
//	go run ./cmd/vramview -trace /tmp/attract.trace -tick 200 -o /tmp/t200.png
//	go run ./cmd/vramview -trace /tmp/attract.trace -tick 200 -mode tiles -o /tmp/tiles.png
//	go run ./cmd/vramview -trace /tmp/attract.trace -every 50 -o /tmp/shots
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"go-msx-penguin/internal/msxrender"
	"go-msx-penguin/internal/reftrace"
)

func main() {
	tracePath := flag.String("trace", "", "reference trace")
	tick := flag.Int("tick", 0, "record to render")
	every := flag.Int("every", 0, "render every Nth record into the -o directory")
	mode := flag.String("mode", "screen", "screen | tiles | sprites")
	out := flag.String("o", "/tmp/vram.png", "output PNG (or directory with -every)")
	flag.Parse()

	r, err := reftrace.Open(*tracePath)
	check(err)
	defer r.Close()
	var rec reftrace.Record

	if *every > 0 {
		check(os.MkdirAll(*out, 0o755))
		for n := 0; ; n++ {
			if err := r.Next(&rec); err == io.EOF {
				return
			} else {
				check(err)
			}
			if n%*every == 0 {
				p := filepath.Join(*out, fmt.Sprintf("tick%05d.png", n))
				write(p, render(*mode, &rec))
				fmt.Printf("%s  state=$%02X irqs=%d\n", p, rec.RAM[0], rec.IRQs)
			}
		}
	}
	check(r.ReadAt(*tick, &rec))
	write(*out, render(*mode, &rec))
	fmt.Printf("tick %d: irqs=%d regs=% X state=$%02X\n", rec.Tick, rec.IRQs, rec.Regs, rec.RAM[0])
}

func render(mode string, rec *reftrace.Record) image.Image {
	switch mode {
	case "tiles":
		return msxrender.TileSheet(rec.VRAM[:], rec.Regs)
	case "sprites":
		return msxrender.SpriteSheet(rec.VRAM[:], rec.Regs, 15)
	}
	return msxrender.Render(rec.VRAM[:], rec.Regs)
}

func write(path string, img image.Image) {
	f, err := os.Create(path)
	check(err)
	check(png.Encode(f, img))
	check(f.Close())
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "vramview:", err)
		os.Exit(1)
	}
}
