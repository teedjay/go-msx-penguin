package game

// Screen building: stage graphics, name-table streams, the horizon row.

// nameStream writes a name-table stream: a first byte whose high nibble
// is the starting row offset and low 2 bits the page ($3800 + n*$100),
// then position bytes >= $E0 (next row, column = byte - $E0) followed by
// tile bytes, ended by $00. add is added to every tile ($40 for $74D3).
// It returns the address of the terminating $00. Ported from $4512-$453E
// (add 0, with the leading zero check) and $74EC-$7517 (High).
func (c *Console) nameStream(hl uint16, add byte, checkFirst bool) uint16 {
	if checkFirst && c.rd(hl) == 0 {
		return hl
	}
	cc := c.rd(hl) & 0xF0
	d := c.rd(hl)&0x03 + 0x78
	hl++
	for {
		b := c.rd(hl) // position byte
		hl++
		sum := uint16(cc) + 0x20
		cc = byte(sum)
		if sum > 0xFF {
			d++
		}
		e := cc + b - 0xE0
		c.vsetwr(uint16(d)<<8 | uint16(e))
		for {
			a := c.rd(hl)
			if a == 0 {
				return hl
			}
			if a >= 0xE0 {
				break
			}
			hl++
			c.vout(a + add)
		}
	}
}

// nameStream4512 is the plain entry. Ported from $4512.
func (c *Console) nameStream4512(hl uint16) uint16 { return c.nameStream(hl, 0, true) }

// fillRuns writes runs of one value: first byte the value, then (count,
// column) pairs until a zero count. The page D starts at $39 and moves to
// the next page for every column below $20 (it is never reset). Ported
// from $44F1-$4511 (High).
func (c *Console) fillRuns(hl uint16) uint16 {
	c.RAM[0x0DF] = c.rd(hl)
	hl++
	d := byte(0x39)
	for {
		cnt := c.rd(hl)
		hl++
		if cnt == 0 {
			return hl
		}
		e := c.rd(hl)
		hl++
		if e < 0x20 {
			d++
		}
		c.vfill(uint16(d)<<8|uint16(e), uint16(cnt), c.RAM[0x0DF])
	}
}

// copy4 copies the 4 bytes at hl to de, cc times, and returns hl+4 and
// the advanced de. Ported from $4595-$45A5 (High).
func (c *Console) copy4(hl, de uint16, cc byte) (uint16, uint16) {
	for {
		for i := uint16(0); i < 4; i++ {
			c.wr(de, c.rd(hl+i))
			de++
		}
		cc--
		if cc == 0 {
			return hl + 4, de
		}
	}
}

// clearNames clears the name table. Ported from $4495-$449C.
func (c *Console) clearNames() { c.vfill(0x3800, 0x300, 0) }

// newGame resets the game variables. Ported from $4443-$4470 (High).
func (c *Console) newGame() {
	c.RAM[0x043] = 0
	c.ldir(0xE043, 0xE044, 0x100) // clears $E043-$E143
	c.ldir(0x4471, 0xE0E0, 9)
	c.vfill(0x0900, 0x100, 0xF0)
	for i := uint16(0); i < 10; i++ {
		c.wr(0xE0D5+i, 5)
	}
}

// stageSetup clears the play variables and builds the stage screen.
// Ported from $4AD6-$4B1E (High).
func (c *Console) stageSetup() {
	c.RAM[0x0F0] = 0
	c.ldir(0xE0F0, 0xE0F1, 0x130) // clears $E0F0-$E220
	c.wr16(0xE100, 0x1010)
	c.RAM[0x110] = 0x10
	c.RAM[0x149] = 0x08
	c.RAM[0x0E9] = 0x05
	hl := uint16(0x3030)
	if c.RAM[0x0E0]&1 != 0 {
		hl = 0x3434
	}
	c.wr16(0xE10E, hl)
	c.RAM[0x13B] = 1
	c.stageGfx5DAC()
	c.stageGfx6249()
	c.rleDest(0x6716) // $6710
	c.loadSprites667C()
	c.stageScreen5019()
	c.RAM[0x13B] = 0
}

// stageGfx5DAC loads stage graphics; streams are reused mirrored. Ported
// from $5DAC-$5DDB (High for the calls).
func (c *Console) stageGfx5DAC() {
	c.rleDest(0x5DDC)
	hl := c.rleAt(0x5DDE, 0x6A88, true)
	c.rleDest(hl)
	c.rleDest(0x6164)
	hl = c.rleAt(0x616B, 0x4A88, false)
	c.rleDest(hl)
	c.rleDest(0x6169)
	c.rleDest(0x623A)
}

// stageGfx6249 loads more stage graphics. The $454C at $6255 continues at
// the VRAM address left by the previous stream in normal mode ($453F
// leaves C = 0). Ported from $6249-$627E (High for the calls).
func (c *Console) stageGfx6249() {
	hl := c.rleDest(0x627F)
	c.rleDest(hl)
	c.rleCont(0x5BE0, false)
	c.rleAt(0x6281, 0x72B0, true)
	c.rleDest(0x660F)
	hl = c.rleDest(0x6533)
	c.rleDest(hl)
	c.rleAt(0x6535, 0x52B0, false)
	c.rleDest(0x666D)
}

