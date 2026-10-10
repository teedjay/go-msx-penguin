package game

// Exact Z80 DAA, for the packed-BCD score and timer arithmetic. Model it
// from the H and C flags of the preceding ADD/ADC/SUB, not as a "BCD add"
// that assumes valid operands.

// daa returns the corrected A and the new carry flag.
func daa(a byte, n, h, c bool) (byte, bool) {
	var diff byte
	carry := c
	if h || a&0x0F > 9 {
		diff |= 0x06
	}
	if c || a > 0x99 {
		diff |= 0x60
		carry = true
	}
	if n {
		return a - diff, carry
	}
	return a + diff, carry
}

// subDAA is SUB b / DAA.
func subDAA(a, b byte) (byte, bool) {
	h := a&0x0F < b&0x0F
	c := a < b
	return daa(a-b, true, h, c)
}

// adcDAA is ADC A,b (with carry in) / DAA.
func adcDAA(a, b byte, cin bool) (byte, bool) {
	ci := byte(0)
	if cin {
		ci = 1
	}
	h := a&0x0F+b&0x0F+ci > 0x0F
	c := uint16(a)+uint16(b)+uint16(ci) > 0xFF
	return daa(a+b+ci, false, h, c)
}
