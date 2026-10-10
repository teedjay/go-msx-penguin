package game

import "go-msx-penguin/internal/rle"

// States 1-6: the logo and title sequence.

// state1 loads the common graphics. Ported from $4135-$4145 (High).
func (c *Console) state1() {
	c.loadCommon()
	c.RAM[0x00A] = 0x11
	c.wr16(0xE00E, 0)
	c.nextState43EE()
}

// state2 scrolls the logo up one row every other tick, then prints the
// line under it. Ported from $4146-$4157 (High).
func (c *Console) state2() {
	if c.RAM[0x003]&1 == 0 {
		return
	}
	if !c.logoStep() {
		return
	}
	c.printText(0x57DE)
	c.nextState43E9()
}

// state3 waits $E004 ticks, then redraws the screen for the title.
// Ported from $4158-$4162 (High).
func (c *Console) state3() {
	c.RAM[0x004]--
	if c.RAM[0x004] != 0 {
		return
	}
	c.title481E()
	// JP $43EB with A from $481E, which ends in the $44DC fill loop
	// and so returns A = 0.
	c.nextState43EB(0)
}

// state4 draws the title columns, then prints the play-select text.
// Ported from $4163-$4170 (High).
func (c *Console) state4() {
	if c.title4839() {
		return
	}
	c.printText(0x5793)
	c.nextState43EB(0)
}

// state5 waits 256 ticks ($E004 starts at 0). Ported from $4171-$4178.
func (c *Console) state5() {
	c.RAM[0x004]--
	if c.RAM[0x004] != 0 {
		return
	}
	c.nextState43E9()
}

// state6 wipes the screen column by column. Ported from $4179-$417F.
func (c *Console) state6() {
	if !c.wipe45A6() {
		return
	}
	c.nextState43EE()
}

// loadCommon loads the common colours and patterns into all three
// Screen 2 thirds. Ported from $5834-$5845 (High: confirmed against the
// original's VRAM).
func (c *Console) loadCommon() {
	for _, de := range []uint16{0x0000, 0x0800, 0x1000} {
		c.loadThird(de)
	}
}

// loadThird loads one third. Ported from $5846-$5884 (High).
func (c *Console) loadThird(de uint16) {
	start := de
	// 16 solid colours: tile n has colour byte n on all 8 rows.
	for a := byte(0); a < 16; a++ {
		for i := 0; i < 8; i++ {
			c.vwrbyte(de, a)
			de++
		}
	}
	c.vfill(de, 0x270, 0xF0)
	// $454C continues at the current VRAM address in the caller's mode;
	// C = 0 (normal) because the fill loop leaves BC = 0, and the PUSH/POP
	// BC around the loop keeps it.
	c.rleCont(0x5D6A, false)
	for i := 0; i < 0x16; i++ {
		c.rleCont(0x5DA0, false)
	}
	c.vsetwr(0x6000 + start) // $4543: patterns, normal mode
	c.rleCont(0x5885, false)
	hl := c.rleCont(0x5C15, false)
	c.rleCont(hl, false)
}

// rleCont decodes the stream at hl to the current VRAM address and
// returns the address after its terminator. Ported from $454C-$4582
// (High); the format is documented in internal/rle.
func (c *Console) rleCont(hl uint16, mirror bool) uint16 {
	out, end, err := rle.Decode(c.rd, hl, mirror)
	if err != nil {
		panic(err)
	}
	for _, b := range out {
		c.vout(b)
	}
	return end
}

// rleAt sets the VRAM address from DE and decodes. Ported from $4543
// (normal) and $4547 (mirrored).
func (c *Console) rleAt(hl, de uint16, mirror bool) uint16 {
	c.vsetwr(de)
	return c.rleCont(hl, mirror)
}

// rleDest reads the VRAM destination from the stream, then decodes in
// normal mode. Ported from $453F-$4542.
func (c *Console) rleDest(hl uint16) uint16 {
	de := c.rd16(hl)
	return c.rleAt(hl+2, de, false)
}

