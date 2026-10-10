package z80dis

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParseConfig reads the analyst's notes. One directive per line, '#'
// starts a comment, addresses are written $XXXX:
//
//	entry $4010 [name]      code entry point
//	label $4045 name        name an address (also usable for RAM)
//	comment $4045 text...   comment shown in the listing
//	noreturn $XXXX          CALL $XXXX never returns
//	inline $XXXX [n]        each CALL $XXXX is followed by n DW code
//	                        pointers (omit n to grow heuristically)
//	table $XXXX n [name]    DW table of n code pointers
//	data $A-$B [note]       declared data range (inclusive)
func ParseConfig(r io.Reader) (*Config, error) {
	cfg := &Config{
		Labels:   map[uint16]string{},
		Comments: map[uint16]string{},
		NoReturn: map[uint16]bool{},
		Inline:   map[uint16]int{},
	}
	sc := bufio.NewScanner(r)
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		if i := strings.IndexByte(text, '#'); i >= 0 {
			text = text[:i]
		}
		f := strings.Fields(text)
		if len(f) == 0 {
			continue
		}
		bad := func(err error) error { return fmt.Errorf("config line %d: %v", line, err) }
		need := func(n int) error {
			if len(f) < n {
				return bad(fmt.Errorf("%s needs %d arguments", f[0], n-1))
			}
			return nil
		}
		if err := need(2); err != nil {
			return nil, err
		}
		if f[0] == "data" {
			lo, hi, ok := strings.Cut(f[1], "-")
			if !ok {
				hi = lo
			}
			l, err1 := parseAddr(lo)
			h, err2 := parseAddr(hi)
			if err1 != nil || err2 != nil || h < l {
				return nil, bad(fmt.Errorf("bad range %q", f[1]))
			}
			cfg.Data = append(cfg.Data, Range{Lo: l, Hi: h, Note: strings.Join(f[2:], " ")})
			continue
		}
		addr, err := parseAddr(f[1])
		if err != nil {
			return nil, bad(err)
		}
		switch f[0] {
		case "entry":
			cfg.Entries = append(cfg.Entries, addr)
			if len(f) > 2 {
				cfg.Labels[addr] = f[2]
			}
		case "label":
			if err := need(3); err != nil {
				return nil, err
			}
			cfg.Labels[addr] = f[2]
		case "comment":
			cfg.Comments[addr] = strings.Join(f[2:], " ")
		case "noreturn":
			cfg.NoReturn[addr] = true
		case "inline":
			n := 0
			if len(f) > 2 {
				if n, err = strconv.Atoi(f[2]); err != nil {
					return nil, bad(err)
				}
			}
			cfg.Inline[addr] = n
		case "table":
			if err := need(3); err != nil {
				return nil, err
			}
			n, err := strconv.Atoi(f[2])
			if err != nil {
				return nil, bad(err)
			}
			cfg.Tables = append(cfg.Tables, Table{Addr: addr, Count: n})
			if len(f) > 3 {
				cfg.Labels[addr] = f[3]
			}
		default:
			return nil, bad(fmt.Errorf("unknown directive %q", f[0]))
		}
	}
	return cfg, sc.Err()
}

func parseAddr(s string) (uint16, error) {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "$"), "0x")
	v, err := strconv.ParseUint(s, 16, 16)
	return uint16(v), err
}
