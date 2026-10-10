// Package z80dis is a Z80 disassembler with recursive descent and
// byte-level coverage tracking, used to map the original ROM. It is an
// analysis tool only: nothing in the game engine executes Z80 code.
package z80dis

import (
	"fmt"
	"strings"
)

// Flow classifies how an instruction affects control flow.
type Flow int

const (
	FlowNone     Flow = iota // falls through
	FlowJump                 // JP nn, JR e: unconditional, target known
	FlowCondJump             // JP cc / JR cc / DJNZ: target known, may fall through
	FlowCall                 // CALL nn
	FlowCondCall             // CALL cc,nn
	FlowRST                  // RST n (a call into page 0)
	FlowRet                  // RET, RETI, RETN
	FlowCondRet              // RET cc: may fall through
	FlowIndirect             // JP (HL) / (IX) / (IY): target unknown
)

// ValKind says what the instruction's 16-bit value means, so it can be
// formatted correctly: only branch targets get routine labels.
type ValKind int

const (
	ValNone   ValKind = iota
	ValTarget         // branch or call target
	ValMem            // memory operand address, e.g. LD A,(nn)
	ValImm            // immediate, e.g. LD HL,nn
)

// Inst is one decoded instruction.
type Inst struct {
	Addr  uint16
	Len   int
	Text  string // mnemonic; the 16-bit value, if any, is written as "{nn}"
	Flow  Flow
	Kind  ValKind
	Val   uint16 // the 16-bit value (or computed relative target)
	Reg16 string // for LD rr,nn and PUSH rr: the register pair
	Valid bool   // false for opcodes that are not real instructions
}

// Format renders the instruction, naming the 16-bit value with name (which
// may return "" for plain hex).
func (in Inst) Format(name func(ValKind, uint16) string) string {
	if in.Kind == ValNone {
		return in.Text
	}
	s := ""
	if name != nil {
		s = name(in.Kind, in.Val)
	}
	if s == "" {
		s = fmt.Sprintf("$%04X", in.Val)
	}
	return strings.Replace(in.Text, "{nn}", s, 1)
}

var (
	tR   = [8]string{"B", "C", "D", "E", "H", "L", "(HL)", "A"}
	tRP  = [4]string{"BC", "DE", "HL", "SP"}
	tRP2 = [4]string{"BC", "DE", "HL", "AF"}
	tCC  = [8]string{"NZ", "Z", "NC", "C", "PO", "PE", "P", "M"}
	tALU = [8]string{"ADD A,", "ADC A,", "SUB ", "SBC A,", "AND ", "XOR ", "OR ", "CP "}
	tROT = [8]string{"RLC", "RRC", "RL", "RR", "SLA", "SRA", "SLL", "SRL"}
	tIM  = [8]string{"0", "0/1", "1", "2", "0", "0/1", "1", "2"}
	tBLI = [4][4]string{
		{"LDI", "CPI", "INI", "OUTI"},
		{"LDD", "CPD", "IND", "OUTD"},
		{"LDIR", "CPIR", "INIR", "OTIR"},
		{"LDDR", "CPDR", "INDR", "OTDR"},
	}
)

// Decode decodes the instruction at pc, reading bytes through mem.
func Decode(mem func(uint16) byte, pc uint16) Inst {
	d := decoder{mem: mem, start: pc, pc: pc}
	in := d.decode()
	in.Addr = pc
	in.Len = int(d.pc - pc)
	return in
}

type decoder struct {
	mem   func(uint16) byte
	start uint16
	pc    uint16
	idx   string // "", "IX" or "IY"
	disp  int8
	haveD bool
}

func (d *decoder) next() byte {
	b := d.mem(d.pc)
	d.pc++
	return b
}

func (d *decoder) nn() uint16 {
	lo := uint16(d.next())
	return lo | uint16(d.next())<<8
}

func hex8(b byte) string { return fmt.Sprintf("$%02X", b) }

// hl is HL or the active index register.
func (d *decoder) hl() string {
	if d.idx != "" {
		return d.idx
	}
	return "HL"
}

func (d *decoder) rp(p int) string {
	if p == 2 {
		return d.hl()
	}
	return tRP[p]
}

func (d *decoder) rp2(p int) string {
	if p == 2 {
		return d.hl()
	}
	return tRP2[p]
}

