// Command disasm maps the original ROM by recursive descent and writes the
// annotated listing (docs/listing.asm, git-ignored: it is ROM-derived).
//
//	go run ./cmd/disasm                 # write the listing, print stats
//	go run ./cmd/disasm -gaps           # unclaimed ranges and who references them
//	go run ./cmd/disasm -peek 4045-40A3 # linear decode, claims nothing
//	go run ./cmd/disasm -reach 4010,4045 # routines reachable from roots
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go-msx-penguin/internal/rom"
	"go-msx-penguin/internal/z80dis"
)

func main() {
	romPath := flag.String("rom", rom.DefaultPath, "ROM image")
	cfgPath := flag.String("cfg", "docs/disasm.cfg", "analysis config")
	out := flag.String("o", "docs/listing.asm", "listing output")
	gaps := flag.Bool("gaps", false, "report unclaimed ranges with the instructions that reference into each")
	peek := flag.String("peek", "", "linear decode of range A-B (hex), without claiming")
	reach := flag.String("reach", "", "comma-separated roots (hex): list the routines reachable from them")
	flag.Parse()

	data, err := rom.Load(*romPath)
	check(err)
	f, err := os.Open(*cfgPath)
	check(err)
	cfg, err := z80dis.ParseConfig(f)
	f.Close()
	check(err)

	a := z80dis.Analyze(data, rom.Base, cfg)

	if *peek != "" {
		lo, hi, ok := strings.Cut(*peek, "-")
		if !ok {
			hi = lo
		}
		l, err1 := strconv.ParseUint(strings.TrimPrefix(lo, "$"), 16, 16)
		h, err2 := strconv.ParseUint(strings.TrimPrefix(hi, "$"), 16, 16)
		if err1 != nil || err2 != nil {
			check(fmt.Errorf("bad -peek range %q", *peek))
		}
		z80dis.Peek(os.Stdout, data, rom.Base, uint16(l), uint16(h), a.Name)
		return
	}

	if *reach != "" {
		var roots []uint16
		for _, f := range strings.Split(*reach, ",") {
			v, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimSpace(f), "$"), 16, 16)
			check(err)
			roots = append(roots, uint16(v))
		}
		calls, n := a.Reach(roots)
		for _, c := range calls {
			fmt.Printf("$%04X %s\n", c, a.Label(c))
		}
		fmt.Printf("%d routines, %d instructions\n", len(calls), n)
		return
	}

	if *gaps {
		for _, g := range a.Gaps() {
			fmt.Printf("$%04X-$%04X (%d bytes)\n", g.Lo, g.Hi, int(g.Hi)-int(g.Lo)+1)
			for _, in := range g.Refs {
				fmt.Printf("    from $%04X: %s\n", in.Addr, in.Format(a.Name))
			}
		}
		return
	}

	w, err := os.Create(*out)
	check(err)
	check(a.WriteListing(w))
	check(w.Close())

	code, dat, un := a.Stats()
	fmt.Printf("code %d bytes, data %d, unclaimed %d; %d instructions, %d tables\n",
		code, dat, un, len(a.Insts), len(a.Tables))
	for _, t := range a.Tables {
		if t.Heuristic {
			fmt.Printf("table $%04X (CALL at $%04X): %d entries, heuristic, tight=%v\n", t.Addr, t.Caller, t.Count, t.Tight)
		}
	}
	for _, c := range a.Conflicts {
		fmt.Println("conflict:", c)
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "disasm:", err)
		os.Exit(1)
	}
}
