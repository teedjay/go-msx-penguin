package game

// Score, timer, distance and the status-line displays.

// newPresses returns the trigger bits ($30) newly pressed since the last
// tick, and the current input byte (B). Ported from $45F8-$4607 (High).
func (c *Console) newPresses() (a, b byte) {
	b = c.RAM[0x009]
	return b & 0x30 & ^(c.RAM[0x008] & 0x30), b
}

// addScore adds the BCD value de to the score at $E043-$E045 and keeps
// the hi-score at $E040-$E042, in live play only: ADD A,A / RET P tests
// bit 6 of $E002. An overflow of the top byte sets the hi-score to 999999
// with two overlapping 16-bit stores. Ported from $4608-$464B (High).
func (c *Console) addScore(de uint16) {
	if (c.RAM[0x002]+c.RAM[0x002])&0x80 == 0 {
		return
	}
	e, carry := adcDAA(c.RAM[0x043], byte(de), false)
	c.RAM[0x043] = e
	d, carry := adcDAA(c.RAM[0x044], byte(de>>8), carry)
	c.RAM[0x044] = d
	if carry {
		top, carry := adcDAA(c.RAM[0x045], 0, true)
		c.RAM[0x045] = top
		if carry {
			c.wr16(0xE040, 0x9999)
			c.wr16(0xE041, 0x9999)
			c.printScores()
			return
		}
	}
	// $4632: compare with the hi-score, top byte first.
	b := c.RAM[0x045]
	switch hi := c.RAM[0x042]; {
	case hi < b:
	case hi > b:
		c.printScore()
		return
	default:
		if c.rd16(0xE040) >= uint16(d)<<8|uint16(e) { // SBC HL,DE: no carry
			c.printScore()
			return
		}
	}
	c.wr16(0xE040, uint16(d)<<8|uint16(e))
	c.RAM[0x042] = b
	c.printScores()
}

// printScores draws the hi-score then the score. Ported from $46A4-$46B6.
func (c *Console) printScores() {
	c.printBCD(0xE042, 0x380F, 3)
	c.printScore()
}

// printScore draws the score. Ported from $46AD-$46B5.
func (c *Console) printScore() { c.printBCD(0xE045, 0x3805, 3) }

// drawStatus prints the status-line labels and all values. Ported from
// $4695-$46A3 (falls through into $46A4).
func (c *Console) drawStatus() {
	c.printText(0x5756)
	c.printTime()
	c.printDistance()
	c.printStage()
	c.printScores()
}

// printDistance draws the BCD distance at $E0E5. Ported from $46F1-$46FA.
func (c *Console) printDistance() { c.printBCD(0xE0E6, 0x382F, 2) }

// printStage draws the stage number at $E0E0. Ported from $46FB-$4702.
func (c *Console) printStage() { c.printBCD(0xE0E0, 0x381C, 1) }

// distanceStep counts the distance at $E0E5 (BCD) down every $E0E9
// ticks, the period following the speed $E100. At zero it sets $E00D.
// Ported from $46B7-$46F0, falling through into $46F1 (High).
func (c *Console) distanceStep() {
	c.RAM[0x0E9]--
	if c.RAM[0x0E9] != 0 {
		return
	}
	c.RAM[0x0E9] = c.RAM[0x100]>>1 - 1
	if c.RAM[0x0E6]|c.RAM[0x0E5] == 0 {
		c.RAM[0x00D] = 1
		return
	}
	lo, carry := subDAA(c.RAM[0x0E5], 1)
	c.RAM[0x0E5] = lo
	if carry {
		c.RAM[0x0E6], _ = subDAA(c.RAM[0x0E6], 1)
	}
	if lo == 0 && c.RAM[0x0E6] != 0 && c.RAM[0x0E6]&0x03 == 0 {
		c.RAM[0x107] = 1
	}
	c.upd52A5()
	c.printDistance()
}

