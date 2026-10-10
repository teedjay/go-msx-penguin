package game

// States 7-15.

// state7 is play with demo input; sub-states via $409A. Ported from
// $4180-$4185.
func (c *Console) state7() {
	switch c.RAM[0x001] {
	case 0:
		c.state7sub0()
	case 1:
		c.state7sub1()
	case 2:
		c.state7sub2()
	}
}

// state7sub0 starts the demo: new game, demo input from $57F4 for $073C
// ticks, then the stage start of state 9 (which moves on to state 15).
// Ported from $418C-$41A2 (High).
func (c *Console) state7sub0() {
	c.newGame()
	c.RAM[0x002] &^= 0x40
	c.wr16(0xE0EE, 0x073C)
	c.wr16(0xE0EC, 0x57F4)
	c.state9()
}

// state7sub1 runs the demo game and counts it down. Ported from
// $41A3-$41C3 (High).
func (c *Console) state7sub1() {
	c.printAt(0x5787, 0x38CB)
	c.RAM[0x133] = 1
	c.playTick()
	n := c.rd16(0xE0EE) - 1
	c.wr16(0xE0EE, n)
	if n != 0 {
		return
	}
	c.RAM[0x133] = 0 // A = H | L = 0
	c.nextSub43F7()
}

// state7sub2 wipes the screen and restarts the attract sequence at state
// 1. Ported from $41C4-$41CE.
func (c *Console) state7sub2() {
	if !c.wipe45A6() {
		return
	}
	c.RAM[0x000] = 0
	c.nextState43EE()
}

// state8 is the start of a real game. Ported from $41CF-$41D4.
func (c *Console) state8() {
	switch c.RAM[0x001] {
	case 0:
		c.state8sub0()
	case 1:
		c.state8sub1()
	case 2:
		c.state8sub2()
	case 3:
		c.state8sub3()
	}
}

// state8sub0 redraws the title and starts the game tune. Ported from
// $41DD-$41ED (High).
func (c *Console) state8sub0() {
	c.clearSprites()
	c.clearNames()
	c.title481E()
	c.startSound(0x92)
	c.nextSub43FC()
}

// state8sub1 draws the whole title in one tick. Ported from $41EE-$4200.
func (c *Console) state8sub1() {
	for c.title4839() {
	}
	c.printText(0x5793)
	c.RAM[0x18D] = 6
	c.nextSub43FC()
}

// state8sub2 blinks the chosen play-select line every 8 ticks, 6 times.
// Ported from $4201-$4230 (High).
func (c *Console) state8sub2() {
	a := c.RAM[0x003]
	if a&0x07 != 0 {
		return
	}
	if a&0x08 == 0 {
		// RLCA twice of ($E002 & $10): $00 or $40 rows further.
		de := addDE(0x3A00, rlca(rlca(c.RAM[0x002]&0x10)))
		c.vfill(de, 0x20, 1)
		return
	}
	c.printText(0x5793)
	c.RAM[0x18D]--
	if c.RAM[0x18D] != 0 {
		return
	}
	c.nextSub43F7()
}

// state8sub3 wipes, resets the game and moves to state 9. Ported from
// $4231-$423A.
func (c *Console) state8sub3() {
	if !c.wipe45A6() {
		return
	}
	c.newGame()
	c.nextState43EE()
}

// state9 sets up the stage time and distance, draws the status line and
// goes to state 15 (stage intro). Ported from $423B-$427C (High).
func (c *Console) state9() {
	hl := addHL(0x4AAE, c.RAM[0x0E8]*4)
	c.wr16(0xE0E6, c.rd16(hl)) // distance, BCD (E0E6/E0E7)
	de := c.rd16(hl + 2)       // time, BCD
	a := c.rd(addHL(0xE0D5, c.RAM[0x0E1]))
	// Time bonus: subtract ($E0D5+n) - $10 (BCD) when it is at least $10.
	if a >= 0x10 {
		v, _ := subDAA(a, 0x10)
		e := byte(de)
		r, _ := subDAA(e, v) // both paths DAA; JR NC tests SUB's carry
		if e < v {
			de -= 0x100
		}
		de = de&0xFF00 | uint16(r)
	}
	c.wr16(0xE0E3, de)
	c.drawStatus()
	c.loadCommon()
	c.RAM[0x000] = 0x0E
	c.nextState43E9()
}

// state10 wipes, sets up the stage and starts the music in live play.
// Ported from $427D-$4295 (High).
func (c *Console) state10() {
	if !c.wipe45A6() {
		return
	}
	c.stageSetup()
	if c.RAM[0x002]&0x40 != 0 {
		c.startSound(0x8A)
	}
	c.RAM[0x133] = 1
	c.nextState43EE()
}

