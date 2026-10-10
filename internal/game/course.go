package game

// The course: edges, objects, wind, the obstacle sprites, stage end.

// course514F steps the course animation by the speed $E100 (period) and
// the counter $E101. Ported from $514F-$517E (High).
func (c *Console) course514F() {
	cc := c.RAM[0x100]
	v := c.RAM[0x101] - 1
	c.RAM[0x101] = v
	switch {
	case v == 0:
		c.RAM[0x101] = cc
		c.edge5169()
	case v == 3:
		c.course50F3()
	case v == 1:
		c.edge5160()
	case v == 2:
		c.course518A(0)
	default:
		if c.RAM[0x101] != cc>>1 {
			return
		}
		c.course518A(1)
	}
}

// course518A animates the active objects: each object's type counts
// 1..$0F and its name-table stream pointer (+2/+3) walks on through the
// ROM. With b != 0 only types >= $0B move. Then the obstacle and
// collision updates. Ported from $518A-$51D6 (High).
func (c *Console) course518A(b byte) {
	hl := uint16(0xE112)
	n := 4
	if c.RAM[0x0E0] >= 5 {
		n = 5
	}
	for ; n > 0; n-- {
		obj := hl
		hl = obj + 6
		if b != 0 && c.rd(obj) < 0x0B {
			continue
		}
		a := c.rd(obj)
		if a == 0 {
			continue
		}
		a++
		c.wr(obj, a)
		if a >= 0x10 {
			c.wr(obj, 0)
		}
		end := c.nameStream4512(c.rd16(obj + 2))
		c.wr16(obj+2, end+1)
	}
	c.upd75A9()
	c.upd77FC()
	c.collide4D1E()
	c.collide4D6B()
}

// course50F3: wind scroll, then on the $E107 signal the next course
// section. Ported from $50F3-$5118 (High).
func (c *Console) course50F3() {
	c.wind5301()
	if c.RAM[0x107]-1 != 0 || c.RAM[0x102]-1 != 0 {
		return
	}
	c.RAM[0x107] = 0
	if c.next5096() != 0 {
		return
	}
	hl := c.rd16(0xE103)
	if c.RAM[0x109]&1 != 0 {
		hl = c.rd16(0xE105)
	}
	c.edgeDraw(hl, 0)
}

// next5096 starts the next course section from the list at $E10A,
// returning $FF at its end. Ported from $5096-$50C5 (High).
func (c *Console) next5096() byte {
	a := c.RAM[0x108]
	c.RAM[0x108]++
	a = c.rd(addHL(c.rd16(0xE10A), a))
	if a == 0xFF {
		return 0xFF
	}
	c.RAM[0x109] = a
	bc := uint16(0xE103)
	if a&1 != 0 {
		bc += 2
	}
	hl := addHL(0x71FB, a*2)
	e := c.rd(hl)
	c.wr(bc, e)
	d := c.rd(hl + 1)
	c.wr(bc+1, d)
	c.horizon50C6(addHL(uint16(d)<<8|uint16(e), 8))
	return 0
}

// spawn51D7 places a new object every $E10E ticks once the distance is
// past $86, from the course table at $E18B. Ported from $51D7-$5261
// (High).
func (c *Console) spawn51D7() {
	c.holdStart()
	if c.RAM[0x0E6] == 0 && c.RAM[0x0E5] < 0x86 {
		return
	}
	a := c.RAM[0x10E]
	c.RAM[0x10F]--
	if c.RAM[0x10F] != 0 {
		return
	}
	c.RAM[0x10F] = a
	hl := uint16(0xE112)
	n := 3
	if c.RAM[0x0E0] >= 5 {
		n = 4
	}
	for c.rd(hl) != 0 {
		hl += 6
		n--
		if n == 0 {
			return
		}
	}
	c.wr(hl, c.rd(hl)+1)
	de := hl + 1
	c.RAM[0x111] = (c.RAM[0x111] + 1) &^ 0x08
	cc := c.rd(addHL(c.rd16(0xE18B), c.RAM[0x111]))
	if c.holdCount() {
		cc = 3
	}
	if cc == 0xFF {
		c.wr(de-1, 0)
		return
	}
	if cc&0x10 != 0 {
		c.RAM[0x190] = 1
		cc &= 0x03
		c.RAM[0x191] = cc
	} else if cc != 0 && c.RAM[0x0FC] != 0 {
		cc++
	}
	c.objInit5262(de, cc)
	if c.RAM[0x190]&1 == 0 {
		return
	}
	cc = ^c.RAM[0x191] & 0x03
	if c.RAM[0x12A] == 0 {
		c.RAM[0x12A]++
		c.objInit5262(0xE12B, cc)
	}
	c.RAM[0x190] = 0
}

