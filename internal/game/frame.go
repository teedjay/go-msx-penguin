package game

import "fmt"

// Boot runs the cartridge INIT. Ported from $4010-$4042 (High).
//
// initIRQs is the number of interrupts the original takes during INIT:
// the BIOS routines it calls (and its own VRAM fill) re-enable them, and
// the handler then runs its per-interrupt work with the busy flag set.
// Under C-BIOS this is 2. The last of them follows the VRAM clear, which
// re-enables interrupts with one pending; any earlier one only differs in
// VRAM writes that the clear wipes and the later one repeats, so running
// all of them after the clear gives the same state. (Medium: confirmed by
// the comparator for 2.)
func (c *Console) Boot(initIRQs int) {
	// DI, IM 1, the H.KEYI hook and the stack have no equivalent: the
	// frontend calls Interrupt directly.
	c.RAM = [ramSize]byte{}
	c.RAM[0x005] = 1 // busy
	c.init447A()
	for i := 0; i < initIRQs; i++ {
		c.PerInterrupt()
	}
	c.RAM[0x005] = 0
	c.RAM[0x000] = 1
}

// init447A sets up the VDP, PSG and sound, and clears VRAM. Ported from
// $447A-$4494 (High).
func (c *Console) init447A() {
	c.setVDPRegs()
	c.wrtpsg(7, 0xB8)
	c.wrtpsg(15, 0x8F) // $45F0
	c.startSound(0x95) // $449D
	c.vfill(0x0000, 0x4000, 0)
}

// setVDPRegs copies the register table at $44BF to the RAM shadow at
// $E038 and writes R0-R7 from it. Ported from $44A3-$44BE (High).
func (c *Console) setVDPRegs() {
	c.ldir(0x44BF, 0xE038, 8)
	for r := byte(0); r < 8; r++ {
		c.wrtvdp(r, c.rd(0xE038+uint16(r)))
	}
}

// Interrupt is one VBlank interrupt: the per-interrupt work and, unless a
// tick is already running, one game tick. Every tick takes one interrupt,
// which is what the headless tools want; a real-time frontend uses Frame.
// Ported from $4045-$4099.
func (c *Console) Interrupt() {
	if c.PerInterrupt() {
		c.Tick()
		c.EndTick()
	}
}

// Frame is one 60Hz frame for a real-time frontend. It paces ticks like
// the original: a tick that would take longer than a frame keeps the
// busy flag set for the frames it owes, which then run the per-interrupt
// work only (sound keeps playing during loads). The cost model, 200
// cycles per VRAM address set-up and 66 per VRAM write against 59659
// cycles per frame, predicts the frame count of 76899 of 76918 ticks in
// the reference traces; the misses are load ticks off by 1-2 frames.
func (c *Console) Frame() {
	if c.owed > 0 {
		c.owed--
		c.PerInterrupt() // busy: per-interrupt work only
		if c.owed == 0 {
			c.EndTick()
		}
		return
	}
	if !c.PerInterrupt() {
		return
	}
	c.vwrites, c.vsets = 0, 0
	c.Tick()
	c.owed = (200*c.vsets + 66*c.vwrites) / 59659
	if c.owed == 0 {
		c.EndTick()
	}
}

// PerInterrupt is the part of the handler that runs on every interrupt,
// up to and including setting the busy flag. It returns false when a tick
// is already running (busy flag set), in which case the original returns
// at once. Ported from $404A-$4080 (High for the flow; the roles of $4C98
// and $464C are Medium).
func (c *Console) PerInterrupt() bool {
	if c.RAM[0x000] != 0 {
		c.soundDriver() // $79EA
	}
	if c.RAM[0x000] < 0x0C {
		if c.RAM[0x140]+c.RAM[0x142] == 0 { // ADD A,(HL): 8-bit
			c.upd4C98()
		}
		c.upd464C()
		var a byte
		if c.RAM[0x081]&0x80 != 0 {
			a = 1
		}
		c.RAM[0x0FC] = a
	}
	if c.RAM[0x005]&1 != 0 {
		return false
	}
	c.RAM[0x005] = 1
	return true
}

// Tick runs one game tick: input, then the state machine. The original
// re-enables interrupts here, so interrupts during a long tick run
// PerInterrupt only. Ported from $4082-$4086.
func (c *Console) Tick() {
	c.ticks++
	c.readInput()
	c.tick4107()
}

// TickNested runs one tick with the interrupts that arrived during it in
// the original delivered where they arrived. sched lists, in order, the
// number of VRAM writes the tick had made when each arrived. The
// original's VRAM write sequences run with interrupts disabled and
// re-enable them right after their last write, so each interrupt is run
// immediately after the write that brings the count to its value (one at
// 0 runs before the tick). This also reproduces what the original does
// when the interrupt's own VRAM writes move the VDP address under an RLE
// stream that runs with interrupts enabled. It returns an error when the
// port's tick makes fewer writes than the schedule needs.
func (c *Console) TickNested(sched []int) error {
	c.vwrites, c.vsets = 0, 0
	c.irqSched = sched
	c.deliver()
	c.Tick()
	c.deliver()
	left := c.irqSched
	c.irqSched = nil
	if len(left) > 0 {
		return fmt.Errorf("tick made %d VRAM writes; %d nested interrupts at %v not delivered", c.vwrites, len(left), left)
	}
	return nil
}