// courseParams picks $E18A and the object-table pointer $E18B for the
// stage ($E0E0) and remaining distance ($E0E6). Ported from $4720-$4761
// (High for the arithmetic).
func (c *Console) courseParams() {
	stage := c.RAM[0x0E0] & 0x0F
	hl := addHL(0x4762, stage*2)
	if c.RAM[0x0E6]&0x10 != 0 {
		hl++
	}
	c.RAM[0x18A] = c.rd(hl)
	hl = c.rd16(addHL(0x479E, stage*2))
	a := rrca(rrca(c.RAM[0x0E6]&0xFC)) &^ 0x08
	if a >= 4 {
		a--
	}
	c.wr16(0xE18B, c.rd16(addHL(hl, a*2)))
}

// holdCount: while bit 0 of $E18E is set, count $E18F down (clearing
// $E18E at zero) and return true, meaning C = 3. Ported from $47DA-$47EB.
func (c *Console) holdCount() bool {
	if c.RAM[0x18E]&1 == 0 {
		return false
	}
	c.RAM[0x18F]--
	if c.RAM[0x18F] == 0 {
		c.RAM[0x18E] = 0
	}
	return true
}

// holdStart arms $E18E/$E18F at certain distances. Ported from
// $47EC-$4813 (Medium for the role).
func (c *Console) holdStart() {
	hl := addHL(0x4814, c.RAM[0x0E0]&0x0F)
	e, d := c.RAM[0x0E5], c.RAM[0x0E6]
	if d < 4 || e != 0 {
		return
	}
	if (c.RAM[0x0E0]+d)&0x03 != 2 {
		return
	}
	c.RAM[0x18E] = 3 // INC A from 2
	c.RAM[0x18F] = c.rd(hl)
}

// speedBar draws the speed meter: $E171-$E176 to name-table $3839.
// Ported from $7724-$775E (High).
func (c *Console) speedBar() {
	hl := uint16(0xE171)
	var cc byte
	if c.RAM[0x140]+c.RAM[0x142] == 0 {
		b := c.RAM[0x100]
		cc = b&1 + 0x42
		n := ^(b>>1)&0x0F - 6 // RRA: carry is clear after ADD A,$42
		for ; n > 0; n-- {
			c.wr(hl, 0x42)
			hl++
		}
	}
	for {
		c.wr(hl, cc)
		hl++
		if byte(hl) == 0x78 {
			break
		}
		cc = 0
	}
	c.vcopy(0xE171, 0x3839, 6)
}

// speedControl applies up/down to the speed $E100 and derives $E148 and
// $E10E. Ported from $769A-$76E4 (High).
func (c *Console) speedControl() {
	c.speedInput()
	c.RAM[0x148] = c.RAM[0x100] >> 1
	cc := byte(0x2C)
	if c.RAM[0x0E6]&0x0C == 0 {
		cc += 4
	}
	if a := c.RAM[0x0E0] & 0xF0; a != 0 {
		if a&0xE0 != 0 {
			cc -= 4
		}
		cc -= 4
	}
	switch a := c.RAM[0x100]; {
	case a < 0x0C:
		cc -= 12
	case a&0x0C == 0:
		cc -= 4
	case a&0x0C == 0x0C:
		cc -= 8
	}
	c.RAM[0x10E] = cc
}

// speedInput: up (bit 0) slows down one step every 12 ticks, down
// (bit 1) speeds up one step every 4 ticks. Ported from $76E5-$7723.
func (c *Console) speedInput() {
	switch c.RAM[0x009] & 0x03 {
	case 1: // $76F5
		c.RAM[0x0FD] = 0
		c.RAM[0x0FF] = 0
		c.RAM[0x0FE]++
		if c.RAM[0x0FE] != 0x0C {
			return
		}
		c.RAM[0x0FE] = 0
		if c.RAM[0x100] < 9 {
			return
		}
		c.RAM[0x100]--
	case 2: // $770D
		c.RAM[0x0FD] = 0
		c.RAM[0x0FE] = 0
		c.RAM[0x0FF]++
		if c.RAM[0x0FF] != 4 {
			return
		}
		c.RAM[0x0FF] = 0
		if c.RAM[0x100] >= 0x13 {
			return
		}
		c.RAM[0x100]++
	}
}
