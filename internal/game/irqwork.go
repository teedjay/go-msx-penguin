package game

// Per-interrupt work other than sound.

// upd4C98 animates the penguin every 8th frame unless $E130 or $E0F9 is
// set. Ported from $4C98-$4CBD (High for the flow, Medium for the role).
func (c *Console) upd4C98() {
	if c.RAM[0x130]|c.RAM[0x0F9] != 0 {
		return
	}
	if c.RAM[0x003]&0x07 != 0 {
		return
	}
	c.penguinFrame()
}

// penguinFrame steps the waddle counter $E0F8 and shows the matching
// frame. Ported from $4CA6-$4CBD.
func (c *Console) penguinFrame() {
	c.RAM[0x0F8]++
	a := c.RAM[0x0F8]
	var cc byte
	if a&1 != 0 {
		cc = 4
		if a&2 != 0 {
			cc = 8
		}
	}
	c.setPenguinPatterns(cc)
	c.putPenguin()
}

// setPenguinPatterns copies four sprite pattern numbers from $4B59+a into
// the penguin's attribute buffer ($E07A, every 4th byte). Runs in the
// alternate register set, so callers keep BC/DE/HL. Ported from
// $4BAA-$4BC0 (High).
func (c *Console) setPenguinPatterns(a byte) {
	hl := addHL(0x4B59, a)
	de := uint16(0xE07A)
	for b := 0; b < 4; b++ {
		c.wr(de, c.rd(hl))
		de = de&0xFF00 | uint16(byte(de)+4) // ADD A,E: no carry into D
		hl++
	}
}

// putPenguin copies the penguin's four sprites to the attribute table,
// then places its shadow. Ported from $4B9B-$4BA9 (High).
func (c *Console) putPenguin() {
	c.vcopy(0xE078, 0x3B28, 0x10)
	c.putShadow()
}

// putShadow derives the shadow sprite pair at $E0A0 from the penguin's
// position and copies it to the table at $3B50. Ported from $4CBE-$4CFE
// (High for the arithmetic, Medium for the role).
func (c *Console) putShadow() {
	hl := c.rd16(0xE078)
	l := byte(hl) + 0x1E
	h := byte(hl >> 8)
	cc := l
	b := h + 0x10
	table := uint16(0x4CFE)
	a := c.RAM[0x0F9]
	ok := a != 0
	if !ok {
		table = 0x4D08
		a = c.RAM[0x143]
		ok = a != 0
	}
	if ok {
		t := c.rd(addHL(table, a))
		h += t
		b -= t
		l, cc = 0xAE, 0xAE
	}
	c.wr16(0xE0A0, uint16(h)<<8|uint16(l))
	c.wr16(0xE0A4, uint16(b)<<8|uint16(cc))
	c.vcopy(0xE0A0, 0x3B50, 0x08)
}

// upd464C counts the timer at $E0E3 (BCD) down once every 64 frames while
// $E133 is set, sounding $09 below 11, and redraws it. Sets $E00C when it
// is already zero. Ported from $464C-$4694 (High for the arithmetic,
// Medium for the role).
func (c *Console) upd464C() {
	if c.RAM[0x133] == 0 {
		return
	}
	if c.RAM[0x0E3]+c.RAM[0x0E4] == 0 { // ADD A,L: 8-bit
		c.RAM[0x00C] = 1
		return
	}
	if c.RAM[0x003]&0x3F != 0 {
		return
	}
	c.timeDec(1)
}

// timeDec counts the BCD timer at $E0E3 down by one and redraws it. With
// cc = 1 (the per-interrupt path) it sounds $09 when the time drops below
// 11; $43D1 enters with cc = 0, so DEC C never reaches 0 there and the
// sound never plays on that path. Ported from $4665-$4692 (High).
func (c *Console) timeDec(cc byte) {
	lo, carry := subDAA(c.RAM[0x0E3], 1)
	c.RAM[0x0E3] = lo
	hi := c.RAM[0x0E4]
	if carry {
		hi, _ = subDAA(hi, 1)
		c.RAM[0x0E4] = hi
	}
	if hi == 0 && c.RAM[0x0E3] < 0x11 {
		cc--
		if cc == 0 {
			c.startSound(0x09)
		}
	}
	c.printTime()
}

// printTime draws the two-byte BCD value at $E0E3 at name-table $3827.
// Ported from $468A-$4692.
func (c *Console) printTime() { c.printBCD(0xE0E4, 0x3827, 2) }

// printBCD writes b BCD bytes, high byte first (hl walks down), as digit
// tiles $10-$19. Ported from $4703-$471F (High).
func (c *Console) printBCD(hl, de uint16, b int) {
	for ; b > 0; b-- {
		a := c.rd(hl)
		lo := a&0x0F | 0x10
		c.vwrbyte(de, a>>4|0x10)
		de++
		c.vwrbyte(de, lo)
		hl--
		de++
	}
}
