// Command play runs the port in a window with sound.
//
//	go run ./cmd/play                      # play
//	go run ./cmd/play -record /tmp/me.keys # also record the keys
//
// Controls: on the title press 1 for joystick (a gamepad) or 2 for
// keyboard. Keyboard: cursor keys (up accelerates, down brakes,
// left/right steer), space jumps. Esc quits.
//
// A recorded schedule replays into the original and the port:
//
//	scripts/refdump.sh /tmp/me.trace N /tmp/me.keys
//	go run ./cmd/refcompare -trace /tmp/me.trace -keys /tmp/me.keys
package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"

	"go-msx-penguin/internal/game"
	"go-msx-penguin/internal/msxrender"
	"go-msx-penguin/internal/psg"
)

const sampleRate = 44100

// keyMap maps host keys to MSX keyboard matrix bits (row, mask).
var keyMap = []struct {
	key  ebiten.Key
	row  int
	mask byte
}{
	{ebiten.KeySpace, 8, 0x01},
	{ebiten.KeyArrowLeft, 8, 0x10},
	{ebiten.KeyArrowUp, 8, 0x20},
	{ebiten.KeyArrowDown, 8, 0x40},
	{ebiten.KeyArrowRight, 8, 0x80},
	{ebiten.KeyDigit1, 0, 0x02},
	{ebiten.KeyDigit2, 0, 0x04},
}

type app struct {
	c      *game.Console
	screen *image.RGBA
	snd    *sound
	rec    *bufio.Writer
	recF   *os.File
}

func main() {
	record := flag.String("record", "", "write the key schedule to this file")
	scale := flag.Int("scale", 3, "window scale")
	flag.Parse()

	a := &app{c: game.New(), screen: image.NewRGBA(image.Rect(0, 0, msxrender.Width, msxrender.Height))}
	a.c.Boot(2)
	if *record != "" {
		f, err := os.Create(*record)
		if err != nil {
			log.Fatal(err)
		}
		a.recF, a.rec = f, bufio.NewWriter(f)
		fmt.Fprintln(a.rec, "# recorded by cmd/play")
		a.rec.Flush()
	}
	a.snd = newSound()

	ebiten.SetWindowTitle("go-msx-penguin")
	ebiten.SetWindowSize(msxrender.Width**scale, msxrender.Height**scale)
	ebiten.SetTPS(60)
	err := ebiten.RunGame(a)
	if a.rec != nil {
		a.rec.Flush()
		a.recF.Close()
	}
	if err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}

func (a *app) Update() error {
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	a.input()
	a.c.Frame()
	a.snd.frame(a.c.PSG)
	return nil
}

// input updates the keyboard matrix and joystick port, recording changes
// against the index of the next tick to start (as refdump counts ticks).
func (a *app) input() {
	for _, k := range keyMap {
		down := ebiten.IsKeyPressed(k.key)
		was := a.c.Keys[k.row]&k.mask == 0
		if down == was {
			continue
		}
		if down {
			a.c.Keys[k.row] &^= k.mask
		} else {
			a.c.Keys[k.row] |= k.mask
		}
		if a.rec != nil {
			dir := "up"
			if down {
				dir = "down"
			}
			fmt.Fprintf(a.rec, "%d:%d:0x%02X:%s\n", a.c.Ticks(), k.row, k.mask, dir)
			a.rec.Flush() // survive the process being killed
		}
	}
	a.c.JoyA = joystick()
}

