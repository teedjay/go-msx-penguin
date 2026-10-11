package main

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"

	"go-msx-penguin/internal/game"
)

// cheat is the cheats' frontend half: letter keys on the title choose
// the starting level (game.Console.StartStage does the rest), T in a game
// adds 100 to the time (game.Console.AddTime), and an overlay in the top
// right corner shows what was chosen or done.
type cheat struct {
	level   int // 0: none chosen; 1 (A) is the normal start
	started int // frames since the cheated game started, -1 before

	note       string // a short in-game cheat message (T: extra time)
	noteFrames int
}

// showFrames is how long the overlay stays after the game starts.
const showFrames = 180

var face = text.NewGoXFace(basicfont.Face7x13)

func (ch *cheat) update(a *app) {
	c := a.c
	inGame := c.RAM[0x002]&0x40 != 0 // live play (keyboard or joystick)
	if ch.noteFrames > 0 {
		ch.noteFrames--
	}
	// T in a game: 100 more time.
	if inGame && a.justPressed(ebiten.KeyT) && c.AddTime() {
		ch.note, ch.noteFrames = "CHEAT: TIME +100", 120
		if a.rec != nil {
			fmt.Fprintf(a.rec, "# cheat: time +100 before tick %d (not replayable in the original)\n", c.Ticks())
			a.rec.Flush()
		}
	}
	switch {
	case ch.level != 0 && ch.started >= 0:
		ch.started++
		if ch.started > showFrames || !inGame {
			ch.level, ch.started = 0, -1
		}
		return
	case ch.level != 0 && inGame:
		ch.started = 0 // the game has just started with this choice
		if a.rec != nil && ch.level > 1 {
			fmt.Fprintf(a.rec, "# cheat: started at level %d (not replayable in the original)\n", ch.level)
			a.rec.Flush()
		}
		return
	}
	// The select screen: the title with PLAY SELECT (states 4-6) or the
	// demo that follows it, before a game starts.
	if inGame || c.RAM[0x000] < 4 {
		return
	}
	for k := ebiten.KeyA; k <= ebiten.KeyZ; k++ {
		if a.justPressed(k) {
			ch.level = int(k-ebiten.KeyA) + 1
			ch.started = -1
			c.StartStage = ch.level
		}
	}
}

func (ch *cheat) draw(screen *ebiten.Image, scale float64) {
	var msg string
	switch {
	case ch.noteFrames > 0:
		msg = ch.note
	case ch.level == 1:
		msg = "LEVEL 1 (NORMAL) - " + game.Destination(1)
	case ch.level > 1:
		msg = fmt.Sprintf("CHEAT: START AT LEVEL %d - %s", ch.level, game.Destination(ch.level))
	default:
		return
	}
	size := textScale(scale)
	w, h := text.Measure(msg, face, 0)
	w, h = w*size, h*size
	pad := 4 * size
	x := float64(screen.Bounds().Dx()) - w - 2*pad - 4
	vector.DrawFilledRect(screen, float32(x), 4, float32(w+2*pad), float32(h+2*pad), color.RGBA{0, 0, 0, 240}, false)
	var op text.DrawOptions
	op.GeoM.Scale(size, size)
	op.GeoM.Translate(x+pad, 4+pad)
	op.ColorScale.ScaleWithColor(color.RGBA{255, 230, 80, 255})
	text.Draw(screen, msg, face, &op)
}

// textScale picks a text scale that stays readable as the window grows.
func textScale(scale float64) float64 {
	if scale < 2.5 {
		return 1
	}
	return 2
}
