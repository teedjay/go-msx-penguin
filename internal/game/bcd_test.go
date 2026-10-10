package game

import "testing"

func TestDAA(t *testing.T) {
	cases := []struct {
		a, b  byte
		sub   bool
		want  byte
		carry bool
	}{
		{0x10, 0x01, true, 0x09, false},
		{0x00, 0x01, true, 0x99, true},
		{0x20, 0x01, true, 0x19, false},
		{0x09, 0x01, false, 0x10, false},
		{0x99, 0x01, false, 0x00, true},
		{0x45, 0x55, false, 0x00, true},
	}
	for _, tc := range cases {
		var got byte
		var c bool
		if tc.sub {
			got, c = subDAA(tc.a, tc.b)
		} else {
			got, c = adcDAA(tc.a, tc.b, false)
		}
		if got != tc.want || c != tc.carry {
			t.Errorf("%02X op %02X (sub=%v): got %02X c=%v, want %02X c=%v", tc.a, tc.b, tc.sub, got, c, tc.want, tc.carry)
		}
	}
}