// state11 is live play; in the demo it jumps straight to state 7 sub 1.
// Ported from $4296-$42BB (High).
func (c *Console) state11() {
	if c.RAM[0x002]&0x40 == 0 {
		c.wr16(0xE000, 0x0107) // state 7, sub-state 1
		return
	}
	c.playTick()
	if c.RAM[0x00C]+c.RAM[0x00D] == 0 {
		return
	}
	c.RAM[0x133] = 0
	if c.RAM[0x00C] != 0 {
		c.RAM[0x000] = 0x0C // time out
	} else {
		c.RAM[0x000] = 0x0E // distance reached
	}
}

// state12: time out. Ported from $42BC-$42E0 (High).
func (c *Console) state12() {
	c.RAM[0x00C] = 0
	for i := uint16(0); i < 4; i++ {
		c.wr(0xE0B8+i*4, 0xE0)
	}
	c.putSprites()
	c.RAM[0x0E2] = 0 // A: $669D's copy loop exits with A = B|C = 0
	c.startSound(0x8C)
	c.printText(0x57D3)
	c.nextState43E9()
}

// state13 waits for the tune to end, then returns to the title (state 7
// sub 2 wipes back to state 1). Ported from $42E1-$42F1.
func (c *Console) state13() {
	if c.RAM[0x012] != 0 {
		return
	}
	c.RAM[0x002] &^= 0x40
	c.wr16(0xE000, 0x0207)
}

// state14 is the stage-clear sequence. Ported from $42F2-$42F7.
func (c *Console) state14() {
	switch c.RAM[0x001] {
	case 0:
		c.clear4308()
	case 1:
		c.clear431B()
	case 2:
		c.clear435B()
	case 3:
		c.clear4369()
	case 4:
		c.clear4386()
	case 5:
		c.clear43B0()
	case 6:
		c.clear43B9()
	case 7:
		c.clear43E0()
	}
}

// clear4308 finishes a jump in progress. Ported from $4308-$431A.
func (c *Console) clear4308() {
	if c.RAM[0x0F9] == 0 {
		c.nextSub43FC()
		return
	}
	c.jump4BED(0xE0F9)
	if c.RAM[0x0F9] != 0 {
		return
	}
	c.nextSub43FC()
}

// clear431B advances to the next stage. Ported from $431B-$435A (High).
func (c *Console) clear431B() {
	c.RAM[0x0E0], _ = adcDAA(c.RAM[0x0E0], 1, false)
	old := c.RAM[0x0E1]
	a := old + 1
	if a >= 0x0A {
		a = 0
		c.RAM[0x0E2] = 0
	}
	c.RAM[0x0E1] = a
	c.wr(addHL(0xE0D5, old), c.RAM[0x0E3])
	c.RAM[0x00D] = 0
	c.RAM[0x0E8]++
	if c.RAM[0x0E8] == 0x0A {
		c.RAM[0x0E8] = 0
	}
	c.wr16(0xE138, uint16(c.RAM[0x079])<<8|0x01)
	c.RAM[0x100] = 0x13
	c.nextSub43FC()
}

// clear435B walks the penguin towards the station. Ported from
// $435B-$4368.
func (c *Console) clear435B() {
	if !c.walk5489(0xFF, 0) {
		return
	}
	c.RAM[0x138] = 0x0C
	c.nextSub43FC()
}

// clear4369 walks on, then shows the station and falls through into
// clear4386. Ported from $4369-$4385 (High).
func (c *Console) clear4369() {
	if !c.walk5489(0x00, c.RAM[0x079]) {
		return
	}
	c.loadSprites6681()
	c.flag54CB()
	c.station552F()
	c.startSound(0x8F)
	c.RAM[0x001] = 4
	c.clear4386()
}

// clear4386 raises the flag while the tune plays. Ported from
// $4386-$43AF (High).
func (c *Console) clear4386() {
	if c.RAM[0x01A]-1 != 0 {
		return
	}
	c.station556B()
	if a := c.RAM[0x0E1]; (a == 0 || a == 5) && c.RAM[0x13A] == 0x0F {
		c.station54E1()
		c.nextSub43FC()
		return
	}
	c.flag54CF()
	if c.RAM[0x13A] != 0x10 {
		return
	}
	c.nextSub43FC()
}

// clear43B0 waits for the tune. Ported from $43B0-$43B8.
func (c *Console) clear43B0() {
	if c.RAM[0x012] != 0 {
		return
	}
	c.nextSub43F9(0x10)
}

// clear43B9 converts the remaining time into score. Ported from
// $43B9-$43DF (High).
func (c *Console) clear43B9() {
	if c.RAM[0x004] != 0 {
		c.RAM[0x004]--
		return
	}
	if c.RAM[0x003]&0x03 != 0 {
		return
	}
	if c.RAM[0x0E3]+c.RAM[0x0E4] == 0 { // ADD A,L: 8-bit
		c.nextSub43F7()
		return
	}
	c.timeDec(0)
	c.addScore(0x0100)
	c.startSound(0x01)
}

