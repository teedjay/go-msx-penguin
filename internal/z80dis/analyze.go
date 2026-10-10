package z80dis

import (
	"fmt"
	"sort"
	"strings"
)

// Byte ownership in the coverage map.
const (
	Unclaimed = iota
	CodeStart // first byte of an instruction
	CodeBody  // later byte of an instruction
	Data      // claimed as data (table entries, declared ranges)
)

// Config holds what the analyst has confirmed about the ROM. It is read
// from a text file (see ParseConfig) and holds addresses only, never ROM
// data.
type Config struct {
	Entries  []uint16          // code entry points
	Labels   map[uint16]string // names for addresses
	Comments map[uint16]string
	NoReturn map[uint16]bool // CALL to these never returns to the caller
	// Inline maps a trampoline address to the number of DW entries that
	// follow each CALL to it (0 = grow heuristically).
	Inline map[uint16]int
	Tables []Table // free-standing DW tables of code pointers
	Data   []Range // declared data ranges
}

// Table is a table of 16-bit code pointers.
type Table struct {
	Addr  uint16
	Count int
	// Set during analysis for inline tables.
	Heuristic bool   // length was grown, not confirmed
	Tight     bool   // table ends exactly at its lowest target
	Caller    uint16 // address of the CALL, for inline tables
}

// Range is an inclusive address range.
type Range struct {
	Lo, Hi uint16
	Note   string
}

// Analysis is the result of recursive descent over the ROM.
type Analysis struct {
	ROM       []byte
	Base      uint16
	Cfg       *Config
	Own       []byte // coverage map, one entry per ROM byte
	Insts     map[uint16]Inst
	Tables    []Table
	Targets   map[uint16]bool   // addresses branched or called to
	Conflicts []string          // misaligned decodes and other problems
	Pushed    map[uint16]uint16 // continuation address -> LD site
}

func (a *Analysis) inROM(addr uint16) bool {
	return int(addr) >= int(a.Base) && int(addr) < int(a.Base)+len(a.ROM)
}

func (a *Analysis) mem(addr uint16) byte {
	if a.inROM(addr) {
		return a.ROM[addr-a.Base]
	}
	return 0
}

func (a *Analysis) own(addr uint16) byte { return a.Own[addr-a.Base] }

func (a *Analysis) word(addr uint16) uint16 {
	return uint16(a.mem(addr)) | uint16(a.mem(addr+1))<<8
}

// Analyze runs the descent. Inline tables with a count of 0 are grown one
// entry at a time, re-running the full descent after each, so code found
// through earlier entries stops a table from overrunning into it.
func Analyze(romData []byte, base uint16, cfg *Config) *Analysis {
	grown := map[uint16]int{} // CALL site -> current entry count
	for {
		a := descend(romData, base, cfg, grown)
		changed := false
		for i := range a.Tables {
			t := &a.Tables[i]
			if !t.Heuristic || t.Tight {
				continue
			}
			// Try one more entry.
			next := t.Addr + uint16(2*t.Count)
			if !a.inROM(next+1) || a.own(next) != Unclaimed || a.own(next+1) != Unclaimed {
				continue
			}
			tgt := a.word(next)
			if !a.inROM(tgt) || (tgt >= t.Addr && tgt <= next+1) {
				continue
			}
			if o := a.own(tgt); o == CodeBody || o == Data {
				continue
			}
			grown[t.Caller] = t.Count + 1
			changed = true
		}
		if !changed {
			return a
		}
	}
}