// deliver runs the scheduled nested interrupts due at the current VRAM
// write count. The handler runs with interrupts disabled, so writes made
// by a delivered interrupt never deliver another from inside it.
func (c *Console) deliver() {
	for !c.inIRQ && len(c.irqSched) > 0 && c.irqSched[0] <= c.vwrites {
		c.irqSched = c.irqSched[1:]
		c.inIRQ = true
		c.PerInterrupt()
		c.inIRQ = false
	}
}

// Ticks is the number of ticks started since power-on, which is also the
// index (as scripts/refdump.tcl counts them) of the next tick to start.
func (c *Console) Ticks() int { return c.ticks }

// VWrites is the number of VRAM writes since the current tick started.
func (c *Console) VWrites() int { return c.vwrites }

// VSets is the number of VRAM address set-ups since the tick started.
func (c *Console) VSets() int { return c.vsets }

// EndTick clears the busy flag. Ported from $4089-$408E.
func (c *Console) EndTick() { c.RAM[0x005] = 0 }

// readInput samples the controls into $E009 (previous value to $E008).
// Only in state 7 and later. Ported from $40A4-$4106 (High for the flow).
func (c *Console) readInput() {
	if c.RAM[0x000] < 7 {
		return
	}
	var a byte
	switch e002 := c.RAM[0x002]; {
	case e002&0x40 == 0:
		// Demo: a new input byte from the pointer at $E0EC every 32
		// ticks; otherwise the last direction is held.
		de := c.rd16(0xE0EC)
		c.RAM[0x0EB]++
		if c.RAM[0x0EB]&0x1F == 0 {
			a = c.rd(de)
			c.wr16(0xE0EC, de+1)
		} else {
			a = c.RAM[0x009] & 0x0F
		}
	case e002&0x10 == 0:
		a = ^c.rdpsg(14) & 0x3F // joystick
	default:
		// Keyboard rows 7 and 8, packed into the joystick bit layout.
		e := rrca(^c.snsmat(7)) & 0x20
		b := rrca(rrca(^c.snsmat(8)))
		cc := b&0x04 | e
		b = rrca(rrca(b))
		cc |= b & 0x18
		a = rrca(b)&0x03 | cc
	}
	c.RAM[0x008] = c.RAM[0x009]
	c.RAM[0x009] = a
}

// tick4107 counts the frame and runs the current state. Ported from
// $4107-$4133 (High).
func (c *Console) tick4107() {
	c.RAM[0x003]++
	if c.startKeys() {
		return // $4413 popped our return address
	}
	switch c.RAM[0x000] {
	case 0: // $4134: RET
	case 1:
		c.state1()
	case 2:
		c.state2()
	case 3:
		c.state3()
	case 4:
		c.state4()
	case 5:
		c.state5()
	case 6:
		c.state6()
	case 7:
		c.state7()
	case 8:
		c.state8()
	case 9:
		c.state9()
	case 10:
		c.state10()
	case 11:
		c.state11()
	case 12:
		c.state12()
	case 13:
		c.state13()
	case 14:
		c.state14()
	case 15:
		c.state15()
	}
}

// startKeys starts a game when 1 (joystick) or 2 (keyboard) is pressed
// outside play. It then POPs $4107's return address, so the tick ends
// without dispatching: it returns true for that. Note the state ends up
// 8, not 7: it stores 7 and then jumps to $43E9, which increments it.
// Ported from $4413-$4441 (High).
func (c *Console) startKeys() bool {
	if c.RAM[0x13B] != 0 || c.RAM[0x002]&0x40 != 0 {
		return false
	}
	var b byte
	switch ^c.snsmat(0) & 0x06 {
	case 0x02:
		b = 0x40
	case 0x04:
		b = 0x50
	default:
		return false
	}
	c.RAM[0x133] = 0
	c.RAM[0x002] = b
	c.RAM[0x000] = 7
	c.nextState43E9()
	return true
}

// State transitions.

// nextState43E9: $E004 = $50, then nextState43EE. Ported from $43E9.
func (c *Console) nextState43E9() { c.nextState43EB(0x50) }

// nextState43EB: $E004 = a, then nextState43EE. Ported from $43EB.
func (c *Console) nextState43EB(a byte) {
	c.RAM[0x004] = a
	c.nextState43EE()
}

// nextState43EE: next state, sub-state 0. Ported from $43EE-$43F6.
func (c *Console) nextState43EE() {
	c.RAM[0x000]++
	c.RAM[0x001] = 0
}
