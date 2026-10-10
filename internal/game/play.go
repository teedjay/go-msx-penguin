package game

// The gameplay tick and the penguin.

// playTick is one tick of play. Ported from $4B1F-$4B58 (High).
func (c *Console) playTick() {
	c.speedBar() // $7724
	c.upd7625()  // $7625
	if c.RAM[0x140] != 0 {
		c.crashed4F69()
		return
	}
	if c.RAM[0x142] != 0 {
		c.hit4E3E()
		return
	}
	c.speedControl() // $769A
	c.penguin4B81()
	c.wind5366()
	c.collide4DA7()
	c.collide4DE4()
	if c.RAM[0x140] != 0 {
		return
	}
	c.course514F()
	c.upd74D3()
	c.distanceStep() // $46B7
	c.courseParams() // $4720
	c.spawn51D7()
	c.upd775F()
}

// penguin4B81 steers, or starts or continues a jump. Ported from
// $4B81-$4B97 (High).
func (c *Console) penguin4B81() {
	if c.RAM[0x0F9] != 0 {
		c.jump4BED(0xE0F9)
		return
	}
	a, b := c.newPresses()
	if a != 0 {
		c.jumpStart4BD9(b)
		return
	}
	de := c.rd16(0xE078)
	d := c.steer4C61(b, byte(de>>8))
	c.place4B98(uint16(d)<<8 | de&0xFF) // EX DE,HL
}

// place4B98 positions the penguin's sprites at hl (H = X, L = Y) and
// shows them. Ported from $4B98-$4B9A.
func (c *Console) place4B98(hl uint16) {
	c.setPositions4BC1(hl)
	c.putPenguin()
}

// setPositions4BC1 writes the four penguin sprites' Y/X from hl (H = X,
// L = Y): TL, TR (+16 X), BL (+16 Y), BR. Ported from $4BC1-$4BD8 (High).
func (c *Console) setPositions4BC1(hl uint16) {
	l, h := byte(hl), byte(hl>>8)
	d := h
	c.RAM[0x078], c.RAM[0x079] = l, h
	h += 0x10
	c.RAM[0x07C], c.RAM[0x07D] = l, h
	l += 0x10
	c.RAM[0x080], c.RAM[0x081] = l, d
	c.RAM[0x084], c.RAM[0x085] = l, h
}

// steer4C61 moves X (d) left or right from input bits 2/3; both pressed
// keeps alternating via $E0FA. Ported from $4C61-$4C70, $4C7B-$4C8D.
func (c *Console) steer4C61(a, d byte) byte {
	a &= 0x0C
	if a == 0 {
		return d
	}
	const hl = 0xE0FA
	if a == 0x0C {
		v := c.rd(hl)
		if v == 0 {
			return d
		}
		if v&0x80 == 0 {
			c.wr(hl, c.rd(hl)|0x80)
			if v&0x02 != 0 {
				return c.left4C71(d, hl)
			}
			return c.right4C8E(d, hl)
		}
		if v&0x01 != 0 {
			return c.left4C71(d, hl)
		}
		return c.right4C8E(d, hl)
	}
	c.wr(hl, c.rd(hl)&^0x80)
	if a != 0x04 {
		return c.right4C8E(d, hl)
	}
	return c.left4C71(d, hl)
}

// left4C71 moves X one pixel left, down to $14, and marks bits 0/1 of
// (hl). Callers pass different hl: $E0FA, $E0FB, $E0D0 or $E0D1. Ported
// from $4C71-$4C7A (High).
func (c *Console) left4C71(d byte, hl uint16) byte {
	if d < 0x14 {
		return d
	}
	d--
	c.wr(hl, c.rd(hl)|0x01)  // SET 0
	c.wr(hl, c.rd(hl)&^0x02) // RES 1
	return d
}

// right4C8E moves X one pixel right, up to $CB. Ported from $4C8E-$4C97.
func (c *Console) right4C8E(d byte, hl uint16) byte {
	if d >= 0xCC {
		return d
	}
	c.wr(hl, c.rd(hl)|0x02)
	c.wr(hl, c.rd(hl)&^0x01)
	return d + 1
}

func (c *Console) left3(d byte, hl uint16) byte { // $4EB7
	for i := 0; i < 3; i++ {
		d = c.left4C71(d, hl)
	}
	return d
}

func (c *Console) right3(d byte, hl uint16) byte { // $4EAE
	for i := 0; i < 3; i++ {
		d = c.right4C8E(d, hl)
	}
	return d
}