// printText writes text at hl: a VRAM address word, then bytes until $FF;
// $FE starts a new address word. Returns the address after the $FF.
// Ported from $4583-$4594 (High).
func (c *Console) printText(hl uint16) uint16 {
	de := c.rd16(hl)
	return c.printAt(hl+2, de)
}

// printAt is printText with the first VRAM address given. Ported from
// $4587-$4594.
func (c *Console) printAt(hl, de uint16) uint16 {
	for {
		a := c.rd(hl)
		hl++
		switch a {
		case 0xFF:
			return hl
		case 0xFE:
			de = c.rd16(hl)
			hl += 2
			continue
		}
		c.vwrbyte(de, a)
		de++
	}
}

// logoStep moves the logo up one row and returns true (Z) when $E00A
// reaches 0. Ported from $486B-$48A4 (High).
func (c *Console) logoStep() bool {
	e00e := c.rd16(0xE00E) + 0x20
	c.wr16(0xE00E, e00e)
	de := 0x3AAA - e00e
	// Three rows of tiles from $44: 3, 11 and 12 wide (B = $0E - C).
	a := byte(0x44)
	b, cc := byte(3), byte(3)
	for {
		row := de
		for ; b > 0; b-- {
			c.vwrbyte(row, a)
			row++
			a++
		}
		de += 0x20
		b = 0x0E - cc
		cc--
		if cc == 0 {
			break
		}
	}
	c.vfill(de, 0x0C, 0)
	c.RAM[0x00A]--
	return c.RAM[0x00A] == 0
}

// title481E redraws the screen for the title. Ported from $481E-$4838.
func (c *Console) title481E() {
	c.loadCommon()
	c.vfill(0x1080, 0x180, 0x70)
	c.RAM[0x00A] = 0
	c.vfill(0x3966, 0x13, 0)
}

// title4839 draws one 3-tile column of the title per tick while $E00A <
// $17, then prints the text at $5785 each tick until $E00A reaches $34.
// Returns true (carry) while not finished. Ported from $4839-$486A (High).
func (c *Console) title4839() bool {
	a := c.RAM[0x00A]
	c.RAM[0x00A]++
	if a >= 0x17 {
		c.printText(0x5785)
		return a < 0x34
	}
	de := uint16(0x3800) | uint16(0x85+a) // ADD A,E: no carry into D
	cc := a*2 + 0xB2
	v := byte(0)
	for b := 0; b < 3; b++ {
		c.vwrbyte(de, v)
		de = addDE(de, 0x20)
		v = cc
		cc++
	}
	return true
}

// wipe45A6 clears the sprites and one name-table column per call, from
// the centre outward, alternating sides via bit 6 of $E004. Returns true
// (the RET M) when finished. Ported from $45A6-$45DA (High).
func (c *Console) wipe45A6() bool {
	c.clearSprites()
	hl := uint16(0xE004)
	b := 0x18
	var e byte
	if c.rd(hl)&0x40 == 0 {
		e = 0x1F - c.rd(hl)
		c.wr(hl, c.rd(hl)|0x40)
	} else {
		c.wr(hl, c.rd(hl)&^0x40)
		v := c.rd(hl) - 1
		c.wr(hl, v)
		if v&0x80 != 0 {
			return true
		}
		e = v
	}
	if c.RAM[0x000] >= 0x0A {
		e += 0x40
		b -= 2
	}
	de := uint16(0x3800) | uint16(e)
	for ; b > 0; b-- {
		c.vwrbyte(de, 0)
		de = addDE(de, 0x20)
	}
	return false
}

// clearSprites zeroes the sprite attribute buffer at $E050 and copies it
// to the table at $3B00. Ported from $45DB-$45EF (High).
func (c *Console) clearSprites() {
	for i := uint16(0); i < 0x80; i++ {
		c.wr(0xE050+i, 0)
	}
	c.vcopy(0xE050, 0x3B00, 0x80)
}