func descend(romData []byte, base uint16, cfg *Config, grown map[uint16]int) *Analysis {
	a := &Analysis{
		ROM: romData, Base: base, Cfg: cfg,
		Own:     make([]byte, len(romData)),
		Insts:   map[uint16]Inst{},
		Targets: map[uint16]bool{},
		Pushed:  map[uint16]uint16{},
	}
	for _, r := range cfg.Data {
		for x := int(r.Lo); x <= int(r.Hi); x++ {
			if a.inROM(uint16(x)) {
				a.Own[uint16(x)-base] = Data
			}
		}
	}
	var work []uint16
	add := func(t uint16) {
		if a.inROM(t) {
			a.Targets[t] = true
			work = append(work, t)
		}
	}
	claimTable := func(t Table) {
		for i := 0; i < t.Count; i++ {
			e := t.Addr + uint16(2*i)
			if !a.inROM(e + 1) {
				break
			}
			a.Own[e-base], a.Own[e+1-base] = Data, Data
		}
		for i := 0; i < t.Count; i++ {
			add(a.word(t.Addr + uint16(2*i)))
		}
	}
	for _, t := range cfg.Tables {
		a.Tables = append(a.Tables, t)
		claimTable(t)
	}
	for _, e := range cfg.Entries {
		add(e)
	}
	for len(work) > 0 {
		pc := work[len(work)-1]
		work = work[:len(work)-1]
		var prev Inst
		for {
			if !a.inROM(pc) {
				break
			}
			switch a.own(pc) {
			case CodeStart:
				goto nextWork
			case CodeBody, Data:
				a.Conflicts = append(a.Conflicts, fmt.Sprintf("$%04X: flow reaches the middle of an instruction or data", pc))
				goto nextWork
			}
			in := Decode(a.mem, pc)
			for i := 0; i < in.Len; i++ {
				if !a.inROM(pc+uint16(i)) || a.own(pc+uint16(i)) != Unclaimed {
					a.Conflicts = append(a.Conflicts, fmt.Sprintf("$%04X: instruction %q overlaps claimed bytes at $%04X", pc, in.Format(nil), pc+uint16(i)))
					goto nextWork
				}
			}
			if !in.Valid {
				a.Conflicts = append(a.Conflicts, fmt.Sprintf("$%04X: invalid or undocumented opcode %q", pc, in.Format(nil)))
			}
			a.Insts[pc] = in
			a.Own[pc-base] = CodeStart
			for i := 1; i < in.Len; i++ {
				a.Own[pc+uint16(i)-base] = CodeBody
			}
			// A constant pushed as a continuation is a code reference:
			// some later RET jumps to it.
			if in.Text == "PUSH "+in.Reg16 && prev.Kind == ValImm && prev.Reg16 == in.Reg16 && a.inROM(prev.Val) {
				switch pushedFate(a, pc+uint16(in.Len)) {
				case fateContinuation:
					a.Pushed[prev.Val] = prev.Addr
					add(prev.Val)
				case fateUnknown:
					a.Conflicts = append(a.Conflicts, fmt.Sprintf("$%04X: pushed constant $%04X: cannot tell continuation from data; confirm by reading", prev.Addr, prev.Val))
				}
			}
			next := pc + uint16(in.Len)
			switch in.Flow {
			case FlowJump:
				add(in.Val)
				goto nextWork
			case FlowCondJump, FlowCondCall:
				add(in.Val)
			case FlowCall:
				add(in.Val)
				if n, isInline := cfg.Inline[in.Val]; isInline {
					t := Table{Addr: next, Count: n, Caller: pc}
					if n == 0 {
						t.Heuristic = true
						t.Count = grown[pc]
						if t.Count == 0 {
							t.Count = 1
						}
					}
					t.Tight = tight(a, t)
					a.Tables = append(a.Tables, t)
					claimTable(t)
					goto nextWork
				}
				if cfg.NoReturn[in.Val] {
					goto nextWork
				}
			case FlowRet, FlowIndirect:
				goto nextWork
			}
			prev = in
			pc = next
		}
	nextWork:
	}
	sort.Slice(a.Tables, func(i, j int) bool { return a.Tables[i].Addr < a.Tables[j].Addr })
	return a
}

type fate int

const (
	fateUnknown fate = iota
	fateContinuation
	fateData
)

// pushedFate follows the code after a PUSH of a constant, tracking stack
// depth, to see what happens to the pushed value. Popped back off: it was
// a data pointer. Still on the stack at a RET or JP (rr): some later RET
// lands on it, so it is a continuation. Calls are assumed balanced, and
// only the fall-through of conditional branches is followed.
func pushedFate(a *Analysis, pc uint16) fate {
	depth := 1
	for n := 0; n < 64 && a.inROM(pc); n++ {
		in := Decode(a.mem, pc)
		switch {
		case strings.HasPrefix(in.Text, "PUSH "):
			depth++
		case strings.HasPrefix(in.Text, "POP "):
			depth--
			if depth == 0 {
				return fateData
			}
		case strings.HasPrefix(in.Text, "EX (SP),") && depth == 1:
			return fateUnknown
		case strings.HasPrefix(in.Text, "LD SP,") || strings.HasPrefix(in.Text, "INC SP") || strings.HasPrefix(in.Text, "DEC SP"):
			return fateUnknown
		}
		switch in.Flow {
		case FlowRet, FlowIndirect:
			return fateContinuation
		case FlowJump:
			pc = in.Val
			continue
		}
		pc += uint16(in.Len)
	}
	return fateUnknown
}

