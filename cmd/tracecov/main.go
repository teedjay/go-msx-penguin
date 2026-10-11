// Command tracecov reports which states and gameplay events a set of
// reference traces exercises, so untested paths can be listed as open
// issues.
//
//	go run ./cmd/tracecov /tmp/attract.trace /tmp/play*.trace
package main

import (
	"fmt"
	"io"
	"os"
	"sort"

	"go-msx-penguin/internal/reftrace"
)

// events are RAM conditions worth seeing at least once (Medium: roles
// as documented in docs/analysis.md).
var events = []struct {
	name string
	hit  func(r *reftrace.Record) bool
}{
	{"live play ($E002 bit 6)", func(r *reftrace.Record) bool { return r.RAM[0x002]&0x40 != 0 }},
	{"jumping ($E0F9)", func(r *reftrace.Record) bool { return r.RAM[0x0F9] != 0 }},
	{"in a hole ($E140)", func(r *reftrace.Record) bool { return r.RAM[0x140] != 0 }},
	{"knocked aside ($E142)", func(r *reftrace.Record) bool { return r.RAM[0x142] != 0 }},
	{"knocked while jumping ($E142 = 3)", func(r *reftrace.Record) bool { return r.RAM[0x142] == 3 }},
	{"wind ($E196)", func(r *reftrace.Record) bool { return r.RAM[0x196] != 0 }},
	{"obstacle moving ($E183 & 3)", func(r *reftrace.Record) bool { return r.RAM[0x183]&3 != 0 }},
	{"big obstacle ($E183 & $E0)", func(r *reftrace.Record) bool { return r.RAM[0x183]&0xE0 != 0 }},
	{"fish sprites ($E14A-$E14D)", func(r *reftrace.Record) bool {
		return r.RAM[0x14A]|r.RAM[0x14B]|r.RAM[0x14C]|r.RAM[0x14D] != 0
	}},
	{"score > 0", func(r *reftrace.Record) bool { return r.RAM[0x043]|r.RAM[0x044]|r.RAM[0x045] != 0 }},
	{"stage 2 or later ($E0E1)", func(r *reftrace.Record) bool { return r.RAM[0x0E1] != 0 }},
	{"time out ($E00C)", func(r *reftrace.Record) bool { return r.RAM[0x00C] != 0 }},
}

func main() {
	states := map[[2]byte]int{}
	hits := make([]int, len(events))
	total := 0
	for _, path := range os.Args[1:] {
		r, err := reftrace.Open(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		var rec reftrace.Record
		for r.Next(&rec) != io.EOF {
			total++
			states[[2]byte{rec.RAM[0], rec.RAM[1]}]++
			for i, e := range events {
				if e.hit(&rec) {
					hits[i]++
				}
			}
		}
		r.Close()
	}
	fmt.Printf("%d ticks\n\nstate/sub   ticks\n", total)
	keys := make([][2]byte, 0, len(states))
	for k := range states {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i][0] < keys[j][0] || keys[i][0] == keys[j][0] && keys[i][1] < keys[j][1]
	})
	for _, k := range keys {
		fmt.Printf("  %2d/%d    %6d\n", k[0], k[1], states[k])
	}
	fmt.Println("\nevents (ticks)")
	for i, e := range events {
		mark := " "
		if hits[i] == 0 {
			mark = "!"
		}
		fmt.Printf(" %s %-36s %6d\n", mark, e.name, hits[i])
	}
}