// jumpStart4BD9 starts a jump: sideways if left/right is held. Ported
// from $4BD9-$4BEC.
func (c *Console) jumpStart4BD9(b byte) {
	c.startSound(0x02)
	a := b & 0x0C
	if a != 0 {
		a = c.RAM[0x0FA] & 0x03
	}
	c.RAM[0x0FB] = a
	c.jumpStep(0xE0F9)
}

// jump4BED continues a jump every 4th tick. Ported from $4BED-$4BF2.
func (c *Console) jump4BED(hl uint16) {
	if c.RAM[0x003]&0x03 != 0 {
		return
	}
	c.jumpStep(hl)
}

// jumpStep advances the jump counter (hl = $E0F9, 0-11) and moves the
// penguin along the arc at $4C55. At the end it checks for objects and
// scores 30 for a cleared one. Ported from $4BF3-$4C54 (High).
func (c *Console) jumpStep(hl uint16) {
	a := c.rd(hl)
	c.wr(hl, a+1)
	if a == 0x0B {
		c.wr(hl, 0)
	}
	var cc byte
	switch {
	case a == 0x0B:
	case a&1 != 0:
		cc = 0x10
	default:
		cc = 0x0C
	}
	c.setPenguinPatterns(cc)
	de := c.rd16(0xE078)
	e := byte(de) + c.rd(addHL(0x4C55, a))
	d := byte(de >> 8)
	// HL = $E0FB here, so the steps below mark $E0FB, not $E0FA.
	switch c.RAM[0x0FB] {
	case 1:
		d = c.left4C71(c.left4C71(d, 0xE0FB), 0xE0FB)
	case 2:
		d = c.right4C8E(c.right4C8E(d, 0xE0FB), 0xE0FB)
	}
	c.place4B98(uint16(d)<<8 | uint16(e))
	if c.RAM[0x0F9] != 0 {
		return
	}
	c.collide4D1E()
	if c.RAM[0x140]+c.RAM[0x142] != 0 {
		return
	}
	if c.RAM[0x132] == 0 { // CP (HL) with A = 0
		return
	}
	c.RAM[0x132] = 0
	c.addScore(0x0030)
}

// collide4D1E checks the penguin (on the ground) against the active
// objects at $E112 (6 bytes each). Type $0D objects carry a pointer
// (+4/+5) to collision ranges. Ported from $4D1E-$4D6A (High for the
// control flow).
func (c *Console) collide4D1E() {
	if c.RAM[0x0F9] != 0 {
		return
	}
	n := 4
	if c.RAM[0x0E0] >= 5 {
		n = 5
	}
	hl := uint16(0xE112)
	for ; n > 0; n-- {
		if c.rd(hl) != 0x0D {
			hl += 6
			continue
		}
		obj := hl
		kind := c.rd(obj + 1)
		ptr := c.rd16(obj + 4)
		x := c.RAM[0x079]
		if 4 >= kind { // DEC A / CP C: no carry
			// $4D4F
			a := x
			if c.rd(ptr)-1 != 0 {
				a = x - c.rd(ptr)
				ptr++
				if a < c.rd(ptr) {
					c.crash4F20()
					return
				}
				a = x
			}
			// $4D5B
			ptr++
			a -= c.rd(ptr)
			ptr++
			if a < c.rd(ptr) {
				c.hit4DFB()
				return
			}
		} else {
			a := x - c.rd(ptr)
			ptr++
			if a < c.rd(ptr) {
				c.fish4FF1(obj + 5)
				return
			}
		}
		hl = obj + 6
	}
}

// collide4D6B checks a jumping penguin against type $0D objects, setting
// $E132 (scores 30 on landing). Original quirk, kept: on a miss the scan
// advances by the comparison result instead of the 5 bytes left in the
// object, so it skips unpredictably. Ported from $4D6B-$4D9C (High).
func (c *Console) collide4D6B() {
	if c.RAM[0x0F9] == 0 {
		return
	}
	hl := uint16(0xE112)
	for n := 5; n > 0; n-- {
		t := c.rd(hl)
		hl++
		a := byte(5)
		if t == 0x0D {
			tbl := addHL(0x4D9D, c.rd(hl)*2)
			a = c.RAM[0x079] - c.rd(tbl)
			tbl++
			if a < c.rd(tbl) {
				c.RAM[0x132] = 1
				return
			}
		}
		hl = addHL(hl, a)
	}
}

