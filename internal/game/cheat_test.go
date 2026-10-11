package game

import "testing"

// stageStarts are the carried-over variables the real game had on
// entering state 9 for stages 1-7, read from a 41821-tick reference
// trace of the original playing through to stage 7 (cmd/keysearch
// schedule): $E0E0, $E0E1, $E0E2, $E0E8.
var stageStarts = [][4]byte{
	{0x01, 0, 0, 0},
	{0x02, 1, 6, 1},
	{0x03, 2, 13, 2},
	{0x04, 3, 19, 3},
	{0x05, 4, 26, 4},
	{0x06, 5, 34, 5},
	{0x07, 6, 39, 6},
}

func TestSkipStageMatchesRealStageStarts(t *testing.T) {
	for n, want := range stageStarts {
		c := New()
		c.newGame()
		c.StartStage = n + 1
		c.applyStartStage()
		got := [4]byte{c.RAM[0x0E0], c.RAM[0x0E1], c.RAM[0x0E2], c.RAM[0x0E8]}
		if got != want {
			t.Errorf("stage %d: got %v, want %v", n+1, got, want)
		}
		if c.StartStage != 0 {
			t.Errorf("stage %d: request not cleared", n+1)
		}
	}
}

// TestCheatStartsAtStage starts real games through the title for every
// selectable stage (A-Z), holding up and jumping now and then, and
// checks the stage shown and that play runs without leaving the ROM data
// the manifest provides.
func TestCheatStartsAtStage(t *testing.T) {
	for stage := 1; stage <= 26; stage++ {
		c := New()
		c.Boot(2)
		for i := 0; i < 250; i++ {
			c.Interrupt()
		}
		c.StartStage = stage
		c.Keys[0] &^= 0x04 // 2: keyboard play
		for i := 0; i < 3; i++ {
			c.Interrupt()
		}
		c.Keys[0] |= 0x04
		c.Keys[8] &^= 0x20 // hold up (accelerate)
		for i := 0; c.RAM[0x000] != 11 && i < 3000; i++ {
			c.Interrupt()
		}
		if c.RAM[0x000] != 11 {
			t.Fatalf("stage %d: play did not start (state %d)", stage, c.RAM[0x000])
		}
		bcd := byte(stage/10)<<4 | byte(stage%10)
		if c.RAM[0x0E0] != bcd || c.RAM[0x0E1] != byte((stage-1)%10) {
			t.Errorf("stage %d: $E0E0=%02X $E0E1=%d", stage, c.RAM[0x0E0], c.RAM[0x0E1])
		}
		for i := 0; i < 2000; i++ {
			if i%40 == 0 {
				c.Keys[8] &^= 0x01 // jump
			} else if i%40 == 3 {
				c.Keys[8] |= 0x01
			}
			c.Interrupt()
		}
	}
}

func TestAddTime(t *testing.T) {
	c := New()
	c.Boot(2)
	for i := 0; i < 250; i++ {
		c.Interrupt()
	}
	if c.AddTime() {
		t.Fatal("AddTime acted on the title screen")
	}
	c.Keys[0] &^= 0x04 // 2: keyboard play
	for i := 0; i < 3; i++ {
		c.Interrupt()
	}
	c.Keys[0] |= 0x04
	for i := 0; c.RAM[0x000] != 11 && i < 3000; i++ {
		c.Interrupt()
	}
	for i := 0; i < 100; i++ {
		c.Interrupt()
	}
	lo, hi := c.RAM[0x0E3], c.RAM[0x0E4]
	if !c.AddTime() {
		t.Fatal("AddTime did not act in play")
	}
	if wantHi, _ := adcDAA(hi, 1, false); c.RAM[0x0E3] != lo || c.RAM[0x0E4] != wantHi {
		t.Errorf("time %02X%02X -> %02X%02X, want +100", hi, lo, c.RAM[0x0E4], c.RAM[0x0E3])
	}
	// The display at $3827: four digit tiles $10-$19, high byte first.
	want := []byte{c.RAM[0x0E4]>>4 | 0x10, c.RAM[0x0E4]&0x0F | 0x10, lo>>4 | 0x10, lo&0x0F | 0x10}
	for i, w := range want {
		if c.VRAM[0x3827+i] != w {
			t.Errorf("time digit %d: tile $%02X, want $%02X", i, c.VRAM[0x3827+i], w)
		}
	}
	c.RAM[0x0E4], c.RAM[0x0E3] = 0x99, 0x50
	c.AddTime()
	if c.RAM[0x0E4] != 0x99 || c.RAM[0x0E3] != 0x99 {
		t.Errorf("cap: got %02X%02X, want 9999", c.RAM[0x0E4], c.RAM[0x0E3])
	}
	// Play keeps running with the extra time.
	for i := 0; i < 500; i++ {
		c.Interrupt()
	}
	if c.RAM[0x000] != 11 {
		t.Errorf("state %d after adding time", c.RAM[0x000])
	}
}

func TestDestination(t *testing.T) {
	want := []string{"AUSTRALIA", "AUSTRALIA", "FRANCE", "NEW ZEALAND", "SOUTH POLE",
		"USA", "USA", "ARGENTINA", "UNITED KINGDOM", "JAPAN"}
	for i, w := range want {
		if got := Destination(i + 1); got != w {
			t.Errorf("stage %d: %q, want %q", i+1, got, w)
		}
	}
	if Destination(11) != "AUSTRALIA" {
		t.Errorf("stage 11: %q", Destination(11))
	}
}
