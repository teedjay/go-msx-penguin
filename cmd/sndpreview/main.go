// Command sndpreview renders a sound id to a WAV file through the ported
// sound driver and the PSG model, so sounds can be checked without a
// window. Output is ROM-derived: write it outside the repo.
//
//	go run ./cmd/sndpreview -id 0x92 -o /tmp/snd92.wav
//	go run ./cmd/sndpreview -all -o /tmp/sounds   # every id that plays
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"go-msx-penguin/internal/game"
	"go-msx-penguin/internal/psg"
)

const rate = 44100

func main() {
	idStr := flag.String("id", "0x92", "sound id")
	all := flag.Bool("all", false, "render every id 0x01-0x9F that produces sound into the -o directory")
	out := flag.String("o", "/tmp/snd.wav", "output WAV (or directory with -all)")
	maxSec := flag.Float64("max", 30, "longest render in seconds (looping sounds)")
	flag.Parse()

	if *all {
		check(os.MkdirAll(*out, 0o755))
		for id := 1; id < 0xA0; id++ {
			if !(id < 0x40 || id >= 0x80) {
				continue
			}
			samples, ok := render(byte(id), *maxSec)
			if !ok {
				continue
			}
			p := filepath.Join(*out, fmt.Sprintf("snd%02X.wav", id))
			writeWAV(p, samples)
			fmt.Printf("%s  %.2fs\n", p, float64(len(samples))/rate)
		}
		return
	}
	id, err := strconv.ParseUint(*idStr, 0, 8)
	check(err)
	samples, _ := render(byte(id), *maxSec)
	writeWAV(*out, samples)
	fmt.Printf("%s  %.2fs\n", *out, float64(len(samples))/rate)
}

// render plays one sound on a fresh console, one driver step per 60th of
// a second, until it stops or maxSec passes.
func render(id byte, maxSec float64) (s []int16, ok bool) {
	defer func() {
		if recover() != nil { // an id past the end of the sound table
			s, ok = nil, false
		}
	}()
	c := game.New()
	c.RAM[0x000] = 1 // any non-zero state: the driver runs
	c.StartSound(id)
	if !c.SoundActive() {
		return nil, false
	}
	chip := psg.New(rate)
	for frame := 0; float64(frame) < maxSec*60 && c.SoundActive(); frame++ {
		c.SoundDriver()
		chip.SetRegs(c.PSG)
		for i := 0; i < rate/60; i++ {
			s = append(s, int16(chip.Sample()*20000))
		}
	}
	return s, true
}

func writeWAV(path string, s []int16) {
	f, err := os.Create(path)
	check(err)
	defer f.Close()
	n := uint32(len(s) * 2)
	w := func(v any) { check(binary.Write(f, binary.LittleEndian, v)) }
	f.WriteString("RIFF")
	w(36 + n)
	f.WriteString("WAVEfmt ")
	w(uint32(16))
	w(uint16(1))
	w(uint16(1))
	w(uint32(rate))
	w(uint32(rate * 2))
	w(uint16(2))
	w(uint16(16))
	f.WriteString("data")
	w(n)
	w(s)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "sndpreview:", err)
		os.Exit(1)
	}
}