// objInit5262 sets an object's kind (+1) and copies the 6-byte record
// at $527B + 6*kind: stream pointer to +2/+3, address of the rest to
// +4/+5. Ported from $5262-$527A (High).
func (c *Console) objInit5262(hl uint16, cc byte) {
	c.wr(hl, cc)
	hl++
	de := addDE(0x527B, cc*6) // 8-bit 6*C
	c.wr(hl, c.rd(de))
	c.wr(hl+1, c.rd(de+1))
	de += 2
	c.wr16(hl+2, de)
}

// upd52A5 starts a wind at certain distances. Ported from $52A5-$52EE
// (High).
func (c *Console) upd52A5() {
	if c.RAM[0x0E6]&1 == 0 || c.RAM[0x0E5] != 0x82 {
		return
	}
	a := c.RAM[0x0E2]
	c.RAM[0x0E2]++
	carry := a&1 != 0 // SRL A
	a >>= 1
	v := c.rd(addHL(0x5391, a))
	if !carry {
		// RRA x4 through the carry (clear here): a 9-bit rotate, of
		// which only the low nibble is used.
		cy := false
		for i := 0; i < 4; i++ {
			v, cy = rra(v, cy)
		}
	}
	cc := v
	a = v & 0x03
	if a == 3 {
		return
	}
	if cc&0x08 != 0 {
		a |= 0x02
	}
	c.RAM[0x194] = a
	if cc&0x04 != 0 {
		c.RAM[0x195] = 2
	}
	c.RAM[0x196] = 1
	c.RAM[0x197] = 0
	c.RAM[0x198] = c.RAM[0x100] >> 2
	c.horizonPart(0x722F) // $5476
	c.stageText52EF()
}

// wind5301 scrolls the horizon row while a wind blows, reverses or ends
// it, and then pushes the penguin. Ported from $5301-$5365, falling
// through into $5366 (High).
func (c *Console) wind5301() {
	if c.RAM[0x196] == 0 {
		return
	}
	if c.RAM[0x194]&1 == 0 {
		a := c.RAM[0x150]
		c.ldir(0xE151, 0xE150, 0x1F)
		c.RAM[0x16F] = a
	} else {
		a := c.RAM[0x16F]
		for i := uint16(0); i < 0x1F; i++ { // LDDR
			c.wr(0xE16F-i, c.rd(0xE16E-i))
		}
		c.RAM[0x150] = a
	}
	c.printHorizon()
	c.RAM[0x197]++
	if c.RAM[0x197]&0x0F == 0 && c.RAM[0x195] != 0 {
		c.RAM[0x195]--
		if c.RAM[0x195] == 0 {
			c.RAM[0x194] ^= 1
			c.stageText52EF()
		}
	}
	if c.RAM[0x0E6]&1 != 0 || c.RAM[0x0E5] >= 0x45 {
		return
	}
	if c.RAM[0x197]&0x0F != 0 {
		return
	}
	c.RAM[0x196] = 0
	c.stageText52F2(0x53BA)
	c.horizonPart(0x72A1) // $5471
	c.wind5366()
}

// wind5366 pushes the penguin sideways every $E198 ticks while a wind
// blows. Ported from $5366-$5390 (High).
func (c *Console) wind5366() {
	if c.RAM[0x196] == 0 {
		return
	}
	c.RAM[0x198]--
	if c.RAM[0x198] != 0 {
		return
	}
	c.RAM[0x198] = c.RAM[0x100] >> 2
	de := c.rd16(0xE078)
	d := byte(de >> 8)
	if c.RAM[0x194]&1 == 0 {
		d = c.left4C71(d, 0xE0D1)
	} else {
		d = c.right4C8E(d, 0xE0D1)
	}
	c.place4B98(uint16(d)<<8 | de&0xFF)
}

// walk5489 moves the penguin one step (every 4th tick) at the stage end
// and returns true (Z) after the 16th. With cc = $FF it computes X from
// $E138/$E139; otherwise X = h. Ported from $5489-$54CA (High).
func (c *Console) walk5489(cc, h byte) bool {
	if c.RAM[0x003]&0x03 != 0 {
		return false
	}
	cc++
	if cc == 0 {
		x := c.RAM[0x139]
		hl := uint16(0x70) - uint16(x)
		de := hl
		b := c.RAM[0x138]
		for {
			hl += de
			b--
			if b == 0 {
				break
			}
		}
		h = rlca(rlca(rlca(rlca(byte(hl>>8)))))&0xF0 | rrca(rrca(rrca(rrca(byte(hl)))))&0x0F
		h += x
	}
	c.setPositions4BC1(uint16(h)<<8 | uint16(c.RAM[0x078]-1))
	c.penguinFrame()
	c.RAM[0x138]++
	return c.RAM[0x138] == 0x10
}

