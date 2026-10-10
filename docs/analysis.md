# ROM analysis

Append-only for corrections: add a dated correction note rather than
editing history away. Confidence: **High** (traced instruction by
instruction and/or confirmed by trace or render), **Medium** (control flow
clear, meaning inferred), **Low** (plausible guess).

## Open issues

- **Tick pacing.** Load ticks take 14 frames in the original (8 such ticks
  in a 4000-tick attract trace, all other ticks 1 frame). Decide how the
  playable frontend models this; the comparator handles it already by
  replaying the interrupt counts.
- **Per-interrupt work is more than sound.** For state < `$0C` the IRQ
  handler runs `$4C98` (when `$E140`+`$E142` = 0), `$464C`, and updates
  `$E0FC` from bit 7 of `$E081`, *before* the busy check. Interrupts that
  arrive during a long tick run this work mid-tick in the original; the
  lockstep comparator can only run it between ticks. Expect possible
  differences on multi-frame ticks. (Medium)
- **Everything reachable is ported** (3121 instructions). Verified by
  the attract trace: boot, states 1-7, 9, 10, 11 (demo branch), 15, the
  demo game, sound, per-interrupt work. **Translated but not yet verified
  by any trace:** live play (state 11's play branch, joystick and keyboard
  input), the real-game start (state 8), time out (12, 13), stage clear
  (14), the fish sprites (`$775F`, live play only), and any path the demo
  does not happen to reach (falls, knocks, wind, fish pickup, scoring).
  Needs scripted-input traces (`-keys`) and a human playing.
- INIT takes 2 interrupts under C-BIOS. Real MSX BIOS routines may give a
  different count; the frontend assumes 2. (Low)
- `$79EA` is presumed to be the sound driver (per interrupt, when state ≠
  0). Unconfirmed. Find out whether game logic reads its RAM.
- No `LD A,R` in the ROM (byte scan and listing). Find what the game uses
  for randomness, if anything.
- `$4EC2-$4ED5` is declared data in `docs/disasm.cfg` without having been
  traced.
- Graphics streams other than the title/common set (`$5834`) are listed
  below but not yet decoded and checked.

## The ROM

| | |
| --- | --- |
| File | `Antarctic Adventure (1984) Konami [RC-701] [9196].rom` |
| Size | 16384 bytes, plain 16KB cartridge at `$4000-$7FFF` |
| CRC32 | `6ac017b9` |
| MD5 | `31620f841985e617f0b887684873ac3e` |
| SHA-1 | `027b653a5c7b91256a124a2accdad2478b10770d` |
| SHA-256 | `f1d1e7783e04b0193c986c56143850ee8c7bd766c382383b860f4785f032fb7b` |
| Last non-fill byte | `$7E45` |

`internal/rom.Load` warns when the SHA-1 does not match.

## Boot and frame structure (High unless marked)

- Header `AB`, INIT `$4010`.
- INIT: `DI`, `IM 1`, `JP $4045` into `H.KEYI` (`$FD9A`), `SP=$E400`,
  clear `$E000-$E7FF`, busy flag `$E005`=1, `CALL $447A` (VDP registers
  from `$44BF`, PSG mixer `R7=$B8`, `R15=$8F`, sound init `$449D`, clear all
  16KB VRAM), clear busy, state `$E000`=1, ack VDP, `EI`, `JR $`.
- Everything runs in the interrupt. `$4045`: push AF/BC/DE/HL, `DI`, ack
  VDP; if state ≠ 0 `CALL $79EA`; if state < `$0C` the per-interrupt work
  above; then if bit 0 of `$E005` is clear: set it, `EI`, `CALL $40A4`
  (input), `CALL $4107` (tick), `DI`, clear it. Otherwise return at once.
- `$4107` tick: `INC ($E003)` (frame counter), `CALL $4413`, dispatch on
  state `$E000` through the `$409A` trampoline (table at `$4114`, 16
  entries, tight).
- `$409A` trampoline: `ADD A,A`, `POP HL` (the inline table after the
  CALL), `HL += A`, load word, `JP (HL)`. Six inline tables in the ROM, all
  tight: `$4114`(16) `$4186`(3) `$41D5`(4) `$42F8`(8) `$48E3`(7) `$76ED`(4).
- RAM used: `$E000-$E1FF` (named operands), stack below `$E400`.

## Input (`$40A4`, Medium)

Only runs in state ≥ 7. New input goes to `$E009`, previous to `$E008`.
`$E002` bit 6 clear: demo input, one byte from the pointer at `$E0EC` every
32 frames (`$E0EB` counter), else the low nibble of the last input is
held. Bit 6 set and bit 4 clear: joystick via PSG R14 (`CPL`, `AND $3F`).
Bit 4 set: keyboard via `SNSMAT` rows 7 and 8, packed into the same
bit layout.

## State machine (`$E000`)

| State | Handler | Seen in attract | Notes |
| --- | --- | --- | --- |
| 0 | `$4134` | | `RET` (idle) |
| 1 | `$4135` | tick 0 (14 frames) | loads common graphics (`$5834`), `$E00A=$11` |
| 2 | `$4146` | ticks 1-34 | |
| 3 | `$4158` | ticks 35-114 | logo screen ("VIDEO CARTRIDGE") |
| 4 | `$4163` | ticks 115-167 | |
| 5 | `$4171` | ticks 168-423 | title, PLAY SELECT |
| 6 | `$4179` | ticks 424-456 | |
| 7 | `$4180` | 457, 692-2576 | gameplay (demo here); sub-state `$E001` |
| 8 | `$41CF` | | |
| 9 | `$423B` | | |
| 10 | `$427D` | 658-690 | |
| 11 | `$4296` | 691 | |
| 12 | `$42BC` | | |
| 13 | `$42E1` | | |
| 14 | `$42F2` | | |
| 15 | `$48DD` | 458-657 | |