// collide4DA7 checks the penguin against the moving obstacle whose
// position $762A saves at $E188. Ported from $4DA7-$4DE3 (High for the
// arithmetic).
func (c *Console) collide4DA7() {
	if c.RAM[0x142]+c.RAM[0x140] != 0 {
		return
	}
	e, d := c.RAM[0x188], c.RAM[0x189]
	if e == 0xE0 {
		return
	}
	l, h := c.RAM[0x078], c.RAM[0x079]
	e -= l
	if e >= 0x0A { // SUB $0A / RET NC
		return
	}
	l = 0x13 + e
	e = e*2 + 0x17
	a := d - h - l
	if uint16(a)+uint16(e) <= 0xFF { // ADD A,E / RET NC
		return
	}
	c.startSound(0x07)
	c.objGone767F(0xE08C, 0xE183)
	c.upd762A()
	c.addScore(0x0300)
}

// collide4DE4 checks the penguin against the sprite at $E090 when its Y
// is $8F. Ported from $4DE4-$4DFA, $4E0F-$4E24 (High).
func (c *Console) collide4DE4() {
	l, h := c.RAM[0x090], c.RAM[0x091]
	if l != 0x8F {
		return
	}
	x := c.RAM[0x079]
	a := h - x
	borrow := h < x // flags kept by PUSH AF
	if uint16(a-0x18)+0x23 <= 0xFF {
		return
	}
	c.wr16(0xE136, 0x0101)
	c.startSound(0x08)
	hl := uint16(0x0102)
	if c.RAM[0x0F9] != 0 {
		hl++
	}
	c.knock4E25(hl, borrow) // POP AF
}

// hit4DFB: knocked by a ground obstacle. Ported from $4DFB-$4E0E.
func (c *Console) hit4DFB() {
	if c.RAM[0x135] != 0 {
		return
	}
	c.startSound(0x03)
	c.knock4E25(0x0101, ^c.RAM[0x0FA]&1 != 0) // CPL / RRA
}

// knock4E25 starts the knocked-aside sequence. Ported from $4E25-$4E3D.
func (c *Console) knock4E25(hl uint16, carry bool) {
	c.wr16(0xE142, hl)
	a := byte(0x20)
	if carry {
		a = 0x24
	}
	c.RAM[0x144] = a
	c.setPenguinPatterns(a)
	c.putPenguin()
	c.wr16(0xE100, 0x1313)
}

// hit4E3E runs the knocked-aside sequence every 4th tick. Ported from
// $4E3E-$4E80 (High).
func (c *Console) hit4E3E() {
	if c.RAM[0x003]&0x03 != 0 {
		return
	}
	if c.RAM[0x142] == 3 {
		c.hit4ED6()
		return
	}
	a := c.RAM[0x143]
	c.RAM[0x143]++
	cc := c.rd(addHL(0x4EC1, a)) // a starts at 1: $4EC1 itself is code
	de := c.rd16(0xE078)
	d := byte(de >> 8)
	for {
		if c.RAM[0x144]&0x04 == 0 {
			d = c.left3(d, 0xE0D0)
		} else {
			d = c.right3(d, 0xE0D0)
		}
		if c.RAM[0x142]-1 == 0 {
			break
		}
		c.RAM[0x142]--
	}
	c.place4B98(uint16(d)<<8 | uint16(byte(de)+cc))
	if c.RAM[0x078] != 0x90 {
		c.hitEnd4EA0()
		return
	}
	c.land4E81()
}

// land4E81 lands: redraws the course edges and objects. Returns the HL
// that $4EA0 leaves ($E143, or $E142 when it reset the sequence), which
// $4F17 writes through. Ported from $4E81-$4E9F (High).
func (c *Console) land4E81() uint16 {
	c.startSound(0x04)
	c.edge5160()
	c.edge5169()
	if c.RAM[0x136] != 0 {
		c.RAM[0x136] = 0
		c.RAM[0x135] = 1
	}
	c.course518A(0)
	c.RAM[0x135] = 0
	return c.hitEnd4EA0()
}

// hitEnd4EA0 ends the sequence once $E143 reaches $15. Ported from
// $4EA0-$4EAD.
func (c *Console) hitEnd4EA0() uint16 {
	hl := uint16(0xE143)
	if c.rd(hl)-0x15 != 0 {
		return hl
	}
	c.wr(hl, 0)
	hl--
	c.wr(hl, 0)
	c.RAM[0x137] = 0
	return hl
}

