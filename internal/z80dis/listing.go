package z80dis

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Label returns the name of a code address: a configured label, Lxxxx for
// any branch target in the ROM, a BIOS name below the ROM, or "".
func (a *Analysis) Label(addr uint16) string {
	if s, ok := a.Cfg.Labels[addr]; ok {
		return s
	}
	if a.inROM(addr) && a.Targets[addr] {
		return fmt.Sprintf("L%04X", addr)
	}
	if !a.inROM(addr) {
		return BIOS[addr]
	}
	return ""
}

// Name formats a 16-bit operand according to what it means.
func (a *Analysis) Name(k ValKind, v uint16) string {
	switch k {
	case ValTarget:
		return a.Label(v)
	case ValMem:
		if s, ok := a.Cfg.Labels[v]; ok {
			return s
		}
		return SysVars[v]
	}
	return ""
}

// WriteListing writes the full annotated listing, ROM order, with
// unclaimed bytes as DB lines.
func (a *Analysis) WriteListing(w io.Writer) error {
	bw := bufio.NewWriter(w)
	tableAt := map[uint16]Table{}
	for _, t := range a.Tables {
		tableAt[t.Addr] = t
	}
	end := int(a.Base) + len(a.ROM)
	for pc := int(a.Base); pc < end; {
		addr := uint16(pc)
		if c, ok := a.Cfg.Comments[addr]; ok {
			fmt.Fprintf(bw, "\n; %s\n", c)
		}
		if l := a.Label(addr); l != "" && a.own(addr) != CodeBody {
			fmt.Fprintf(bw, "%s:\n", l)
		}
		if t, ok := tableAt[addr]; ok {
			note := ""
			if t.Heuristic {
				note = fmt.Sprintf(" (heuristic, %d entries", t.Count)
				if t.Tight {
					note += ", tight"
				}
				note += ")"
			}
			fmt.Fprintf(bw, "                                ; jump table%s\n", note)
		}
		switch a.own(addr) {
		case CodeStart:
			in := a.Insts[addr]
			cmt := ""
			if in.Kind == ValImm && a.inROM(in.Val) {
				if l := a.Label(in.Val); l != "" {
					cmt = " ; -> " + l
				}
			}
			if _, ok := a.Pushed[in.Val]; ok && in.Kind == ValImm {
				cmt += " (pushed continuation)"
			}
			fmt.Fprintf(bw, "  %04X  %-12s  %s%s\n", addr, hexBytes(a.ROM[pc-int(a.Base):pc-int(a.Base)+in.Len]), in.Format(a.Name), cmt)
			pc += in.Len
		case Data:
			// Inside a table: two bytes per DW line.
			if inTable(a.Tables, addr) {
				v := a.word(addr)
				fmt.Fprintf(bw, "  %04X  %-12s  DW %s\n", addr, hexBytes(a.ROM[pc-int(a.Base):pc-int(a.Base)+2]), orHex(a.Label(v), v))
				pc += 2
				continue
			}
			pc = a.writeDB(bw, pc, Data)
		default:
			pc = a.writeDB(bw, pc, Unclaimed)
		}
	}
	return bw.Flush()
}

// writeDB writes up to 8 bytes of one ownership kind, stopping at a label.
func (a *Analysis) writeDB(bw *bufio.Writer, pc int, kind byte) int {
	end := int(a.Base) + len(a.ROM)
	start := pc
	for pc < end && pc-start < 8 && a.Own[pc-int(a.Base)] == kind {
		if pc != start && (a.Label(uint16(pc)) != "" || a.Cfg.Comments[uint16(pc)] != "") {
			break
		}
		pc++
	}
	b := a.ROM[start-int(a.Base) : pc-int(a.Base)]
	parts := make([]string, len(b))
	for i, x := range b {
		parts[i] = fmt.Sprintf("$%02X", x)
	}
	tag := "?"
	if kind == Data {
		tag = "data"
	}
	fmt.Fprintf(bw, "  %04X  %-12s  DB %s  ; %s\n", start, "", strings.Join(parts, ","), tag)
	return pc
}

func inTable(ts []Table, addr uint16) bool {
	for _, t := range ts {
		if addr >= t.Addr && addr < t.Addr+uint16(2*t.Count) {
			return true
		}
	}
	return false
}

func orHex(s string, v uint16) string {
	if s != "" {
		return s
	}
	return fmt.Sprintf("$%04X", v)
}

func hexBytes(b []byte) string {
	parts := make([]string, len(b))
	for i, x := range b {
		parts[i] = fmt.Sprintf("%02X", x)
	}
	return strings.Join(parts, " ")
}

// Peek decodes linearly from lo to hi without claiming anything.
func Peek(w io.Writer, romData []byte, base, lo, hi uint16, name func(ValKind, uint16) string) {
	mem := func(a uint16) byte {
		if a >= base && int(a) < int(base)+len(romData) {
			return romData[a-base]
		}
		return 0
	}
	for pc := int(lo); pc <= int(hi); {
		in := Decode(mem, uint16(pc))
		var bs []byte
		for i := 0; i < in.Len; i++ {
			bs = append(bs, mem(uint16(pc+i)))
		}
		fmt.Fprintf(w, "  %04X  %-12s  %s\n", pc, hexBytes(bs), in.Format(name))
		pc += in.Len
	}
}