// clear43E0 wipes and starts the next stage at state 9 (via 8+1). Ported
// from $43E0-$43E8.
func (c *Console) clear43E0() {
	if !c.wipe45A6() {
		return
	}
	c.RAM[0x000] = 0x08
	c.nextState43E9()
}

// state15 is the stage intro map. Ported from $48DD-$48E2.
func (c *Console) state15() {
	switch c.RAM[0x001] {
	case 0:
		c.intro48F1()
	case 1:
		c.intro490B()
	case 2:
		c.intro4912()
	case 3:
		c.intro4949()
	case 4:
		c.intro4965()
	case 5:
		c.intro4972()
	case 6:
		c.intro49C9()
	}
}

// intro48F1 sets up the map drawing. Ported from $48F1-$490A.
func (c *Console) intro48F1() {
	c.wr16(0xE0F2, 0x49D6)
	c.wr16(0xE0F0, 0x3884)
	c.vfill(0x1080, 0x180, 0xF4)
	c.nextSub43FC()
}

// intro490B / intro4949 draw the top and bottom border. Ported from
// $490B-$4911, $4949-$4964 (High).
func (c *Console) intro490B() { c.border494E(0x3883, 0x92) }
func (c *Console) intro4949() { c.border494E(0x3AA3, 0x91) }

func (c *Console) border494E(de uint16, a byte) {
	c.vwrbyte(de, a)
	de++
	a += 4
	c.vfill(de, 0x18, a)
	c.vout(a - 2)
	c.nextSub43FC()
}

// intro4912 draws the map one row every other tick from the list at
// $E0F2. Ported from $4912-$4948 (High).
func (c *Console) intro4912() {
	if c.RAM[0x003]&1 != 0 {
		return
	}
	hl := c.rd16(0xE0F0) + 0x20
	c.wr16(0xE0F0, hl)
	de := hl
	c.vfill(de, 0x18, 0x0A)
	de++
	c.vfill(de, 0x16, 0x04)
	hl = c.rd16(0xE0F2)
	a := c.rd(hl)
	hl++
	if a == 0 {
		c.nextSub43FC()
		return
	}
	if a != 0xFF {
		hl = c.printAt(hl, de&0xFF00|uint16(a))
	}
	c.wr16(0xE0F2, hl)
}

// intro4965 starts the route drawing. Ported from $4965-$4971.
func (c *Console) intro4965() {
	c.wr16(0xE0F4, 0x3913)
	c.RAM[0x0F6] = 0
	c.nextSub43FC()
}

// intro4972 draws the route one step every other tick: each byte of the
// list at $4A85 has a direction (high nibble 0/4/8/C) and a tile offset;
// stages already passed use tiles from $A4, the rest from $97. Ported
// from $4972-$49C8 (High).
func (c *Console) intro4972() {
	if c.RAM[0x003]&1 != 0 {
		return
	}
	n := c.RAM[0x0F6]
	a := c.rd(addDE(0x4A85, n))
	c.RAM[0x0D0] = a
	if a == 0x20 {
		c.nextSub43FC()
		return
	}
	c.RAM[0x0F6] = n + 1
	cc := byte(0x97)
	if c.RAM[0x0E7] >= c.RAM[0x0F6] {
		cc = 0xA4
	}
	// XOR A / RRD: A = low nibble, ($E0D0) = high nibble.
	v := c.RAM[0x0D0]
	b := v & 0x0F
	c.RAM[0x0D0] = v >> 4
	de := c.rd16(0xE0F4)
	switch c.RAM[0x0D0] { // JP (HL) into $49B6 + 0/4/8/C
	case 0x0:
		de = addDE(de-0x100, 0xE0) // up
	case 0x4:
		de = addDE(de, 0x01) // right
	case 0x8:
		de = addDE(de, 0x20) // down
	case 0xC:
		de = addDE(de-0x100, 0xFF) // left
	default:
		c.unported(0x49B5) // TODO: a direction value the route data never uses
	}
	c.wr16(0xE0F4, de)
	c.vwrbyte(de, b+cc)
}

// intro49C9 waits, then moves to state 10. Ported from $49C9-$49D5.
func (c *Console) intro49C9() {
	c.RAM[0x004]--
	if c.RAM[0x004] != 0 {
		return
	}
	c.RAM[0x000] = 9
	c.nextState43E9()
}

// Sub-state transitions.

// nextSub43F7: $E004 = $50, next sub-state. Ported from $43F7.
func (c *Console) nextSub43F7() { c.nextSub43F9(0x50) }

// nextSub43F9: $E004 = a, next sub-state. Ported from $43F9.
func (c *Console) nextSub43F9(a byte) {
	c.RAM[0x004] = a
	c.nextSub43FC()
}

// nextSub43FC: next sub-state. Ported from $43FC-$4400.
func (c *Console) nextSub43FC() { c.RAM[0x001]++ }

func rlca(a byte) byte { return a<<1 | a>>7 }
