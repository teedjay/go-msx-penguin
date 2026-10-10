package game

// Sound driver. Its RAM is game state (compared every tick), so it is
// ported faithfully as PSG register writes; a chip model for output comes
// later.
//
// Three channel records of 10 bytes at $E010, $E01A, $E024 (High for the
// layout, Medium for the field roles):
//
//	+0 note counter     +1 note length    +2 sound id (0 = idle; bit 7 = music)
//	+3/+4 data pointer  +5 octave shift   +6 music volume
//	+7 current volume   +8 decay counter  +9 loop counter
//
// Channel n uses PSG tone registers 2n/2n+1 (C = 2n+1 holds the coarse
// register) and volume register 8+n.

const sndChan0 = 0xE010

// startSound requests sound id a. Ported from $7983-$7990 (which only
// wraps $7991 in DI/EI and register saves).
func (c *Console) startSound(a byte) { c.sound7991(a) }

// sound7991 starts sound a on its channel group, unless a higher id is
// playing there. Ids below $8A use channel 2 only, $8A-$8B channels 0-1,
// $8C and up all three. Ported from $7991-$79CB (High).
func (c *Console) sound7991(a byte) {
	b := 2
	hl := uint16(0xE012)
	switch {
	case a < 0x8A:
		b = 1
		hl = 0xE026
	case a < 0x8C:
	default:
		b = 3
	}
	if a < c.rd(hl) {
		return
	}
	id := a
	de := addDE(0x7B0C, (a&0x3F)*2) // ADD A,A wraps at 8 bits
	for ; b > 0; b-- {
		c.wr(hl-2, 1)
		c.wr(hl-1, 1)
		c.wr(hl, id)
		c.wr(hl+1, c.rd(de))
		de++
		c.wr(hl+2, c.rd(de))
		de++
		hl += 10
	}
}

// soundDriver runs once per interrupt. Ported from $79EA-$7A13 (High).
func (c *Console) soundDriver() {
	c.wrtpsg(7, c.rdpsg(7)&0xB8) // tones on, keep the noise bits
	ch := byte(1)                // C: the channel's coarse tone register
	ix := uint16(sndChan0)
	for n := 0; n < 3; n++ {
		if a := c.rd(ix + 2); a != 0 {
			c.sndChannel(ix, ch, a)
		}
		ch += 2
		ix += 10
	}
}

// sndChannel advances one channel. a is its sound id; the JP M at $7A14
// tests the sign flag set by the OR A at $7A07 before the CALL NZ.
// Ported from $7A14-$7A1A.
func (c *Console) sndChannel(ix uint16, ch, a byte) {
	if a&0x80 != 0 {
		c.sndMusicTick(ix, ch)
		return
	}
	if c.dec(ix) != 0 {
		return
	}
	c.sndNext(ix, ch)
}

// dec decrements a RAM byte and returns the result, like DEC (IX+d).
func (c *Console) dec(a uint16) byte {
	v := c.rd(a) - 1
	c.wr(a, v)
	return v
}

// sndNext reads the channel's next data item. Ported from $7A1B-$7A62.
func (c *Console) sndNext(ix uint16, ch byte) {
	hl := c.rd16(ix + 3)
	a := c.rd(hl)
	switch {
	case a == 0xFE:
		c.sndLoop(ix, ch, hl)
		return
	case a > 0xFE:
		c.sndStop(ix, ch)
		return
	}
	if c.rd(ix+2)&0x80 != 0 {
		c.sndMusicNote(ix, ch, hl, a)
		return
	}
	// Effect data: optional $2n sets the note length, then a volume
	// nibble + tone high nibble byte and a tone low byte.
	if a&0xF0 == 0x20 {
		c.wr(ix+1, c.rd(hl)&0x0F)
		hl++
	}
	a = c.rd(hl)
	b := a & 0xF0
	d := a ^ b
	hl++
	e := c.rd(hl)
	hl++
	c.wr16(ix+3, hl)
	c.sndTone(ch, uint16(d)<<8|uint16(e))
	c.sndStartNote(ix, ch, rrca(rrca(rrca(rrca(b))))&0x0F)
}

