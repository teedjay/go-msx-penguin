// Package manifest lists every piece of ROM data the port uses, by address
// and method. It holds addresses only, never ROM bytes: cmd/extract
// rebuilds the data from the user's own ROM.
package manifest

// Kind is how an entry is produced from the ROM.
type Kind int

const (
	// Raw is a byte range copied as is. The engine reads ROM data only
	// through Raw entries (assets.ROMByte), with the original's addresses.
	Raw Kind = iota
	// RLE is a stream decoded by internal/rle. Decoded output must have
	// exactly Want bytes and the stream must end at End.
	RLE
)

// Entry is one manifest item.
type Entry struct {
	Name string
	Kind Kind
	Addr uint16 // first ROM byte
	Len  int    // Raw: byte count
	// RLE only.
	Mirror bool
	Want   int    // decoded size
	End    uint16 // address just past the terminator
	Note   string // what it is and how sure we are
}

// Entries is the manifest. Raw entries are the data gaps the code
// references (cmd/disasm -gaps), plus code bytes the original reads as
// data. Roles are as the ported routines use them.
var Entries = []Entry{
	{Name: "data_4471", Kind: Raw, Addr: 0x4471, Len: 9, Note: "new-game values for $E0E0-$E0E8 ($4443) (High: ported, verified by traces)"},
	{Name: "vdp_regs", Kind: Raw, Addr: 0x44BF, Len: 8, Note: "R0-R7 initial values (High)"},
	{Name: "data_4762", Kind: Raw, Addr: 0x4762, Len: 0x78, Note: "per-stage course parameters: $E18A bytes and object-table pointers ($4720) (High: ported, verified by traces)"},
	{Name: "data_4814", Kind: Raw, Addr: 0x4814, Len: 10, Note: "per-stage hold lengths ($47EC) (High: ported, verified by traces)"},
	{Name: "data_49D6", Kind: Raw, Addr: 0x49D6, Len: 0x100, Note: "stage-map text list ($4912), route steps at $4A85 ($4972), per-stage distance/time at $4AAE ($423B) (High: ported, verified by traces)"},
	{Name: "data_4B59", Kind: Raw, Addr: 0x4B59, Len: 40, Note: "penguin sprite pattern sets ($4BAA) (High: ported, verified by traces)"},
	{Name: "data_4C55", Kind: Raw, Addr: 0x4C55, Len: 12, Note: "jump arc Y offsets ($4BF3, $4ED6) (High: ported, verified by traces)"},
	{Name: "data_4CFF", Kind: Raw, Addr: 0x4CFF, Len: 31, Note: "shadow X offsets, indexed from $4CFE/$4D08 ($4CBE) (High: ported, verified by traces)"},
	// $4D6B's range table at $4D9D is indexed by a RAM byte that its
	// pointer quirk ($4D90) can pick from anywhere in RAM (confirmed in
	// openMSX: tick 23676 of a searched schedule reads $4DAF via a fish
	// counter at $E14A), so every byte up to $4D9D + 2*255 + 1 can be read.
	// The entry also covers the knock offsets at $4EC2-$4ED5.
	{Name: "scan_4D9D", Kind: Raw, Addr: 0x4D9D, Len: 0x4F9D - 0x4D9D, Note: "range table for $4D6B plus all code its quirky index can reach; includes the $4EC2 knock offsets (High)"},
	{Name: "data_5119", Kind: Raw, Addr: 0x5119, Len: 54, Note: "per-stage course section lists ($5096) and stage flags at $5145 ($5019) (High: ported, verified by traces)"},
	{Name: "data_527B", Kind: Raw, Addr: 0x527B, Len: 42, Note: "6-byte object records per kind ($5262) (High: ported, verified by traces)"},
	{Name: "data_5391", Kind: Raw, Addr: 0x5391, Len: 224, Note: "wind schedule ($52A5) and wind texts from $53B2 ($52EF) (High: ported, verified by traces)"},
	{Name: "data_54FC", Kind: Raw, Addr: 0x54FC, Len: 51, Note: "station flag name-table streams ($54CF, $54E1) (High: ported, verified by traces)"},
	{Name: "data_5589", Kind: Raw, Addr: 0x5589, Len: 683, Note: "station names ($552F), station RLE pointers at $5602, status-line and title texts, demo input from $57F4 (High: ported, verified by traces)"},
	{Name: "demo_tail_5834", Kind: Raw, Addr: 0x5834, Len: 2, Note: "the first two code bytes of $5834, which the demo input reader consumes after its table at $57F4 runs out (trace: $E0EC reaches $5836) (High)"},
	{Name: "gfx_5885", Kind: Raw, Addr: 0x5885, Len: 0x5DA9 - 0x5885, Note: "RLE streams, common graphics (High)"},
	{Name: "data_5DA9", Kind: Raw, Addr: 0x5DA9, Len: 3, Note: "stage screen RLE stream ($5019) (High: ported, verified by traces)"},
	{Name: "gfx_5DDC", Kind: Raw, Addr: 0x5DDC, Len: 1133, Note: "stage graphics RLE streams ($5DAC, $5019) (High: ported, verified by traces)"},
	{Name: "gfx_627F", Kind: Raw, Addr: 0x627F, Len: 1021, Note: "stage graphics RLE streams ($6249, $552F) (High: ported, verified by traces)"},
	{Name: "data_66A9", Kind: Raw, Addr: 0x66A9, Len: 103, Note: "sprite lists ($667C, $6681, $4FB7) (High: ported, verified by traces)"},
	{Name: "gfx_6716", Kind: Raw, Addr: 0x6716, Len: 3517, Note: "stage graphics RLE streams ($6710, $54E1) and course data ($5019, $5096) (High: ported, verified by traces)"},
	{Name: "data_7519", Kind: Raw, Addr: 0x7519, Len: 144, Note: "scenery name-table streams ($74D3) (High: ported, verified by traces)"},
	{Name: "data_77F0", Kind: Raw, Addr: 0x77F0, Len: 12, Note: "fish X steps and start positions ($775F) (High: ported, verified by traces)"},
	{Name: "data_787B", Kind: Raw, Addr: 0x787B, Len: 264, Note: "big obstacle sprite tables ($77FC) (High: ported, verified by traces)"},
	{Name: "data_7AF1", Kind: Raw, Addr: 0x7AF1, Len: 0x7E47 - 0x7AF1, Note: "sound data; its last $FF (stop) at $7E46 is also the first byte of the fill (found by the ROMByte panic)"},

	// Common graphics loaded by $5834 into each Screen 2 third.
	{Name: "common_colour_5D6A", Kind: RLE, Addr: 0x5D6A, Want: 688, End: 0x5DA9, Note: "colour rows after the $F0 fill (High)"},
	{Name: "common_colour_5DA0", Kind: RLE, Addr: 0x5DA0, Want: 16, End: 0x5DA9, Note: "tail of $5D6A, written 22 times (High)"},
	{Name: "common_patterns_5885", Kind: RLE, Addr: 0x5885, Want: 1312, End: 0x5C37, Note: "patterns, part 1 (High)"},
	{Name: "common_patterns_5C15", Kind: RLE, Addr: 0x5C15, Want: 104, End: 0x5C37, Note: "tail of $5885, written again (High)"},
	{Name: "common_patterns_5C37", Kind: RLE, Addr: 0x5C37, Want: 376, End: 0x5D6A, Note: "patterns, part 3; ends tight at $5D6A (High)"},
}
