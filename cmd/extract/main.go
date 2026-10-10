// Command extract rebuilds every manifest entry from the user's ROM into
// assets/ (git-ignored). With -check it re-derives everything and diffs
// against the files on disk without writing.
//
//	go run ./cmd/extract
//	go run ./cmd/extract -check
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"go-msx-penguin/internal/manifest"
	"go-msx-penguin/internal/rle"
	"go-msx-penguin/internal/rom"
)

func main() {
	romPath := flag.String("rom", rom.DefaultPath, "ROM image")
	dir := flag.String("dir", "assets", "assets directory")
	checkOnly := flag.Bool("check", false, "verify the files on disk instead of writing")
	flag.Parse()

	data, err := rom.Load(*romPath)
	fail(err)
	rd := func(a uint16) byte { return data[a-rom.Base] }

	bad := 0
	for _, e := range manifest.Entries {
		out, err := derive(e, data, rd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", e.Name, err)
			bad++
			continue
		}
		path := filepath.Join(*dir, Path(e))
		if *checkOnly {
			have, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(have, out) {
				fmt.Fprintf(os.Stderr, "%s: %s differs from the ROM (%v)\n", e.Name, path, err)
				bad++
			}
			continue
		}
		fail(os.MkdirAll(filepath.Dir(path), 0o755))
		fail(os.WriteFile(path, out, 0o644))
	}
	if bad > 0 {
		fmt.Fprintf(os.Stderr, "extract: %d of %d entries failed\n", bad, len(manifest.Entries))
		os.Exit(1)
	}
	verb := "wrote"
	if *checkOnly {
		verb = "checked"
	}
	fmt.Printf("extract: %s %d entries\n", verb, len(manifest.Entries))
}

// Path is where an entry lives under the assets directory.
func Path(e manifest.Entry) string {
	if e.Kind == manifest.RLE {
		return filepath.Join("gfx", e.Name+".bin")
	}
	return filepath.Join("raw", e.Name+".bin")
}

func derive(e manifest.Entry, data []byte, rd rle.Reader) ([]byte, error) {
	switch e.Kind {
	case manifest.Raw:
		lo := int(e.Addr) - rom.Base
		if lo < 0 || lo+e.Len > len(data) {
			return nil, fmt.Errorf("range $%04X+%d outside the ROM", e.Addr, e.Len)
		}
		return append([]byte(nil), data[lo:lo+e.Len]...), nil
	case manifest.RLE:
		out, end, err := rle.Decode(rd, e.Addr, e.Mirror)
		if err != nil {
			return nil, err
		}
		if len(out) != e.Want || end != e.End {
			return nil, fmt.Errorf("decoded %d bytes ending $%04X, manifest says %d ending $%04X", len(out), end, e.Want, e.End)
		}
		return out, nil
	}
	return nil, fmt.Errorf("unknown kind %d", e.Kind)
}

func fail(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "extract:", err)
		os.Exit(1)
	}
}
