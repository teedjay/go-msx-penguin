// Package rle decodes the ROM's run-length graphics streams.
//
// Ported from $454C-$4582 (High: traced instruction by instruction). A
// stream is a sequence of runs ended by a $00 control byte:
//
//	$01-$7F n   read one byte and write it n times (repeat run: the DJNZ
//	            at $455F targets the OUT, not the read)
//	$80-$FF     n = low 7 bits; copy the next n bytes (literal run: the
//	            DJNZ at $456D targets the read). $80 copies 256 bytes,
//	            because B = 0 makes DJNZ loop 256 times.
//
// In mirror mode (entry $4547, C = 1) every byte read is bit-reversed by
// $4573, which flips 8-pixel pattern rows horizontally.
package rle

import "fmt"

// Reader reads one ROM byte at a CPU address.
type Reader func(addr uint16) byte

// Decode decodes the stream at addr and returns the output and the address
// just past the terminator.
func Decode(rd Reader, addr uint16, mirror bool) (out []byte, end uint16, err error) {
	fetch := func() byte {
		b := rd(addr)
		addr++
		if mirror {
			b = Reverse(b)
		}
		return b
	}
	for {
		if len(out) > 0x4000 {
			return nil, addr, fmt.Errorf("rle: stream does not terminate within 16KB")
		}
		ctl := rd(addr)
		addr++
		switch {
		case ctl == 0:
			return out, addr, nil
		case ctl&0x80 == 0:
			v := fetch()
			for i := 0; i < int(ctl); i++ {
				out = append(out, v)
			}
		default:
			n := int(ctl & 0x7F)
			if n == 0 {
				n = 256
			}
			for i := 0; i < n; i++ {
				out = append(out, fetch())
			}
		}
	}
}

// Reverse reverses the bit order of b. Ported from $4573-$4582.
func Reverse(b byte) byte {
	var r byte
	for i := 0; i < 8; i++ {
		r = r<<1 | b&1
		b >>= 1
	}
	return r
}
