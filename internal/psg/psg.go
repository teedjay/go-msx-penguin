// Package psg models the AY-3-8910 sound chip closely enough to hear the
// game: three square-wave tone channels, the noise generator, the mixer,
// the logarithmic volume table and the envelope generator. The game's
// sound driver only writes registers (internal/game); this turns register
// values into samples.
package psg

// Clock is the MSX PSG clock in Hz.
const Clock = 1789772.5

// volumes is the AY-3-8910 output level per volume step (about 3dB per
// step), normalised to 1.
var volumes = [16]float64{
	0, 0.0137, 0.0205, 0.0291, 0.0423, 0.0618, 0.0847, 0.1369,
	0.1691, 0.2647, 0.3527, 0.4499, 0.5704, 0.6873, 0.8482, 1,
}

// Chip is one PSG. Set registers with Write, then pull samples.
type Chip struct {
	Regs [16]byte

	rate      float64 // samples per second
	toneCount [3]float64
	toneOut   [3]bool
	noiseCnt  float64
	lfsr      uint32
	noiseOut  bool
	envCount  float64
	envStep   int
	envHold   bool
}

// New returns a chip producing samples at the given rate.
func New(rate int) *Chip {
	return &Chip{rate: float64(rate), lfsr: 1, Regs: [16]byte{7: 0xFF}}
}

// Write sets register r. Writing R13 restarts the envelope.
func (p *Chip) Write(r, v byte) {
	r &= 15
	p.Regs[r] = v
	if r == 13 {
		p.envStep, p.envHold, p.envCount = 0, false, 0
	}
}

// SetRegs copies a whole register file (R13 is applied only if changed,
// since rewriting it restarts the envelope).
func (p *Chip) SetRegs(regs [16]byte) {
	for r := byte(0); r < 16; r++ {
		if r == 13 && regs[13] == p.Regs[13] {
			continue
		}
		p.Write(r, regs[r])
	}
}

// Sample returns the next mono sample in [0, 1].
func (p *Chip) Sample() float64 {
	// Each generator steps at Clock/16 per period unit.
	step := Clock / 16 / p.rate
	for ch := 0; ch < 3; ch++ {
		period := float64(uint16(p.Regs[2*ch]) | uint16(p.Regs[2*ch+1]&0x0F)<<8)
		if period == 0 {
			period = 1
		}
		p.toneCount[ch] += step
		for p.toneCount[ch] >= period/2 {
			p.toneCount[ch] -= period / 2
			p.toneOut[ch] = !p.toneOut[ch]
		}
	}
	np := float64(p.Regs[6] & 0x1F)
	if np == 0 {
		np = 1
	}
	p.noiseCnt += step
	for p.noiseCnt >= np {
		p.noiseCnt -= np
		bit := (p.lfsr ^ p.lfsr>>3) & 1
		p.lfsr = p.lfsr>>1 | bit<<16
		p.noiseOut = p.lfsr&1 != 0
	}
	env := p.envelope(step)

	var out float64
	mix := p.Regs[7]
	for ch := 0; ch < 3; ch++ {
		tone := p.toneOut[ch] || mix&(1<<ch) != 0
		noise := p.noiseOut || mix&(8<<ch) != 0
		if !tone || !noise {
			continue
		}
		v := p.Regs[8+ch]
		level := volumes[v&0x0F]
		if v&0x10 != 0 {
			level = volumes[env]
		}
		out += level
	}
	return out / 3
}

// envelope advances the envelope generator and returns its level.
func (p *Chip) envelope(step float64) int {
	period := float64(uint16(p.Regs[11]) | uint16(p.Regs[12])<<8)
	if period == 0 {
		period = 1
	}
	shape := p.Regs[13] & 0x0F
	if !p.envHold {
		p.envCount += step
		for p.envCount >= period {
			p.envCount -= period
			p.envStep++
			if p.envStep >= 32 {
				if shape&0x08 == 0 || shape&0x01 != 0 { // hold
					p.envHold = true
					p.envStep = 31
					break
				}
				p.envStep = 0
			}
		}
	}
	attack := shape&0x04 != 0
	level := p.envStep & 0x0F
	cycle := p.envStep / 16
	if shape&0x08 != 0 && shape&0x02 != 0 && cycle == 1 { // alternate
		attack = !attack
	}
	if p.envHold {
		switch {
		case shape&0x08 == 0:
			return 0
		case shape&0x02 != 0: // hold after alternating
			if attack {
				return 0
			}
			return 15
		case attack:
			return 15
		default:
			return 0
		}
	}
	if attack {
		return level
	}
	return 15 - level
}
