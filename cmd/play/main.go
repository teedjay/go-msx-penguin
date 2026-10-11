// Command play runs the port in a window with sound.
//
//	go run ./cmd/play                      # play
//	go run ./cmd/play -record /tmp/me.keys # also record the keys
//
// Controls: on the title press 1 for joystick (a gamepad) or 2 for
// keyboard. Keyboard: cursor keys (up accelerates, down brakes,
// left/right steer), space jumps. Esc quits.
//
// Level-select cheat: on the title screen press a letter, A for level 1
// (normal) to Z for level 26, before starting the game. The choice shows
// in the top right corner, drawn by this frontend over the picture (not
// through the MSX video chip). A cheated game cannot be replayed into the
// original from a -record file.
//
// Extra-time cheat: press T during a game to add 100 to the time.
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
	"image/png"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

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
	frame  *ebiten.Image // the MSX picture, scaled into the window
	cheat  cheat

	frames    int
	shot      string
	shotFrame int
	shotDone  bool
	script    map[ebiten.Key][]int // scripted press start frames
	snd       *sound
	rec       *bufio.Writer
	recF      *os.File
}

func main() {
	record := flag.String("record", "", "write the key schedule to this file")
	scale := flag.Int("scale", 3, "window scale")
	level := flag.Int("level", 0, "preselect the level-select cheat (1-26), as if its letter were pressed on the title")
	shot := flag.String("shot", "", "save the window image (overlay included) to this PNG at -shotframe and quit")
	shotFrame := flag.Int("shotframe", 300, "frame for -shot")
	press := flag.String("press", "", "scripted key presses for checks, frame:KEY,... (each held 3 frames), e.g. 260:2,1200:T")
	flag.Parse()

	a := &app{
		c:      game.New(),
		screen: image.NewRGBA(image.Rect(0, 0, msxrender.Width, msxrender.Height)),
		frame:  ebiten.NewImage(msxrender.Width, msxrender.Height),
	}
	a.c.Boot(2)
	if *level > 0 {
		a.cheat = cheat{level: *level, started: -1}
		a.c.StartStage = *level
	}
	a.shot, a.shotFrame = *shot, *shotFrame
	a.script = parsePress(*press)
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
	if a.shotDone {
		return ebiten.Termination
	}
	a.frames++
	a.input()
	a.cheat.update(a)
	a.c.Frame()
	a.snd.frame(a.c.PSG)
	return nil
}

// input updates the keyboard matrix and joystick port, recording changes
// against the index of the next tick to start (as refdump counts ticks).
func (a *app) input() {
	for _, k := range keyMap {
		down := a.pressed(k.key)
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

// pressed reports whether a key is held, really or by -press.
func (a *app) pressed(k ebiten.Key) bool {
	if ebiten.IsKeyPressed(k) {
		return true
	}
	for _, f := range a.script[k] {
		if a.frames >= f && a.frames < f+3 {
			return true
		}
	}
	return false
}

// justPressed reports whether a key went down this frame.
func (a *app) justPressed(k ebiten.Key) bool {
	if inpututil.IsKeyJustPressed(k) {
		return true
	}
	for _, f := range a.script[k] {
		if a.frames == f {
			return true
		}
	}
	return false
}

// parsePress reads -press: frame:KEY pairs, KEY a letter, digit or a
// name ebiten knows (Space, ArrowUp, ...).
func parsePress(s string) map[ebiten.Key][]int {
	m := map[ebiten.Key][]int{}
	for _, item := range strings.Split(s, ",") {
		fs, ks, ok := strings.Cut(strings.TrimSpace(item), ":")
		if !ok {
			continue
		}
		f, err := strconv.Atoi(fs)
		if err != nil {
			log.Fatalf("-press: bad frame in %q", item)
		}
		var k ebiten.Key
		if len(ks) == 1 && ks[0] >= '0' && ks[0] <= '9' {
			ks = "Digit" + ks
		}
		if err := k.UnmarshalText([]byte(ks)); err != nil {
			log.Fatalf("-press: bad key in %q", item)
		}
		m[k] = append(m[k], f)
	}
	return m
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
	a.frame.WritePixels(a.screen.Pix)
	// Scale the MSX picture to fit the window, keeping its aspect.
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	k := math.Min(float64(sw)/msxrender.Width, float64(sh)/msxrender.Height)
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(k, k)
	op.GeoM.Translate((float64(sw)-k*msxrender.Width)/2, (float64(sh)-k*msxrender.Height)/2)
	screen.DrawImage(a.frame, &op)
	a.cheat.draw(screen, k)
	if a.shot != "" && a.frames >= a.shotFrame && !a.shotDone {
		a.saveShot(screen)
	}
}

// saveShot writes the composited window image to the -shot file.
func (a *app) saveShot(screen *ebiten.Image) {
	b := screen.Bounds()
	img := image.NewRGBA(b)
	screen.ReadPixels(img.Pix)
	f, err := os.Create(a.shot)
	if err == nil {
		err = png.Encode(f, img)
		f.Close()
	}
	if err != nil {
		log.Print(err)
	}
	a.shotDone = true
}

// Layout uses the window's own pixels, so the overlay is drawn at full
// resolution over the scaled picture.
func (a *app) Layout(w, h int) (int, int) { return w, h }

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

	player *oto.Player
}

const (
	frameSamples = sampleRate / 60
	frameBytes   = frameSamples * 4
	maxQueued    = 2 * frameBytes // at most this queued (latency): drop the oldest beyond it
)

// Audio latency: the device keeps devBuffer of audio (split into four
// buffers by oto), the player playerBuffer, and the queue below at most
// maxQueued. Ebitengine's audio package fixes the device part at about
// 100 ms, which made sound effects audibly late, so this drives oto (the
// library under it) directly. AUDIO_DEV / AUDIO_PLAYER override the two
// buffer sizes (Go durations) for machines that crackle.
var (
	devBuffer    = 40 * time.Millisecond
	playerBuffer = 20 * time.Millisecond
)

func newSound() *sound {
	s := &sound{chip: psg.New(sampleRate)}
	s.cond = sync.NewCond(&s.mu)
	if d, err := time.ParseDuration(os.Getenv("AUDIO_DEV")); err == nil {
		devBuffer = d
	}
	if d, err := time.ParseDuration(os.Getenv("AUDIO_PLAYER")); err == nil {
		playerBuffer = d
	}
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
		BufferSize:   devBuffer,
	})
	if err != nil {
		log.Fatal(err)
	}
	<-ready
	p := ctx.NewPlayer(s)
	p.SetBufferSize(int(playerBuffer * sampleRate * 4 / time.Second))
	p.Play()
	s.player = p
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