// flag54CB / flag54CF draw the station flag, alternating two streams.
// Ported from $54CB-$54E0.
func (c *Console) flag54CB() {
	c.RAM[0x13A] = 0
	c.flag54CF()
}

func (c *Console) flag54CF() {
	a := c.RAM[0x13A]
	c.RAM[0x13A]++
	hl := uint16(0x54FC)
	if a&1 != 0 {
		hl = 0x5510
	}
	c.nameStream4512(hl)
}

// station54E1 draws the bonus scene. Ported from $54E1-$54FB.
func (c *Console) station54E1() {
	c.rleDest(0x6B3B)
	c.ldir(0x6700, 0xE06C, 0x10)
	c.putSprites()
	c.nameStream4512(0x551A)
}

// station552F draws the station and its name. Ported from $552F-$556A.
func (c *Console) station552F() {
	c.rleAt(0x660A, 0x5100, false)
	c.printText(c.rd16(addHL(0x5589, c.RAM[0x0E1]*2)))
	hl := c.rd16(addHL(0x5602, (c.RAM[0x0E0]&0x0F)*2))
	hl = c.rleAt(hl, 0x5F40, false)
	c.RAM[0x063] = c.rd(hl)
	c.RAM[0x067] = c.rd(hl + 1)
	c.putStation557C()
}

// station556B raises the flag sprites 2 pixels per call until Y = $36.
// Ported from $556B-$557B.
func (c *Console) station556B() {
	a := c.RAM[0x060] - 2
	if a == 0x36 {
		return
	}
	c.RAM[0x060], c.RAM[0x064], c.RAM[0x068] = a, a, a
	c.putStation557C()
}

// putStation557C copies three sprites from $E060 to $3B10. Ported from
// $557C-$5588.
func (c *Console) putStation557C() { c.vcopy(0xE060, 0x3B10, 0x0C) }

// upd74D3 draws the scenery picked by the distance every 32 units while
// the high byte is 0. Ported from $74D3-$7518 (High).
func (c *Console) upd74D3() {
	l := c.RAM[0x0E5]
	if c.RAM[0x0E6] != 0 || l&0x1F != 0 {
		return
	}
	a := rlca(rlca(rlca(l))) * 2
	c.nameStream(c.rd16(addHL(0x7519, a)), 0x40, false)
}

// upd75A9 launches the moving obstacle from an object of type 7 with
// kind < 3. Ported from $75A9-$7624 (High for the flow).
func (c *Console) upd75A9() {
	const hl = 0xE183
	if c.rd(hl)&0xE3 != 0 {
		return
	}
	de := uint16(0xE113)
	for b := 3; b > 0; b-- {
		if c.rd(de) < 3 {
			if c.rd(de-1) == 7 {
				c.launch75C9(de - 1)
				return
			}
		}
		de += 6
	}
}

// launch75C9: de = the object. Ported from $75C9-$7624.
func (c *Console) launch75C9(de uint16) {
	const hl = 0xE183
	c.wr16(0xE181, de)
	de++
	if c.RAM[0x003] >= c.RAM[0x18A] {
		c.RAM[0x192] = 0
		switch c.rd(de) {
		case 0:
			c.wr(hl, c.rd(hl)|0x40)
		case 1:
			c.wr(hl, c.rd(hl)|0x80)
		default:
			c.wr(hl, c.rd(hl)|0x20)
		}
		return
	}
	var z bool
	if a := c.RAM[0x009] & 0x0C; a != 0 {
		z = a&0x04 == 0
	} else {
		c.RAM[0x185]++
		z = c.RAM[0x185]&1 == 0
	}
	c.wr(hl, c.rd(hl)|0x01)
	a := byte(0x90)
	if !z {
		a = 0x80
		c.wr(hl, rlca(c.rd(hl))) // RLC (HL)
	}
	var b byte
	switch c.rd(de) {
	case 0:
		b = 0x7A
	case 1:
		b = 0x64
	default:
		b = 0x92
	}
	c.RAM[0x08C], c.RAM[0x08D], c.RAM[0x08E] = 0x66, b, a
}

// upd7625 runs the obstacle every other tick. Ported from $7625-$7629.
func (c *Console) upd7625() {
	if c.RAM[0x003]&1 != 0 {
		return
	}
	c.upd762A()
}

