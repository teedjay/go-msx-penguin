package assets

import "testing"

func TestROMByte(t *testing.T) {
	// The VDP register table at $44BF starts with R0 = $02 (Screen 2).
	if got := ROMByte(0x44BF); got != 0x02 {
		t.Errorf("ROMByte($44BF) = $%02X, want $02", got)
	}
	defer func() {
		if recover() == nil {
			t.Errorf("ROMByte($4010) inside code did not panic")
		}
	}()
	ROMByte(0x4010)
}