// tight reports whether the table ends exactly at its lowest target, the
// strongest cheap evidence that its length is right.
func tight(a *Analysis, t Table) bool {
	if t.Count == 0 {
		return false
	}
	lo := uint16(0xFFFF)
	for i := 0; i < t.Count; i++ {
		if w := a.word(t.Addr + uint16(2*i)); w < lo {
			lo = w
		}
	}
	return lo == t.Addr+uint16(2*t.Count)
}

// Gap is a run of bytes no code or declared data claims, with the
// instructions whose 16-bit value points into it.
type Gap struct {
	Lo, Hi uint16
	Refs   []Inst
}

// Gaps lists unclaimed ranges.
func (a *Analysis) Gaps() []Gap {
	var gaps []Gap
	for i := 0; i < len(a.Own); {
		if a.Own[i] != Unclaimed {
			i++
			continue
		}
		j := i
		for j < len(a.Own) && a.Own[j] == Unclaimed {
			j++
		}
		gaps = append(gaps, Gap{Lo: a.Base + uint16(i), Hi: a.Base + uint16(j-1)})
		i = j
	}
	addrs := a.sortedInsts()
	for gi := range gaps {
		g := &gaps[gi]
		for _, pc := range addrs {
			in := a.Insts[pc]
			if in.Kind != ValNone && in.Val >= g.Lo && in.Val <= g.Hi {
				g.Refs = append(g.Refs, in)
			}
		}
	}
	return gaps
}

func (a *Analysis) sortedInsts() []uint16 {
	addrs := make([]uint16, 0, len(a.Insts))
	for pc := range a.Insts {
		addrs = append(addrs, pc)
	}
	sort.Slice(addrs, func(i, j int) bool { return addrs[i] < addrs[j] })
	return addrs
}

// Stats counts bytes by ownership.
func (a *Analysis) Stats() (code, data, unclaimed int) {
	for _, o := range a.Own {
		switch o {
		case CodeStart, CodeBody:
			code++
		case Data:
			data++
		default:
			unclaimed++
		}
	}
	return
}

// Reach follows control flow from roots through analysed code (calls,
// jumps, fall-through and inline-table entries) and returns the call
// targets reached and the number of instructions reached.
func (a *Analysis) Reach(roots []uint16) (calls []uint16, insts int) {
	seen := map[uint16]bool{}
	callSet := map[uint16]bool{}
	tableAt := map[uint16]Table{}
	for _, t := range a.Tables {
		if t.Caller != 0 {
			tableAt[t.Caller] = t
		}
	}
	work := append([]uint16(nil), roots...)
	for _, r := range roots {
		callSet[r] = true
	}
	for len(work) > 0 {
		pc := work[len(work)-1]
		work = work[:len(work)-1]
		for {
			in, ok := a.Insts[pc]
			if !ok || seen[pc] {
				break
			}
			seen[pc] = true
			insts++
			if in.Flow == FlowCall || in.Flow == FlowCondCall {
				if a.inROM(in.Val) {
					callSet[in.Val] = true
					work = append(work, in.Val)
				}
				if t, ok := tableAt[pc]; ok {
					for i := 0; i < t.Count; i++ {
						e := a.word(t.Addr + uint16(2*i))
						callSet[e] = true
						work = append(work, e)
					}
					break
				}
				if in.Flow == FlowCall && a.Cfg.NoReturn[in.Val] {
					break
				}
			}
			if in.Flow == FlowJump || in.Flow == FlowCondJump {
				if a.inROM(in.Val) {
					work = append(work, in.Val)
				}
				if in.Flow == FlowJump {
					break
				}
			}
			if in.Flow == FlowRet || in.Flow == FlowIndirect {
				break
			}
			pc += uint16(in.Len)
		}
	}
	for c := range callSet {
		calls = append(calls, c)
	}
	sort.Slice(calls, func(i, j int) bool { return calls[i] < calls[j] })
	return calls, insts
}