// hit4ED6 is the knocked sequence while jumping ($E142 = 3). Ported from
// $4ED6-$4F1F (High).
func (c *Console) hit4ED6() {
	hl := uint16(0xE0F9)
	a := c.rd(hl)
	c.wr(hl, a+1)
	if a == 0x0B {
		c.wr(hl, 0)
	}
	cc := c.RAM[0x144]
	c.setPenguinPatterns(cc)
	de := c.rd16(0xE078)
	e := byte(de) + c.rd(addHL(0x4C55, a))
	d := byte(de >> 8)
	if cc&0x04 == 0 {
		d = c.left3(d, 0xE0D0)
	} else {
		d = c.right3(d, 0xE0D0)
	}
	c.place4B98(uint16(d)<<8 | uint16(e))
	if c.RAM[0x0F9] != 0 {
		return
	}
	c.RAM[0x135] = 1
	hl = c.land4E81()
	c.RAM[0x135] = 0
	c.wr(hl-1, 1)
	c.startSound(0x04)
}

// crash4F20: fell into a hole. Ported from $4F20-$4F41 (High).
func (c *Console) crash4F20() {
	c.wr16(0xE140, 0x0001)
	c.RAM[0x142] = 0
	c.RAM[0x0F8] = 0xFF
	c.startSound(0x05)
	for i := uint16(0); i < 4; i++ {
		c.wr(0xE068+i*4, 0xB6)
	}
	c.fall4F42()
}

// fall4F42 shows the penguin in the hole. Ported from $4F42-$4F68.
func (c *Console) fall4F42() {
	c.setPositions4BC1(uint16(c.RAM[0x079])<<8 | 0x9F)
	c.setPenguinPatterns(0x10)
	c.RAM[0x0A0] = 0xE0
	c.wr16(0xE0A3, 0xE00A)
	c.putHole4F5A()
}

// putHole4F5A copies the hole sprites and the shadow. Ported from
// $4F5A-$4F68 (JP $4CF3).
func (c *Console) putHole4F5A() {
	c.vcopy(0xE068, 0x3B18, 0x20)
	c.vcopy(0xE0A0, 0x3B50, 0x08)
}

// crashed4F69 animates the penguin climbing out until a trigger is
// pressed. Ported from $4F69-$4FB6 (High).
func (c *Console) crashed4F69() {
	c.RAM[0x141] = (c.RAM[0x141] + 1) &^ 0x80
	if c.RAM[0x141] < 0x20 {
		c.fall4F42()
		return
	}
	if a, _ := c.newPresses(); a != 0 {
		c.recover4FB7()
		return
	}
	cc := c.RAM[0x003]
	if cc&0x07 != 0 {
		return
	}
	a, b, de := byte(0x08), byte(0x99), uint16(0x1470)
	if cc&0x08 != 0 {
		a, b, de = 0x04, 0x96, 0x1874
		if cc&0x10 != 0 {
			a, de = 0x0B, 0x1C78
		}
	}
	h := c.RAM[0x079]
	c.RAM[0x0A1] = a + h
	c.RAM[0x0A2] = byte(de)
	c.RAM[0x0A0] = b + 0x10
	c.setPositions4BC1(uint16(h)<<8 | uint16(b))
	c.setPenguinPatterns(byte(de >> 8)) // POP AF: A = D
	c.putHole4F5A()
}

// recover4FB7 puts the penguin back on the course. Ported from
// $4FB7-$4FF0 (High).
func (c *Console) recover4FB7() {
	c.RAM[0x140] = 0
	c.RAM[0x0F8] = 0
	c.wr16(0xE100, 0x0313)
	x := c.RAM[0x079]
	hl, de := c.copy4(0x66AA, 0xE068, 4)
	for b := 0; b < 4; b++ {
		n := c.rd(hl)
		hl++
		hl, de = c.copy4(hl, de, n)
	}
	c.setPositions4BC1(uint16(x)<<8 | 0x90) // POP HL: H = the pushed A
	c.wr16(0xE0A2, 0x04A0)
	c.putPenguin()
	c.putSprites()
}

// fish4FF1 collects a fish: removes the object, draws its stream from a
// RAM copy and scores 500. de is the object's +5. Ported from
// $4FF1-$5018 (High).
func (c *Console) fish4FF1(de uint16) {
	obj := de - 5
	src := c.rd16(obj+2) + 1
	c.wr(obj, 0)
	c.ldir(src, 0xE1A0, 0x0D)
	c.RAM[0x1AD] = 0
	c.startSound(0x06)
	c.nameStream4512(0xE1A0)
	c.addScore(0x0500)
}
