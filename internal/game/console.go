// Package game is the native port of the cartridge. Each function is
// rewritten from one Z80 routine, and its doc comment names the address
// range it came from. No Z80 code runs.
//
// Confidence key used in comments: High (traced instruction by
// instruction and/or confirmed by the lockstep comparator), Medium
// (control flow clear, meaning inferred), Low (plausible guess).
package game

import (
	"fmt"

	"go-msx-penguin/assets"
)

const (
	ramBase = 0xE000
	ramSize = 0x800
)

// Console is the hardware the original ran on, reduced to what it uses.
type Console struct {
	RAM  [ramSize]byte // $E000-$E7FF
	VRAM [0x4000]byte
	Regs [8]byte  // VDP registers R0-R7
	PSG  [16]byte // PSG registers as last written

	// Keys is the keyboard matrix as BIOS SNSMAT returns it: a clear bit
	// is a pressed key. JoyA is PSG port A as read through R14: bits 0-5
	// are joystick 1 (clear = pressed).
	Keys [11]byte
	JoyA byte

	// Unported names the first not-yet-ported routine the engine reached,
	// so the comparator can report "ticks 0-N match, tick N reached
	// unported $XXXX".
	Unported string

	vaddr uint16 // VDP address pointer

	// Nested interrupt delivery for lockstep runs (see TickNested).
	vwrites  int   // VRAM data writes since the tick started
	vsets    int   // VRAM address set-ups since the tick started
	irqSched []int // vwrites values at which nested interrupts arrive
	inIRQ    bool

	owed  int // frames the running tick still takes (Frame pacing)
	ticks int // ticks started since power-on
}

// New returns a powered-on console with no keys pressed.
func New() *Console {
	c := &Console{JoyA: 0x3F}
	for i := range c.Keys {
		c.Keys[i] = 0xFF
	}
	return c
}

// unported records the first routine the engine needed but does not have.
func (c *Console) unported(addr uint16) {
	if c.Unported == "" {
		c.Unported = fmt.Sprintf("$%04X", addr)
	}
}

// CPU-address helpers, so pointer-walking routines translate line for
// line. The cartridge range reads through the asset manifest and ignores
// writes, as on hardware; anything unmapped panics.

func (c *Console) rd(a uint16) byte {
	switch {
	case a >= 0x4000 && a < 0x8000:
		return assets.ROMByte(a)
	case a < 2:
		// After a game over the demo input reader runs with its pointer
		// at $E0EC still 0 and reads the BIOS: every MSX BIOS (and
		// C-BIOS) starts with DI ($F3), JP ($C3). Nothing further has
		// been seen read; anything else in page 0 still panics.
		return [2]byte{0xF3, 0xC3}[a]
	case a >= ramBase && a < ramBase+ramSize:
		return c.RAM[a-ramBase]
	}
	panic(fmt.Sprintf("game: read of unmapped address $%04X", a))
}

func (c *Console) wr(a uint16, v byte) {
	switch {
	case a >= 0x4000 && a < 0x8000:
		// Writes to the cartridge have no effect.
	case a >= ramBase && a < ramBase+ramSize:
		c.RAM[a-ramBase] = v
	default:
		panic(fmt.Sprintf("game: write of unmapped address $%04X", a))
	}
}

func (c *Console) rd16(a uint16) uint16 { return uint16(c.rd(a)) | uint16(c.rd(a+1))<<8 }

func (c *Console) wr16(a, v uint16) {
	c.wr(a, byte(v))
	c.wr(a+1, byte(v>>8))
}

// ldir copies n bytes upward, like LDIR (n = 0 copies 65536).
func (c *Console) ldir(src, dst uint16, n uint16) {
	for {
		c.wr(dst, c.rd(src))
		src++
		dst++
		n--
		if n == 0 {
			return
		}
	}
}

// BIOS equivalents.

// wrtvdp is BIOS WRTVDP ($0047).
func (c *Console) wrtvdp(reg, v byte) { c.Regs[reg&7] = v }

// wrtpsg is BIOS WRTPSG ($0093).
func (c *Console) wrtpsg(reg, v byte) { c.PSG[reg&15] = v }

// rdpsg is BIOS RDPSG ($0096). R14 is the joystick port.
func (c *Console) rdpsg(reg byte) byte {
	if reg&15 == 14 {
		return c.JoyA
	}
	return c.PSG[reg&15]
}

// snsmat is BIOS SNSMAT ($0141).
func (c *Console) snsmat(row byte) byte {
	if int(row) < len(c.Keys) {
		return c.Keys[row]
	}
	return 0xFF
}

// VDP access. The original passes VRAM addresses with bit 14 sometimes
// set ($6000 for $2000); the VDP only sees 14 bits.

// vsetwr sets the VRAM write address. Ported from $48B7-$48C5.
func (c *Console) vsetwr(de uint16) {
	c.vaddr = de & 0x3FFF
	c.vsets++
}

// vout writes one byte at the VRAM address and advances it, like
// OUT (C),A with C holding the VDP data port.
func (c *Console) vout(v byte) {
	c.VRAM[c.vaddr&0x3FFF] = v
	c.vaddr = (c.vaddr + 1) & 0x3FFF
	c.vwrites++
	if c.irqSched != nil {
		c.deliver()
	}
}

// vwrbyte writes A to VRAM[DE]. Ported from $48A5-$48AD.
func (c *Console) vwrbyte(de uint16, a byte) {
	c.vsetwr(de)
	c.vout(a)
}

// vcopy copies bc bytes from CPU address hl to VRAM de. Ported from
// $44C7-$44DB.
func (c *Console) vcopy(hl, de, bc uint16) {
	c.vsetwr(de)
	for {
		c.vout(c.rd(hl))
		hl++
		bc--
		if bc == 0 {
			return
		}
	}
}

// vfill writes a to bc bytes of VRAM from de. Ported from $44DC-$44F0. It
// returns with BC = 0, which some callers rely on (C = 0 selects the
// normal RLE mode).
func (c *Console) vfill(de, bc uint16, a byte) {
	c.vsetwr(de)
	for {
		c.vout(a)
		bc--
		if bc == 0 {
			return
		}
	}
}

// addDE is DE += A with carry into D. Ported from $48D8-$48DC.
func addDE(de uint16, a byte) uint16 { return de + uint16(a) }

// addHL is HL += A with carry into H. Ported from $48D3-$48D7.
func addHL(hl uint16, a byte) uint16 { return hl + uint16(a) }

func rrca(a byte) byte { return a>>1 | a<<7 }
