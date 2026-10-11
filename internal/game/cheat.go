package game

import "go-msx-penguin/assets"

// Level-select cheat. Not part of the original: an extension over the
// port, hooked at one point (state8sub3, right after newGame) and
// inactive unless StartStage is set, so every reference trace still
// matches.
//
// A new game normally reaches stage N by playing stages 1..N-1. Besides
// what stageSetup and state9 rebuild for every stage anyway, that
// carries over (High: compared with the real stage starts of a
// 41821-tick trace, see cheat_test.go):
//
//	$E0E0  stage number, BCD            +1 per clear ($431B)
//	$E0E1  course index 0-9             +1 per clear, wraps at 10
//	$E0E8  time/distance table index    same as $E0E1
//	$E0E2  wind schedule index          +1 at every wind trigger during
//	       a stage ($52A5), reset when $E0E1 wraps
//
// Not reproduced, by design: the score stays 0, and the per-course time
// left at each clear ($E0D5-$E0DE, used for a time penalty on the next
// lap) keeps its new-game value 5, as on a first lap.

// applyStartStage advances a fresh game to StartStage, then clears the
// request (it is one-shot).
func (c *Console) applyStartStage() {
	for s := 1; s < c.StartStage; s++ {
		c.skipStage()
	}
	c.StartStage = 0
}

// skipStage advances the carried-over variables as playing and clearing
// the current stage does.
func (c *Console) skipStage() {
	// The distance counts down from S00 ($E0E6 from the table at $4AAE,
	// as state9 sets it, with $E0E5 = 0) to 0000, and after every step
	// upd52A5 advances $E0E2 when $E0E6 is odd and $E0E5 = $82: once for
	// every odd hundreds value below S.
	h := c.rd(addHL(0x4AAE, c.RAM[0x0E8]*4))
	for h != 0 {
		h, _ = subDAA(h, 1)
		if h&1 != 0 {
			c.RAM[0x0E2]++
		}
	}
	// clear431B's progression (without recording the time left).
	c.RAM[0x0E0], _ = adcDAA(c.RAM[0x0E0], 1, false)
	a := c.RAM[0x0E1] + 1
	if a >= 0x0A {
		a = 0
		c.RAM[0x0E2] = 0
	}
	c.RAM[0x0E1] = a
	c.RAM[0x0E8]++
	if c.RAM[0x0E8] == 0x0A {
		c.RAM[0x0E8] = 0
	}
}

// AddTime is the extra-time cheat: it adds 100 to the BCD timer at
// $E0E3-$E0E4 (capped at 9999) and redraws it at once with the game's own
// digit routine (the original redraws it only every 64 frames). It only
// acts in live play while the timer runs: state 11, $E002 bit 6, $E133
// set, and not once time has run out ($E00C, the time-out is then already
// decided). It returns whether it acted. Call it between frames.
func (c *Console) AddTime() bool {
	if c.RAM[0x000] != 11 || c.RAM[0x002]&0x40 == 0 || c.RAM[0x133] == 0 || c.RAM[0x00C] != 0 {
		return false
	}
	hi, carry := adcDAA(c.RAM[0x0E4], 1, false)
	if carry {
		c.RAM[0x0E3], hi = 0x99, 0x99
	}
	c.RAM[0x0E4] = hi
	c.printTime()
	return true
}

// Destination returns the name of the station stage n runs to (n from
// 1; levels repeat every 10), decoded from the game's own text table at
// $5589 that $552F prints at the stage end. That table is indexed by the
// course index after the clear has advanced it, so stage n shows entry
// n mod 10 (checked on the original's screens: stage 1 ends at
// AUSTRALIA, stage 5 at the South Pole). Font bytes are ASCII - $20,
// except a few tiles drawn specially.
func Destination(n int) string {
	special := map[byte]string{
		0x0F: " ", 0xC9: "F", 0xCA: "W", 0xCB: "Z", // letter tiles
		0xCE: "SOUTH POLE", 0xCF: "", 0xD0: "", 0xD1: "", // the kanji 南極点
	}
	rd := assets.ROMByte
	p := uint16(rd(0x5589+uint16(2*(n%10)))) | uint16(rd(0x558A+uint16(2*(n%10))))<<8
	var s []byte
	for a := p + 2; rd(a) != 0xFF; a++ { // skip the VRAM address word
		b := rd(a)
		if v, ok := special[b]; ok {
			s = append(s, v...)
		} else if b != 0x20 { // $20: the dash either side of the name
			s = append(s, b+0x20)
		}
	}
	return string(s)
}
