package game

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
