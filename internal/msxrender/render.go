// Package msxrender composites TMS9918A VRAM into an image. It is the one
// renderer: the game draws every frame through it, and the headless tools
// (screenshot, vramview, refcompare) use it too, so a still can never show
// something the running game would not.
package msxrender

import (
	"image"
	"image/color"
)

const (
	Width  = 256
	Height = 192
)

// Palette is the TMS9918A palette (index 0 is transparent and shows the
// backdrop).
var Palette = [16]color.RGBA{
	{0, 0, 0, 255}, {0, 0, 0, 255}, {33, 200, 66, 255}, {94, 220, 120, 255},
	{84, 85, 237, 255}, {125, 118, 252, 255}, {212, 82, 77, 255}, {66, 235, 245, 255},
	{252, 85, 84, 255}, {255, 121, 120, 255}, {212, 193, 84, 255}, {230, 206, 128, 255},
	{33, 176, 59, 255}, {201, 91, 186, 255}, {204, 204, 204, 255}, {255, 255, 255, 255},
}

// Render draws the screen described by vram and the VDP registers.
func Render(vram []byte, regs [8]byte) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	RenderInto(img, vram, regs)
	return img
}

// RenderInto draws into an existing 256x192 image.
func RenderInto(img *image.RGBA, vram []byte, regs [8]byte) {
	back := regs[7] & 0x0F
	idx := make([]byte, Width*Height) // colour index per pixel, 0 = transparent
	if regs[1]&0x40 != 0 {            // display enabled
		m3 := regs[0]&0x02 != 0
		m1 := regs[1]&0x10 != 0
		m2 := regs[1]&0x08 != 0
		switch {
		case m3 && !m1 && !m2:
			graphics2(idx, vram, regs)
		case !m1 && !m2 && !m3:
			graphics1(idx, vram, regs)
		}
		if !m1 { // no sprites in text mode
			sprites(idx, vram, regs)
		}
	}
	for i, c := range idx {
		if c == 0 {
			c = back
		}
		p := Palette[c]
		o := i * 4
		img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = p.R, p.G, p.B, 255
	}
}

func graphics2(idx, vram []byte, regs [8]byte) {
	nameBase := int(regs[2]&0x0F) << 10
	pgBase := int(regs[4]&0x04) << 11
	pgMask := int(regs[4]&0x03)<<11 | 0x7FF
	ctBase := int(regs[3]&0x80) << 6
	ctMask := int(regs[3]&0x7F)<<6 | 0x3F
	for row := 0; row < 24; row++ {
		third := row / 8
		for col := 0; col < 32; col++ {
			n := int(vram[nameBase+row*32+col])
			for line := 0; line < 8; line++ {
				i := third<<11 | n<<3 | line
				pat := vram[pgBase|i&pgMask]
				clr := vram[ctBase|i&ctMask]
				plot8(idx, col*8, row*8+line, pat, clr)
			}
		}
	}
}

func graphics1(idx, vram []byte, regs [8]byte) {
	nameBase := int(regs[2]&0x0F) << 10
	pgBase := int(regs[4]&0x07) << 11
	ctBase := int(regs[3]) << 6
	for row := 0; row < 24; row++ {
		for col := 0; col < 32; col++ {
			n := int(vram[nameBase+row*32+col])
			clr := vram[ctBase+n>>3]
			for line := 0; line < 8; line++ {
				plot8(idx, col*8, row*8+line, vram[pgBase+n*8+line], clr)
			}
		}
	}
}

func plot8(idx []byte, x, y int, pat, clr byte) {
	fg, bg := clr>>4, clr&0x0F
	o := y*Width + x
	for b := 0; b < 8; b++ {
		if pat&(0x80>>b) != 0 {
			idx[o+b] = fg
		} else {
			idx[o+b] = bg
		}
	}
}

// sprites draws the sprite plane. Y=$D0 ends the list; the displayed row
// is Y+1; at most four sprites show on a line (lower numbers win, and
// transparent sprites still count); colour bit 7 (early clock) shifts the
// sprite 32 pixels left. A 16x16 sprite ignores the low 2 bits of its
// pattern number, and its quadrants are TL, BL, TR, BR.
func sprites(idx, vram []byte, regs [8]byte) {
	satBase := int(regs[5]&0x7F) << 7
	spBase := int(regs[6]&0x07) << 11
	big := regs[1]&0x02 != 0
	scale := 1
	if regs[1]&0x01 != 0 {
		scale = 2
	}
	size := 8
	if big {
		size = 16
	}
	var perLine [Height]int
	var drawn [Width * Height]bool
	for s := 0; s < 32; s++ {
		a := satBase + s*4
		y := int(vram[a])
		if y == 0xD0 {
			break
		}
		x := int(vram[a+1])
		pat := int(vram[a+2])
		clr := vram[a+3]
		if big {
			pat &^= 3
		}
		if clr&0x80 != 0 {
			x -= 32
		}
		top := y + 1
		if y >= 0xE0 { // wraps: partially visible at the top
			top -= 256
		}
		c := clr & 0x0F
		for py := 0; py < size*scale; py++ {
			sy := top + py
			if sy < 0 || sy >= Height {
				continue
			}
			perLine[sy]++
			if perLine[sy] > 4 || c == 0 {
				continue
			}
			row := py / scale
			for px := 0; px < size*scale; px++ {
				sx := x + px
				if sx < 0 || sx >= Width {
					continue
				}
				col := px / scale
				q := (col/8)*16 + row // quadrant order TL, BL, TR, BR
				bits := vram[spBase+pat*8+q]
				if bits&(0x80>>(col%8)) == 0 {
					continue
				}
				o := sy*Width + sx
				if !drawn[o] { // lower sprite numbers are in front
					drawn[o] = true
					idx[o] = c
				}
			}
		}
	}
}