// sndStartNote sets the note counters and writes volume h. Ported from
// $7A56-$7A62 and $7A8C-$7A91.
func (c *Console) sndStartNote(ix uint16, ch, h byte) {
	a := c.rd(ix + 1)
	c.wr(ix, a)
	c.wr(ix+8, a+3)
	c.sndVolume(ch, h)
}

// sndVolume writes volume h to the channel's volume register. RRCA of
// 1, 3, 5 gives $80, $81, $82, and adding $88 wraps to registers 8-10.
// Ported from $7A8C-$7A91.
func (c *Console) sndVolume(ch, h byte) { c.wrtpsg(rrca(ch)+0x88, h) }

// sndTone writes a 12-bit tone period. Ported from $7AE6-$7AF0.
func (c *Console) sndTone(ch byte, hl uint16) {
	c.wrtpsg(ch, byte(hl>>8))
	c.wrtpsg(ch-1, byte(hl))
}

// sndStop silences and frees the channel. Ported from $7A64-$7A69.
func (c *Console) sndStop(ix uint16, ch byte) {
	c.wr(ix+2, 0)
	c.sndVolume(ch, 0)
}

// sndLoop handles $FE n: repeat the sound n times ($FF: forever) by
// restarting its whole channel group through $7991. Ported from
// $79CC-$79E9 (High).
func (c *Console) sndLoop(ix uint16, ch byte, hl uint16) {
	hl++
	a := c.rd(hl) + 1
	if a != 0 {
		v := c.rd(ix+9) + 1
		c.wr(ix+9, v)
		if a-1 == v {
			c.wr(ix+9, 0)
			c.sndStop(ix, ch)
			return
		}
	}
	c.sound7991(c.rd(ix + 2))
}

// sndMusicTick runs a music channel's note counter and volume decay.
// Ported from $7A6B-$7A8B (High for the control flow).
func (c *Console) sndMusicTick(ix uint16, ch byte) {
	if c.dec(ix) == 0 {
		c.sndNext(ix, ch)
		return
	}
	a := c.dec(ix + 8)
	if a != c.rd(ix) {
		c.dec(ix + 8)
	} else if a >= 1 {
		return
	}
	v := c.rd(ix+7) - 1
	if v&0x80 != 0 { // RET M
		return
	}
	c.wr(ix+7, v)
	c.sndVolume(ch, v)
}

// sndMusicNote plays a music note byte a at hl: high nibble indexes the
// length table at $7AFE, low nibble is the note (12 = rest). $FD o sets
// octave and volume first. Ported from $7A94-$7AEE (High).
func (c *Console) sndMusicNote(ix uint16, ch byte, hl uint16, a byte) {
	if a == 0xFD {
		hl++
		a = c.rd(hl)
		c.wr(ix+5, a&0x07)
		c.wr(ix+6, (a&0xF8)>>3) // XOR (HL) then RRCA x3 of a value with low bits clear
		hl++
		a = c.rd(hl)
	}
	b := a & 0x0F
	a ^= b // the high nibble
	hl++
	c.wr16(ix+3, hl)
	c.wr(ix+1, c.rd(addHL(0x7AFE, a>>4)))
	v := b - 0x0C
	c.wr(ix+7, v)
	if v != 0 {
		v = c.rd(ix + 6)
		c.wr(ix+7, v)
	}
	c.sndStartNote(ix, ch, v)
	// The rest note (12) reads $7AFE, the start of the length table, as
	// its period; harmless at volume 0 and kept as is.
	period := uint16(c.rd(addHL(0x7AF2, b)))
	for n := c.rd(ix + 5); n > 0; n-- {
		period += period // ADD HL,HL
	}
	c.sndTone(ch, period)
}
