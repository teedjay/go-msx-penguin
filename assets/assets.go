// Package assets serves the ROM data the engine needs, extracted from the
// user's own ROM by cmd/extract (run it first: the .bin files are
// git-ignored, and this package does not build without them).
package assets

import (
	"embed"
	"fmt"
	"sort"

	"go-msx-penguin/internal/manifest"
)

//go:embed raw/*.bin gfx/*.bin
var files embed.FS

type span struct {
	lo   uint16
	data []byte
}

var raw []span // sorted by address

func init() {
	for _, e := range manifest.Entries {
		if e.Kind != manifest.Raw {
			continue
		}
		b, err := files.ReadFile("raw/" + e.Name + ".bin")
		if err != nil || len(b) != e.Len {
			panic(fmt.Sprintf("assets: %s missing or wrong size; run go run ./cmd/extract", e.Name))
		}
		raw = append(raw, span{e.Addr, b})
	}
	sort.Slice(raw, func(i, j int) bool { return raw[i].lo < raw[j].lo })
}

// ROMByte returns the ROM byte at a CPU address inside a Raw manifest
// entry, and panics for any other address: a read of something never
// extracted must fail loudly, not return a plausible zero.
func ROMByte(addr uint16) byte {
	i := sort.Search(len(raw), func(i int) bool { return raw[i].lo > addr }) - 1
	if i >= 0 {
		if off := int(addr) - int(raw[i].lo); off < len(raw[i].data) {
			return raw[i].data[off]
		}
	}
	panic(fmt.Sprintf("assets: ROM read at $%04X is outside every Raw manifest entry", addr))
}

// Decoded returns a decoded (RLE) manifest entry by name.
func Decoded(name string) []byte {
	b, err := files.ReadFile("gfx/" + name + ".bin")
	if err != nil {
		panic(fmt.Sprintf("assets: no decoded entry %q", name))
	}
	return b
}
