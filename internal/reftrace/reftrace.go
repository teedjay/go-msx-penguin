// Package reftrace reads the per-tick state records written by
// scripts/refdump.tcl from the original running in openMSX. Traces are
// ROM-derived: keep them outside the repo.
package reftrace

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const (
	RAMBase  = 0xE000
	RAMSize  = 0x800
	VRAMSize = 0x4000
	recSize  = 4 + 4 + 4 + 8 + 16 + RAMSize + VRAMSize
)

// Record is the original's state at the start of one game tick.
type Record struct {
	Tick uint32
	IRQs uint32 // interrupts taken since boot, at this tick start
	Regs [8]byte
	PSG  [16]byte
	RAM  [RAMSize]byte // $E000-$E7FF
	VRAM [VRAMSize]byte
}

// Reader streams records from a trace file.
type Reader struct {
	f  *os.File
	br *bufio.Reader
}

func Open(path string) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &Reader{f: f, br: bufio.NewReaderSize(f, recSize*4)}, nil
}

func (r *Reader) Close() error { return r.f.Close() }

// Next reads the next record; io.EOF at a clean end.
func (r *Reader) Next(rec *Record) error {
	var buf [recSize]byte
	if _, err := io.ReadFull(r.br, buf[:]); err != nil {
		if err == io.ErrUnexpectedEOF {
			return fmt.Errorf("reftrace: truncated record")
		}
		return err
	}
	if string(buf[:4]) != "TICK" {
		return fmt.Errorf("reftrace: bad record magic %q", buf[:4])
	}
	rec.Tick = binary.LittleEndian.Uint32(buf[4:])
	rec.IRQs = binary.LittleEndian.Uint32(buf[8:])
	o := 12
	o += copy(rec.Regs[:], buf[o:])
	o += copy(rec.PSG[:], buf[o:])
	o += copy(rec.RAM[:], buf[o:])
	copy(rec.VRAM[:], buf[o:])
	return nil
}

// ReadAt seeks to record n and reads it.
func (r *Reader) ReadAt(n int, rec *Record) error {
	if _, err := r.f.Seek(int64(n)*recSize, io.SeekStart); err != nil {
		return err
	}
	r.br.Reset(r.f)
	return r.Next(rec)
}