// joystick reads the first gamepad as an MSX joystick (clear = pressed):
// bits 0-3 up, down, left, right; 4-5 triggers A and B.
func joystick() byte {
	v := byte(0x3F)
	ids := ebiten.AppendGamepadIDs(nil)
	if len(ids) == 0 {
		return v
	}
	id := ids[0]
	press := func(bit byte) { v &^= bit }
	if ebiten.IsStandardGamepadLayoutAvailable(id) {
		b := func(sb ebiten.StandardGamepadButton) bool { return ebiten.IsStandardGamepadButtonPressed(id, sb) }
		ax := func(sa ebiten.StandardGamepadAxis) float64 { return ebiten.StandardGamepadAxisValue(id, sa) }
		if b(ebiten.StandardGamepadButtonLeftTop) || ax(ebiten.StandardGamepadAxisLeftStickVertical) < -0.5 {
			press(0x01)
		}
		if b(ebiten.StandardGamepadButtonLeftBottom) || ax(ebiten.StandardGamepadAxisLeftStickVertical) > 0.5 {
			press(0x02)
		}
		if b(ebiten.StandardGamepadButtonLeftLeft) || ax(ebiten.StandardGamepadAxisLeftStickHorizontal) < -0.5 {
			press(0x04)
		}
		if b(ebiten.StandardGamepadButtonLeftRight) || ax(ebiten.StandardGamepadAxisLeftStickHorizontal) > 0.5 {
			press(0x08)
		}
		if b(ebiten.StandardGamepadButtonRightBottom) {
			press(0x10)
		}
		if b(ebiten.StandardGamepadButtonRightRight) {
			press(0x20)
		}
	}
	return v
}

func (a *app) Draw(screen *ebiten.Image) {
	msxrender.RenderInto(a.screen, a.c.VRAM[:], a.c.Regs)
	screen.WritePixels(a.screen.Pix)
}

func (a *app) Layout(int, int) (int, int) { return msxrender.Width, msxrender.Height }

// sound turns the game's PSG registers into audio in step with game
// frames. Each frame the game thread applies that frame's registers and
// generates exactly one frame of samples (44100/60 = 735) into a queue
// the audio thread drains. (Applying registers from the audio thread
// instead, once per Read, sampled them only a few times a second, which
// garbled the music and lost short sound effects.)
type sound struct {
	mu    sync.Mutex
	cond  *sync.Cond
	chip  *psg.Chip
	queue []byte // 16-bit little-endian stereo
	pcm   []int16
}

const (
	frameSamples = sampleRate / 60
	frameBytes   = frameSamples * 4
	maxQueued    = 6 * frameBytes // drop the oldest beyond this (latency cap)
)

func newSound() *sound {
	s := &sound{chip: psg.New(sampleRate)}
	s.cond = sync.NewCond(&s.mu)
	ctx := audio.NewContext(sampleRate)
	p, err := ctx.NewPlayer(s)
	if err != nil {
		log.Fatal(err)
	}
	p.SetBufferSize(50 * time.Millisecond)
	p.Play()
	return s
}

// frame generates one frame of audio from the registers the game left.
func (s *sound) frame(regs [16]byte) {
	s.pcm = s.chip.Frame(regs, frameSamples, 12000, s.pcm[:0])
	buf := make([]byte, 0, frameBytes)
	for _, v := range s.pcm {
		buf = append(buf, byte(v), byte(v>>8), byte(v), byte(v>>8)) // stereo
	}
	s.mu.Lock()
	s.queue = append(s.queue, buf...)
	if over := len(s.queue) - maxQueued; over > 0 {
		s.queue = s.queue[over/4*4:]
	}
	s.mu.Unlock()
	s.cond.Signal()
}

// Read hands queued samples to the audio device, waiting briefly for the
// game when the queue is empty and playing silence if it stalls (a paused
// or minimised window).
func (s *sound) Read(p []byte) (int, error) {
	n := len(p) / 4 * 4
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == 0 {
		deadline := time.Now().Add(30 * time.Millisecond)
		go func() {
			time.Sleep(30 * time.Millisecond)
			s.cond.Broadcast()
		}()
		for len(s.queue) == 0 && time.Now().Before(deadline) {
			s.cond.Wait()
		}
	}
	if len(s.queue) == 0 {
		for i := 0; i < n; i++ {
			p[i] = 0
		}
		return n, nil
	}
	if n > len(s.queue) {
		n = len(s.queue)
	}
	copy(p, s.queue[:n])
	s.queue = s.queue[n:]
	return n, nil
}