// indexed returns "(IX+d)", reading the displacement on first use.
func (d *decoder) indexed() string {
	if !d.haveD {
		d.disp = int8(d.next())
		d.haveD = true
	}
	if d.disp < 0 {
		return fmt.Sprintf("(%s-$%02X)", d.idx, -int(d.disp))
	}
	return fmt.Sprintf("(%s+$%02X)", d.idx, d.disp)
}

// r returns register y. With an index prefix, H/L become IXH/IXL unless
// the same instruction also uses (IX+d) (noIdxHL).
func (d *decoder) r(i int, noIdxHL bool) string {
	if d.idx != "" {
		switch i {
		case 6:
			return d.indexed()
		case 4, 5:
			if !noIdxHL {
				return d.idx + tR[i][:1] // IXH / IXL
			}
		}
	}
	return tR[i]
}

func ok(text string) Inst { return Inst{Text: text, Valid: true} }

func (d *decoder) withVal(text string, kind ValKind, v uint16) Inst {
	return Inst{Text: text, Kind: kind, Val: v, Valid: true}
}

func (d *decoder) decode() Inst {
	op := d.next()
	switch op {
	case 0xCB:
		return d.decodeCB()
	case 0xED:
		return d.decodeED()
	case 0xDD, 0xFD:
		d.idx = "IX"
		if op == 0xFD {
			d.idx = "IY"
		}
		nxt := d.mem(d.pc)
		if nxt == 0xDD || nxt == 0xFD || nxt == 0xED {
			// The prefix acts as a NOP; real code does not do this.
			return Inst{Text: "DB " + hex8(op), Valid: false}
		}
		op = d.next()
		if op == 0xCB {
			return d.decodeIndexCB()
		}
	}
	return d.decodeMain(op)
}

func (d *decoder) decodeMain(op byte) Inst {
	x, y, z := int(op>>6), int(op>>3&7), int(op&7)
	p, q := y>>1, y&1
	switch x {
	case 0:
		switch z {
		case 0:
			switch y {
			case 0:
				return ok("NOP")
			case 1:
				return ok("EX AF,AF'")
			default:
				e := int8(d.next())
				t := uint16(int(d.pc) + int(e))
				in := d.withVal("", ValTarget, t)
				switch {
				case y == 2:
					in.Text, in.Flow = "DJNZ {nn}", FlowCondJump
				case y == 3:
					in.Text, in.Flow = "JR {nn}", FlowJump
				default:
					in.Text, in.Flow = "JR "+tCC[y-4]+",{nn}", FlowCondJump
				}
				return in
			}
		case 1:
			if q == 0 {
				in := d.withVal("LD "+d.rp(p)+",{nn}", ValImm, d.nn())
				in.Reg16 = d.rp(p)
				return in
			}
			return ok("ADD " + d.hl() + "," + d.rp(p))
		case 2:
			switch y {
			case 0:
				return ok("LD (BC),A")
			case 1:
				return ok("LD A,(BC)")
			case 2:
				return ok("LD (DE),A")
			case 3:
				return ok("LD A,(DE)")
			case 4:
				return d.withVal("LD ({nn}),"+d.hl(), ValMem, d.nn())
			case 5:
				return d.withVal("LD "+d.hl()+",({nn})", ValMem, d.nn())
			case 6:
				return d.withVal("LD ({nn}),A", ValMem, d.nn())
			default:
				return d.withVal("LD A,({nn})", ValMem, d.nn())
			}
		case 3:
			if q == 0 {
				return ok("INC " + d.rp(p))
			}
			return ok("DEC " + d.rp(p))
		case 4:
			return ok("INC " + d.r(y, false))
		case 5:
			return ok("DEC " + d.r(y, false))
		case 6:
			dst := d.r(y, false) // reads the displacement before n
			return ok("LD " + dst + "," + hex8(d.next()))
		default:
			return ok([8]string{"RLCA", "RRCA", "RLA", "RRA", "DAA", "CPL", "SCF", "CCF"}[y])
		}
	case 1:
		if y == 6 && z == 6 {
			return ok("HALT")
		}
		mem := y == 6 || z == 6
		return ok("LD " + d.r(y, mem) + "," + d.r(z, mem))
	case 2:
		return ok(tALU[y] + d.r(z, false))
	}
	// x == 3
	switch z {
	case 0:
		in := ok("RET " + tCC[y])
		in.Flow = FlowCondRet
		return in
	case 1:
		if q == 0 {
			in := ok("POP " + d.rp2(p))
			return in
		}
		switch p {
		case 0:
			in := ok("RET")
			in.Flow = FlowRet
			return in
		case 1:
			return ok("EXX")
		case 2:
			in := ok("JP (" + d.hl() + ")")
			in.Flow = FlowIndirect
			return in
		default:
			return ok("LD SP," + d.hl())
		}
	case 2:
		in := d.withVal("JP "+tCC[y]+",{nn}", ValTarget, d.nn())
		in.Flow = FlowCondJump
		return in
	case 3:
		switch y {
		case 0:
			in := d.withVal("JP {nn}", ValTarget, d.nn())
			in.Flow = FlowJump
			return in
		case 2:
			return ok("OUT (" + hex8(d.next()) + "),A")
		case 3:
			return ok("IN A,(" + hex8(d.next()) + ")")
		case 4:
			return ok("EX (SP)," + d.hl())
		case 5:
			return ok("EX DE,HL")
		case 6:
			return ok("DI")
		case 7:
			return ok("EI")
		}
		// y == 1 (CB) is handled before decodeMain.
	case 4:
		in := d.withVal("CALL "+tCC[y]+",{nn}", ValTarget, d.nn())
		in.Flow = FlowCondCall
		return in
	case 5:
		if q == 0 {
			in := ok("PUSH " + d.rp2(p))
			in.Reg16 = d.rp2(p)
			return in
		}
		if p == 0 {
			in := d.withVal("CALL {nn}", ValTarget, d.nn())
			in.Flow = FlowCall
			return in
		}
		// prefixes handled in decode
	case 6:
		return ok(tALU[y] + hex8(d.next()))
	case 7:
		in := d.withVal("RST {nn}", ValTarget, uint16(y*8))
		in.Flow = FlowRST
		return in
	}
	return Inst{Text: "DB " + hex8(byte(0)), Valid: false}
}

