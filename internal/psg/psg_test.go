package psg

import (
	"math"
	"testing"
)

// TestToneFrequency checks a channel's square wave runs at Clock/(16*P).
func TestToneFrequency(t *testing.T) {
	const rate = 44100
	p := New(rate)
	p.Write(0, 0xFE) // period 254: 440.4 Hz
	p.Write(7, 0xFE) // tone A only
	p.Write(8, 15)
	rises, last := 0, false
	for i := 0; i < rate; i++ {
		on := p.Sample() > 0
		if on && !last {
			rises++
		}
		last = on
	}
	want := Clock / (16 * 254)
	if math.Abs(float64(rises)-want) > 2 {
		t.Errorf("got %d cycles per second, want %.1f", rises, want)
	}
}
