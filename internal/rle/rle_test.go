package rle

import (
	"bytes"
	"testing"
)

func TestDecode(t *testing.T) {
	src := []byte{0x03, 0xAA, 0x82, 0x01, 0x02, 0x00, 0xEE}
	rd := func(a uint16) byte { return src[a] }
	out, end, err := Decode(rd, 0, false)
	if err != nil || !bytes.Equal(out, []byte{0xAA, 0xAA, 0xAA, 0x01, 0x02}) || end != 6 {
		t.Errorf("got % X end %d err %v", out, end, err)
	}
	out, _, _ = Decode(rd, 0, true)
	if !bytes.Equal(out, []byte{0x55, 0x55, 0x55, 0x80, 0x40}) {
		t.Errorf("mirror: got % X", out)
	}
}

func TestLiteral256(t *testing.T) {
	src := append([]byte{0x80}, make([]byte, 256)...)
	src = append(src, 0x00)
	out, end, err := Decode(func(a uint16) byte { return src[a] }, 0, false)
	if err != nil || len(out) != 256 || end != 258 {
		t.Errorf("len %d end %d err %v", len(out), end, err)
	}
}
