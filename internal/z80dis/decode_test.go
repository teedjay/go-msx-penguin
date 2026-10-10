package z80dis

import "testing"

func TestDecode(t *testing.T) {
	cases := []struct {
		bytes []byte
		want  string
		flow  Flow
	}{
		{[]byte{0x00}, "NOP", FlowNone},
		{[]byte{0x08}, "EX AF,AF'", FlowNone},
		{[]byte{0x10, 0xFE}, "DJNZ $1000", FlowCondJump},
		{[]byte{0x18, 0xFE}, "JR $1000", FlowJump},
		{[]byte{0x20, 0x03}, "JR NZ,$1005", FlowCondJump},
		{[]byte{0x21, 0x45, 0x40}, "LD HL,$4045", FlowNone},
		{[]byte{0x22, 0x9B, 0xFD}, "LD ($FD9B),HL", FlowNone},
		{[]byte{0x2A, 0x00, 0xE0}, "LD HL,($E000)", FlowNone},
		{[]byte{0x32, 0x9A, 0xFD}, "LD ($FD9A),A", FlowNone},
		{[]byte{0x36, 0x00}, "LD (HL),$00", FlowNone},
		{[]byte{0x76}, "HALT", FlowNone},
		{[]byte{0x7E}, "LD A,(HL)", FlowNone},
		{[]byte{0x86}, "ADD A,(HL)", FlowNone},
		{[]byte{0xB7}, "OR A", FlowNone},
		{[]byte{0xC0}, "RET NZ", FlowCondRet},
		{[]byte{0xC3, 0x34, 0x12}, "JP $1234", FlowJump},
		{[]byte{0xC5}, "PUSH BC", FlowNone},
		{[]byte{0xC9}, "RET", FlowRet},
		{[]byte{0xCD, 0x47, 0x00}, "CALL $0047", FlowCall},
		{[]byte{0xD3, 0x98}, "OUT ($98),A", FlowNone},
		{[]byte{0xDC, 0x34, 0x12}, "CALL C,$1234", FlowCondCall},
		{[]byte{0xE9}, "JP (HL)", FlowIndirect},
		{[]byte{0xEB}, "EX DE,HL", FlowNone},
		{[]byte{0xE3}, "EX (SP),HL", FlowNone},
		{[]byte{0xEF}, "RST $0028", FlowRST},
		{[]byte{0xFE, 0x0C}, "CP $0C", FlowNone},
		{[]byte{0xCB, 0x7F}, "BIT 7,A", FlowNone},
		{[]byte{0xCB, 0x46}, "BIT 0,(HL)", FlowNone},
		{[]byte{0xCB, 0x1E}, "RR (HL)", FlowNone},
		{[]byte{0xED, 0x56}, "IM 1", FlowNone},
		{[]byte{0xED, 0xB0}, "LDIR", FlowNone},
		{[]byte{0xED, 0xA3}, "OUTI", FlowNone},
		{[]byte{0xED, 0x79}, "OUT (C),A", FlowNone},
		{[]byte{0xED, 0x52}, "SBC HL,DE", FlowNone},
		{[]byte{0xED, 0x4B, 0x00, 0xE0}, "LD BC,($E000)", FlowNone},
		{[]byte{0xED, 0x5F}, "LD A,R", FlowNone},
		{[]byte{0xED, 0x4D}, "RETI", FlowRet},
		{[]byte{0xDD, 0x21, 0x00, 0xE1}, "LD IX,$E100", FlowNone},
		{[]byte{0xDD, 0x7E, 0x05}, "LD A,(IX+$05)", FlowNone},
		{[]byte{0xFD, 0x77, 0xFF}, "LD (IY-$01),A", FlowNone},
		{[]byte{0xDD, 0x66, 0x02}, "LD H,(IX+$02)", FlowNone},
		{[]byte{0xDD, 0x36, 0x03, 0x7F}, "LD (IX+$03),$7F", FlowNone},
		{[]byte{0xDD, 0x34, 0x01}, "INC (IX+$01)", FlowNone},
		{[]byte{0xDD, 0x19}, "ADD IX,DE", FlowNone},
		{[]byte{0xDD, 0x29}, "ADD IX,IX", FlowNone},
		{[]byte{0xDD, 0xE9}, "JP (IX)", FlowIndirect},
		{[]byte{0xDD, 0xE5}, "PUSH IX", FlowNone},
		{[]byte{0xDD, 0x7C}, "LD A,IXH", FlowNone},
		{[]byte{0xDD, 0xEB}, "EX DE,HL", FlowNone},
		{[]byte{0xDD, 0xCB, 0x00, 0x1E}, "RR (IX+$00)", FlowNone},
		{[]byte{0xFD, 0xCB, 0x02, 0x46}, "BIT 0,(IY+$02)", FlowNone},
		{[]byte{0xDD, 0xCB, 0xFE, 0xC6}, "SET 0,(IX-$02)", FlowNone},
	}
	for _, c := range cases {
		mem := func(a uint16) byte {
			i := int(a) - 0x1000
			if i >= 0 && i < len(c.bytes) {
				return c.bytes[i]
			}
			return 0
		}
		in := Decode(mem, 0x1000)
		got := in.Format(nil)
		if got != c.want || in.Len != len(c.bytes) || in.Flow != c.flow {
			t.Errorf("% X: got %q len %d flow %d, want %q len %d flow %d",
				c.bytes, got, in.Len, in.Flow, c.want, len(c.bytes), c.flow)
		}
	}
}

// TestDecodeAllLengths decodes every opcode with every prefix and checks
// the length is sane, which catches a branch that forgot to consume an
// operand.
func TestDecodeAllLengths(t *testing.T) {
	for _, pre := range [][]byte{nil, {0xCB}, {0xED}, {0xDD}, {0xFD}, {0xDD, 0xCB, 0x01}} {
		for op := 0; op < 256; op++ {
			buf := append(append([]byte{}, pre...), byte(op), 0x11, 0x22, 0x33)
			in := Decode(func(a uint16) byte {
				if int(a) < len(buf) {
					return buf[a]
				}
				return 0
			}, 0)
			if in.Len < 1 || in.Len > 4 {
				t.Errorf("% X: len %d", buf[:len(pre)+1], in.Len)
			}
		}
	}
}