Attract sequence: 1, 2, 3, 4, 5, 6, 7, 15, 10, 11, 7 (demo), then back to
1 at tick 2577.

## VRAM map (High: derived from the `$44BF` table, confirmed by rendering)

R0-R7 = `02 E2 0E 7F 07 76 03 E1`: Screen 2, 16x16 sprites, display and
IRQ on, backdrop 1.

| Table | Address |
| --- | --- |
| Colour | `$0000` (3 x `$800`) |
| Sprite patterns | `$1800` |
| Patterns | `$2000` (3 x `$800`) |
| Name table | `$3800` |
| Sprite attributes | `$3B00` |

The RAM shadow of the registers is at `$E038-$E03F`. Not yet known whether
any register changes later.

VRAM helpers: `$48A5` writes A to VRAM[DE]; `$48B7` / `$48C6` set up
write / read. **The port bytes are used swapped** relative to the BIOS
documentation: the write path takes its port from `$0006`, the read path
from `$0007`. Both are `$98` on real machines. Callers pass VRAM addresses
with bit 14 set (`$6000` for `$2000`); the routines set and clear it.

## Compression (High)

`$454C`, entries `$453F` (read destination word from the stream), `$4543`
(normal), `$4547` (mirrored), `$454C` (continue with the caller's address
and mode in C). Control byte `$00` ends; `$01-$7F` repeats the next byte n
times (the `DJNZ` targets the `OUT`); `$80-$FF` copies n = low 7 bits
literal bytes (the `DJNZ` targets the read; `$80` copies 256). Mirror mode
bit-reverses every byte read (`$4573`). Ported in `internal/rle`.

Streams can **share tails**: `$5DA0` is a suffix of the `$5D6A` stream,
`$5C15` of `$5885`; both decode to the same terminator.

Other formats, not yet traced: `$4512` (name-table writer with `$E0+`
position codes), `$44F1` (fill-based), `$4583` (text: destination word,
bytes until `$FF`, `$FE` = new destination).

## Assets

### Common graphics (`$5834`, High)

Per Screen 2 third (DE = `$0000`, `$0800`, `$1000`):

- Colour: 16 tiles x 8 bytes of value 0..15 (solid background colours),
  `$270` bytes of `$F0`, the `$5D6A` stream (688 bytes), then the `$5DA0`
  stream 22 times (16 bytes each). Total `$700`.
- Patterns at `$2000+DE`: `$5885` stream (1312), `$5C15` stream (104),
  then the stream that follows its terminator, `$5C37-$5D69` (376). Total
  `$700`, ending tight at `$5D6A`.

### Other decoder call sites (not yet decoded)

`$5DAC`, `$6249`, `$6710`, `$54E1`, `$552F`, `$5538`, `$5555`, `$503D`;
text via `$4583` from `$57DE`, `$5793`, `$57D3`, `$5756`, `$5785`.

## Verification log

- 2026-10-10: attract trace, 4000 ticks from power-on (C-BIOS_MSX1_JP).
  Rendering ticks 50, 200 and 1000 through `internal/msxrender` gave the
  logo, the title screen and demo gameplay with correct text and sprites.
- 2026-10-10: the five common-graphics RLE manifest entries, assembled as
  `$5834` writes them, equal the original's VRAM after the state 1 load
  (trace record 1) in all three thirds: colour `$2F0-$6FF` (1040 bytes)
  and patterns `$2000-$26FF` (1792 bytes). `extract -check` shown to fail
  on a one-byte change.

- 2026-10-10: `cmd/refcompare` on the 4000-tick attract trace: **ticks
  0-457 match** in RAM (bar the stack, `$E380-$E3FF`), all VRAM, VDP
  R0-R7 and PSG R0-R13/R15. Tick 458 is the first in state 7 (unported).
  The comparator was shown to fail: one flipped RAM byte at tick 200 and
  one flipped VRAM byte at tick 300 were each reported at that tick and
  address.

- 2026-10-10: after porting states 7-15: **all 4000 ticks of the attract
  trace match**, including the 1885-tick demo game and the return to
  state 1. Two manifest gaps showed up first as `ROMByte` panics, both
  genuine original behaviour: the last sound's `$FF` stop byte is `$7E46`,
  the first byte of the fill; and the demo input reader runs past its
  table into the code bytes at `$5834-$5835` (the trace shows `$E0EC`
  reaching `$5836`). Comparator re-proven at tick 1500 (demo play).

## Corrections

- 2026-10-10: `$49B6-$49C8` was listed as data. It is four 4-byte code
  stubs reached by `JP (HL)` at `$49B5` (cursor moves for the stage map
  route). Now disassembler entries; removed from the manifest.
- 2026-10-10: `$7E46` is not only fill: it is the stop byte of the last
  sound. The `data_7AF1` entry now includes it.

## Original quirks kept

- `$4D90`: on a miss, the jump-collision scan advances by the leftover
  comparison result instead of the 5 bytes left in the object record.
- `$4E50`: the knocked-aside offsets table is addressed from `$4EC1`, a
  code byte; the index starts at 1, so the code byte is never read.
- Demo input reads two code bytes (`$5834-$5835`) at the end of the demo.

## Dead code

`$4401`, `$4442`, `$48AE`: no CALL/JP and no 16-bit word in the ROM points
at them.