func (d *decoder) decodeCB() Inst {
	op := d.next()
	x, y, z := int(op>>6), int(op>>3&7), int(op&7)
	reg := tR[z]
	switch x {
	case 0:
		return ok(tROT[y] + " " + reg)
	case 1:
		return ok(fmt.Sprintf("BIT %d,%s", y, reg))
	case 2:
		return ok(fmt.Sprintf("RES %d,%s", y, reg))
	}
	return ok(fmt.Sprintf("SET %d,%s", y, reg))
}

// decodeIndexCB handles DD CB d op / FD CB d op.
func (d *decoder) decodeIndexCB() Inst {
	m := d.indexed()
	op := d.next()
	x, y, z := int(op>>6), int(op>>3&7), int(op&7)
	var s string
	switch x {
	case 0:
		s = tROT[y] + " " + m
	case 1:
		return ok(fmt.Sprintf("BIT %d,%s", y, m))
	case 2:
		s = fmt.Sprintf("RES %d,%s", y, m)
	default:
		s = fmt.Sprintf("SET %d,%s", y, m)
	}
	in := ok(s)
	if z != 6 {
		in.Text += "," + tR[z] // undocumented copy to a register
		in.Valid = false
	}
	return in
}

func (d *decoder) decodeED() Inst {
	op := d.next()
	x, y, z := int(op>>6), int(op>>3&7), int(op&7)
	p, q := y>>1, y&1
	if x == 1 {
		switch z {
		case 0:
			if y == 6 {
				return ok("IN (C)")
			}
			return ok("IN " + tR[y] + ",(C)")
		case 1:
			if y == 6 {
				return ok("OUT (C),0")
			}
			return ok("OUT (C)," + tR[y])
		case 2:
			if q == 0 {
				return ok("SBC HL," + tRP[p])
			}
			return ok("ADC HL," + tRP[p])
		case 3:
			if q == 0 {
				return d.withVal("LD ({nn}),"+tRP[p], ValMem, d.nn())
			}
			return d.withVal("LD "+tRP[p]+",({nn})", ValMem, d.nn())
		case 4:
			return ok("NEG")
		case 5:
			in := ok("RETN")
			if y == 1 {
				in.Text = "RETI"
			}
			in.Flow = FlowRet
			return in
		case 6:
			return ok("IM " + tIM[y])
		default:
			return ok([8]string{"LD I,A", "LD R,A", "LD A,I", "LD A,R", "RRD", "RLD", "NOP", "NOP"}[y])
		}
	}
	if x == 2 && z <= 3 && y >= 4 {
		return ok(tBLI[y-4][z])
	}
	return Inst{Text: "DB $ED," + hex8(op), Valid: false}
}