// loadSprites667C / loadSprites6681 fill the sprite buffer at $E050 from
// a list of (count, 4 bytes) records and copy it to the table. Ported
// from $667C-$66A8 (High).
func (c *Console) loadSprites667C() { c.loadSpriteList(0x66A9) }
func (c *Console) loadSprites6681() { c.loadSpriteList(0x66E6) }

func (c *Console) loadSpriteList(hl uint16) {
	for i := uint16(0); i < 0x80; i++ {
		c.wr(0xE050+i, 0)
	}
	de := uint16(0xE050)
	for {
		n := c.rd(hl)
		hl++
		if n == 0 {
			break
		}
		hl, de = c.copy4(hl, de, n)
	}
	c.putSprites()
}

// putSprites copies the sprite buffer to the attribute table. Ported from
// $669D-$66A8.
func (c *Console) putSprites() { c.vcopy(0xE050, 0x3B00, 0x80) }

// stageScreen5019 draws the course screen for stage $E0E1. Ported from
// $5019-$5095 (High).
func (c *Console) stageScreen5019() {
	hl := addHL(0x5145, c.RAM[0x0E1])
	a := byte(0x07)
	if c.rd(hl)&1 != 0 {
		a = 0x09
	}
	c.RAM[0x10C] = a
	src, dst := uint16(0x5D9E), uint16(0x6200)
	if c.rd(hl) != 0 {
		src, dst = 0x5DA9, 0x621D
	}
	c.rleAt(src, 0x4588, false)
	c.rleAt(dst, 0x4F78, false) // POP HL: the second pointer is a stream
	c.vfill(0x3860, 0xE0, c.RAM[0x10C])
	c.vfill(0x3940, 0x1C0, 0x0F)
	c.horizon50C6(0x720B)
	c.horizon50C6(0x7248)
	c.wr16(0xE10A, addHL(0x5119, c.RAM[0x0E1]*4))
	c.RAM[0x102] = 0
	c.RAM[0x108] = 0
	c.wr16(0xE103, 0x7203)
	c.wr16(0xE105, 0x7240)
	c.edge5160()
	c.edge5170()
}

// horizon50C6 draws a fill-run block and text, then builds the 16-byte
// half of the horizon row named by the next byte and prints the row.
// Ported from $50C6-$50CC, falling into $50CD.
func (c *Console) horizon50C6(hl uint16) {
	hl = c.fillRuns(hl)
	hl = c.printText(hl)
	c.horizonRow(hl, c.rd(hl))
}

// horizonRow copies 16 bytes after hl into $E1xx (xx = e), replacing 0
// with the colour byte $E10C, then prints the whole row. Ported from
// $50CD-$50DE.
func (c *Console) horizonRow(hl uint16, e byte) {
	cc := c.RAM[0x10C]
	de := uint16(0xE100) | uint16(e)
	for b := 0; b < 0x10; b++ {
		hl++
		a := c.rd(hl)
		if a == 0 {
			a = cc
		}
		c.wr(de, a)
		de++
	}
	c.printHorizon()
}

// printHorizon prints the 32-byte row buffer $E150-$E16F at $3920 by
// building a text record in RAM ($E14E address, $E170 = $FF). Returns
// with A = 0. Ported from $50DF-$50F2 (High).
func (c *Console) printHorizon() {
	c.wr16(0xE14E, 0x3920)
	c.RAM[0x170] = 0xFF
	c.printText(0xE14E)
}

// horizonPart draws the half row for the current wind side: from
// $722F ($5476) or $72A1 ($5471), using E = first byte, minus $10 when
// bit 0 of $E194 is set. Only when bit 1 of $E194 is set. Ported from
// $5471-$5488 (High).
func (c *Console) horizonPart(hl uint16) {
	a := c.RAM[0x194]
	if a&0x02 == 0 {
		return
	}
	e := c.rd(hl)
	if a&1 != 0 { // RRA: carry = bit 0
		e -= 0x10
	}
	c.horizonRow(hl, e)
}

// edge... draw the course edges from the pointer tables at $E103/$E105,
// indexed by $E102. Ported from $5160-$517E (High).

// edge5160: entry $5160, table $E105, index $E102.
func (c *Console) edge5160() { c.edgeDraw(c.rd16(0xE105), c.RAM[0x102]) }

// edge5169: entry $5169, steps $E102 (keeping bit 2 clear), table $E103.
func (c *Console) edge5169() {
	a := c.RAM[0x102]
	c.RAM[0x102] = (a + 1) &^ 0x04
	c.edgeDraw(c.rd16(0xE103), a)
}

// edge5170: entry $5170, table $E103, with A = 0: the preceding
// nameStream4512 always returns through OR A / RET Z.
func (c *Console) edge5170() { c.edgeDraw(c.rd16(0xE103), 0) }

// edgeDraw: $5173 HL += A*2, load the stream pointer, nameStream4512.
func (c *Console) edgeDraw(hl uint16, a byte) {
	c.nameStream4512(c.rd16(addHL(hl, a*2)))
}

// stageText52EF / stageText52F2 print the text picked by $E194 from the
// pointer table at hl. Ported from $52EF-$5300.
func (c *Console) stageText52EF() { c.stageText52F2(0x53B2) }

func (c *Console) stageText52F2(hl uint16) {
	c.printText(c.rd16(addHL(hl, c.RAM[0x194]*2)))
}
