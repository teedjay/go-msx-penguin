// Package rom loads the original cartridge image and checks that it is the
// dump every address in this project refers to.
package rom

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
)

const (
	// Base is the CPU address the 16KB cartridge is mapped at.
	Base = 0x4000
	// Size is the cartridge size in bytes.
	Size = 0x4000
	// DefaultPath is the file name a user must supply, in the repo root.
	DefaultPath = "Antarctic Adventure (1984) Konami [RC-701] [9196].rom"
	// SHA1 of the known-good dump. CRC32 6ac017b9, MD5
	// 31620f841985e617f0b887684873ac3e.
	SHA1 = "027b653a5c7b91256a124a2accdad2478b10770d"
)

// Load reads the ROM image. A size mismatch is an error; a checksum
// mismatch is a warning on stderr, since addresses will then be wrong but
// the tools may still be useful for comparison.
func Load(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) != Size {
		return nil, fmt.Errorf("%s: %d bytes, want %d", path, len(b), Size)
	}
	if sum := sha1.Sum(b); hex.EncodeToString(sum[:]) != SHA1 {
		fmt.Fprintf(os.Stderr, "warning: %s: SHA-1 %x does not match the known dump %s; addresses in this project may not apply\n", path, sum, SHA1)
	}
	return b, nil
}
