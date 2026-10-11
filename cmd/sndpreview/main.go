// Command sndpreview renders a sound id to a WAV file through the ported
// sound driver and the PSG model, so sounds can be checked without a
// window. Output is ROM-derived: write it outside the repo.
//
//	go run ./cmd/sndpreview -id 0x92 -o /tmp/snd92.wav
//	go run ./cmd/sndpreview -all -o /tmp/sounds   # every id that plays
//	go run ./cmd/sndpreview -trace /tmp/attract.trace -irqs 1500 -o /tmp/port.wav
//
// The -trace mode renders the game's audio over a reference trace's
// interrupts, placed where the original took them, one MSX frame of
// samples per interrupt from the first interrupt on, for comparison with
// openMSX's own recording of the same run (record start -audioonly).
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go-msx-penguin/internal/game"
	"go-msx-penguin/internal/psg"
	"go-msx-penguin/internal/reftrace"
)

const (
	rate = 44100
	gain = 20000
)

func main() {
	idStr := flag.String("id", "0x92", "sound id")
	all := flag.Bool("all", false, "render every id 0x01-0x9F that produces sound into the -o directory")
	out := flag.String("o", "/tmp/snd.wav", "output WAV (or directory with -all)")
	maxSec := flag.Float64("max", 30, "longest render in seconds (looping sounds)")
	tracePath := flag.String("trace", "", "render the game's audio over this reference trace")
	keysPath := flag.String("keys", "", "key schedule for -trace")
	irqs := flag.Int("irqs", 1500, "interrupts to render with -trace")
	flag.Parse()

	if *tracePath != "" {
		writeWAV(*out, renderTrace(*tracePath, *keysPath, *irqs))
		fmt.Println(*out)
		return
	}

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
		s = chip.Frame(c.PSG, rate/60, gain, s)
	}
	return s, true
}

// renderTrace replays a trace's interrupt schedule on the port (as
// cmd/refcompare does) and renders audio per interrupt. An MSX frame is
// 228*262 cycles of 3579545 Hz.
func renderTrace(path, keysPath string, irqs int) []int16 {
	r, err := reftrace.Open(path)
	check(err)
	defer r.Close()
	keys := map[int][][3]int{}
	if keysPath != "" {
		b, err := os.ReadFile(keysPath)
		check(err)
		for _, l := range strings.Split(string(b), "\n") {
			p := strings.Split(strings.TrimSpace(strings.SplitN(l, "#", 2)[0]), ":")
			if len(p) != 4 {
				continue
			}
			t, _ := strconv.Atoi(p[0])
			row, _ := strconv.Atoi(p[1])
			m, _ := strconv.ParseUint(p[2], 0, 8)
			dn := 0
			if p[3] == "down" {
				dn = 1
			}
			keys[t] = append(keys[t], [3]int{row, int(m), dn})
		}
	}
	const perFrame = rate * 228 * 262 / 3579545.0
	chip := psg.New(rate)
	var s []int16
	acc, count := 0.0, 0
	c := game.New()
	c.OnInterrupt = func() {
		count++
		if count > irqs {
			return
		}
		acc += perFrame
		n := int(acc)
		acc -= float64(n)
		s = chip.Frame(c.PSG, n, gain, s)
	}
	c.Boot(int(r.InitIRQs))
	prev := r.InitIRQs
	var rec reftrace.Record
	for n := 0; count < irqs; n++ {
		if err := r.Next(&rec); err != nil {
			break
		}
		d := int(rec.IRQs - prev)
		prev = rec.IRQs
		if n > 0 {
			check(c.TickNested(rec.Offsets[:d-1]))
		}
		c.EndTick()
		c.PerInterrupt()
		for _, k := range keys[n] {
			if k[2] == 1 {
				c.Keys[k[0]] &^= byte(k[1])
			} else {
				c.Keys[k[0]] |= byte(k[1])
			}
		}
	}
	return s
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
