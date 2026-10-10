package z80dis

import "testing"

// rom builds a tiny image at $4000 from byte slices placed at addresses.
func rom(parts map[uint16][]byte) []byte {
	b := make([]byte, 0x100)
	for a, p := range parts {
		copy(b[a-0x4000:], p)
	}
	return b
}

func TestInlineTableGrowsToTight(t *testing.T) {
	// $4000: LD A,0 / CALL $4020 / DW $4009,$400A / RET / RET.
	img := rom(map[uint16][]byte{
		0x4000: {0x3E, 0x00, 0xCD, 0x20, 0x40},
		0x4005: {0x09, 0x40, 0x0A, 0x40}, // the inline table
		0x4009: {0xC9, 0xC9},             // handlers 1 and 2
		0x4020: {0x87, 0xE1, 0xE9},       // trampoline
	})
	cfg := &Config{Entries: []uint16{0x4000}, Inline: map[uint16]int{0x4020: 0}}
	a := Analyze(img, 0x4000, cfg)
	if len(a.Tables) != 1 {
		t.Fatalf("got %d tables", len(a.Tables))
	}
	tb := a.Tables[0]
	if tb.Addr != 0x4005 || tb.Count != 2 || !tb.Tight {
		t.Errorf("table %+v, want $4005 with 2 tight entries", tb)
	}
	if a.own(0x400A) != CodeStart {
		t.Errorf("handler 2 not decoded")
	}
}

func TestPushedFate(t *testing.T) {
	// Data pointer: LD HL,$4080 / PUSH HL / ... / POP HL / RET.
	data := rom(map[uint16][]byte{
		0x4000: {0x21, 0x80, 0x40, 0xE5, 0x00, 0xE1, 0xC9},
		0x4080: {0x01, 0x02},
	})
	a := Analyze(data, 0x4000, &Config{Entries: []uint16{0x4000}})
	if a.Targets[0x4080] {
		t.Errorf("popped data pointer treated as code")
	}
	// Continuation: LD HL,$4080 / PUSH HL / JP $4010; $4010: RET.
	cont := rom(map[uint16][]byte{
		0x4000: {0x21, 0x80, 0x40, 0xE5, 0xC3, 0x10, 0x40},
		0x4010: {0xC9},
		0x4080: {0xC9},
	})
	a = Analyze(cont, 0x4000, &Config{Entries: []uint16{0x4000}})
	if a.own(0x4080) != CodeStart {
		t.Errorf("pushed continuation not followed")
	}
}
