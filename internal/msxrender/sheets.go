package msxrender

import "image"

// TileSheet draws all 768 Screen 2 pattern+colour pairs as three 16x16
// grids of tiles (one per screen third), side by side, using the same
// table addressing as the screen renderer.
func TileSheet(vram []byte, regs [8]byte) *image.RGBA {
	const cell = 8
	img := image.NewRGBA(image.Rect(0, 0, 3*16*cell, 16*cell))
	idx := make([]byte, img.Rect.Dx()*img.Rect.Dy())
	pgBase := int(regs[4]&0x04) << 11
	pgMask := int(regs[4]&0x03)<<11 | 0x7FF
	ctBase := int(regs[3]&0x80) << 6
	ctMask := int(regs[3]&0x7F)<<6 | 0x3F
	w := img.Rect.Dx()
	for third := 0; third < 3; third++ {
		for n := 0; n < 256; n++ {
			x0 := third*16*cell + n%16*cell
			y0 := n / 16 * cell
			for line := 0; line < 8; line++ {
				i := third<<11 | n<<3 | line
				pat, clr := vram[pgBase|i&pgMask], vram[ctBase|i&ctMask]
				fg, bg := clr>>4, clr&0x0F
				for b := 0; b < 8; b++ {
					c := bg
					if pat&(0x80>>b) != 0 {
						c = fg
					}
					idx[(y0+line)*w+x0+b] = c
				}
			}
		}
	}
	paint(img, idx, regs[7]&0x0F)
	return img
}

// SpriteSheet draws the sprite pattern table as 64 16x16 sprites (8x8
// grid, quadrants TL, BL, TR, BR) in colour c.
func SpriteSheet(vram []byte, regs [8]byte, c byte) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 8*17, 8*17))
	w := img.Rect.Dx()
	idx := make([]byte, w*img.Rect.Dy())
	spBase := int(regs[6]&0x07) << 11
	for s := 0; s < 64; s++ {
		x0, y0 := s%8*17, s/8*17
		for row := 0; row < 16; row++ {
			for col := 0; col < 16; col++ {
				bits := vram[spBase+s*32+(col/8)*16+row]
				if bits&(0x80>>(col%8)) != 0 {
					idx[(y0+row)*w+x0+col] = c
				}
			}
		}
	}
	paint(img, idx, 1)
	return img
}

func paint(img *image.RGBA, idx []byte, back byte) {
	for i, c := range idx {
		if c == 0 {
			c = back
		}
		p := Palette[c]
		o := i * 4
		img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = p.R, p.G, p.B, 255
	}
}