// upd762A shows and moves the obstacle sprite $E08C (Y, X, pattern
// colour). Ported from $762A-$767E (High).
func (c *Console) upd762A() {
	c.wr16(0xE188, c.rd16(0xE08C))
	c.vcopy(0xE08C, 0x3B3C, 4)
	if c.RAM[0x183]&0x03 == 0 {
		return
	}
	c.blink7687(0xE08E)
	if c.RAM[0x183]&1 != 0 {
		c.RAM[0x08D] += 2
	} else {
		c.RAM[0x08D] -= 2
	}
	c.RAM[0x184]++
	a := c.RAM[0x184]
	switch {
	case a < 0x08:
		c.RAM[0x08C] -= 2
	case a < 0x10:
	case a == 0x10:
		c.RAM[0x08E] += 8
	case a >= 0x22:
		c.objGone767F(0xE08C, 0xE183)
	default:
		cc := byte(5)
		if a >= 0x1A {
			cc = 7
		}
		c.RAM[0x08C] += cc
	}
}

// objGone767F hides the obstacle and stops it. Ported from $767F-$7686.
func (c *Console) objGone767F(hl, de uint16) {
	c.wr(hl, 0xE0)
	c.wr(de, 0)
	c.wr(de+1, 0)
	c.upd762A()
}

// blink7687 toggles bit 2 of (hl) (and clears bits 0-1) every 16 ticks:
// SRL x3, CCF, RLA x3. Ported from $7687-$7699 (High).
func (c *Console) blink7687(hl uint16) {
	if c.RAM[0x003]&0x0F != 0 {
		return
	}
	v := c.rd(hl)
	cy := v&0x04 != 0 // the last bit SRL shifts out
	v >>= 3
	cy = !cy
	for i := 0; i < 3; i++ {
		v, cy = rla(v, cy)
	}
	c.wr(hl, v)
}

// upd77FC places the big obstacle sprites from the tables at $787B for
// the object $E181 points at. Ported from $77FC-$787A (High for the
// flow).
func (c *Console) upd77FC() {
	if c.RAM[0x183]&0xE0 == 0 {
		return
	}
	a := c.rd(c.rd16(0xE181)) - 0x0F
	var hl uint16
	b := 4
	if a == 0 {
		c.RAM[0x183] = 0
		hl = 0x7977
	} else {
		a += 8
		hl = c.rd16(addHL(0x787B, a*2))
		if a >= 6 && c.RAM[0x137]&1 == 0 {
			c.RAM[0x192] = 1
		}
		d := byte(0x0C)
		if a < 3 {
			d, b = 6, 2
		}
		switch e := c.RAM[0x183]; {
		case e == 0x40:
		case e < 0x40:
			hl = addHL(hl, d)
		default:
			hl = addHL(addHL(hl, d), d)
		}
	}
	de := uint16(0xE090)
	for ; b > 0; b-- {
		for i := 0; i < 3; i++ {
			c.wr(de, c.rd(hl))
			hl++
			de++
		}
		de++
	}
	src := uint16(0xE090)
	if c.RAM[0x192]&1 != 0 {
		c.vcopy(src, 0x3B00, 0x10)
		src = 0xE050
	}
	c.vcopy(src, 0x3B40, 0x10)
}

// upd775F animates the four fish sprites at $E0B8 in live play. Ported
// from $775F-$77EF (High for the flow).
func (c *Console) upd775F() {
	if c.RAM[0x002]&0x40 == 0 {
		return
	}
	de := uint16(0xE0B8)
	hl := uint16(0xE14A)
	for b := byte(4); b > 0; b-- {
		if c.rd(hl) != 0 {
			de += 4
			hl++
			continue
		}
		c.wr(hl, c.rd(hl)+1)
		t := addHL(0x77F2, b*2)
		c.wr(de, c.rd(t))
		c.wr(de+1, c.rd(t+1))
		c.wr(de+2, 0xE0)
		c.wr(de+3, 0x0F)
		de += 4
		hl++
	}
	c.RAM[0x149]--
	if c.RAM[0x149] != 0 {
		return
	}
	c.RAM[0x149] = c.RAM[0x148]
	hl = 0xE14A
	de = 0xE0B8
	for b := byte(0); b != 4; b++ {
		if c.rd(hl) != 0 {
			if c.rd(de) == 0x08 {
				c.wr(de, 0xD1)
				c.wr(hl, 0)
			} else {
				c.wr(hl, c.rd(hl)+1)
				c.wr(de, c.rd(de)-1)
				x := de + 1
				c.wr(x, c.rd(addDE(0x77F0, b))+c.rd(x))
				switch v := c.rd(hl); v {
				case 0x0C:
					c.wr(de+2, 0xDC)
				case 0x18:
					c.wr(de+2, 0xD8)
				}
			}
		}
		de += 4
		hl++
	}
	c.vcopy(0xE0B8, 0x3B68, 0x10)
}

func rra(a byte, cy bool) (byte, bool) {
	out := a&1 != 0
	a >>= 1
	if cy {
		a |= 0x80
	}
	return a, out
}

func rla(a byte, cy bool) (byte, bool) {
	out := a&0x80 != 0
	a <<= 1
	if cy {
		a |= 1
	}
	return a, out
}
