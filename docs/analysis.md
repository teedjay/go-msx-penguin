# ROM analysis

Append-only for corrections: add a dated correction note rather than
editing history away. Confidence: **High** (traced instruction by
instruction and/or confirmed by trace or render), **Medium** (control flow
clear, meaning inferred), **Low** (plausible guess).

## Open issues

- Joystick input is translated but not verified by a trace (openMSX's
  key joystick follows host key events, which a script cannot send).
- The frontend's tick pacing (`game.Frame`) estimates a tick's length
  from its VRAM traffic; load ticks can be 1-2 frames off. Real MSX BIOS
  routines may also take different times from C-BIOS.
- PSG model: the envelope generator is implemented but unverified (the
  game never writes R11-R13). Audio around the start of the game tune
  (after the 14-frame load in state 8) comes out about 6 frames later
  than the original's in the headless check; elsewhere it agrees.
- **Per-interrupt work is more than sound.** For state < `$0C` the IRQ
  handler runs `$4C98` (when `$E140`+`$E142` = 0), `$464C`, and updates
  `$E0FC` from bit 7 of `$E081`, *before* the busy check. Interrupts that
  arrive during a long tick run this work mid-tick in the original; the
  lockstep comparator can only run it between ticks. Expect possible
  differences on multi-frame ticks. (Medium)
- **Everything reachable is ported and verified, except joystick
  input.** Traces (see the verification log) cover the attract demo, the
  real-game start, keyboard play, falls, knocks (including while
  jumping), obstacles, fish, wind, time out, game over, stage clears with
  the bonus scene after the 5th, and stages 1-7. Joystick input (`$40A4`
  via PSG R14) is translated but no trace drives a joystick yet.
- INIT takes 2 interrupts under C-BIOS. Real MSX BIOS routines may give a
  different count; the frontend assumes 2. (Low)

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

The RAM shadow of the registers is at `$E038-$E03F`. The registers never
change after INIT.

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

Other formats (ported, `internal/game/screen.go`): `$4512` (name-table writer with `$E0+`
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

### Other decoder call sites (decoded at runtime, verified via VRAM)

`$5DAC`, `$6249`, `$6710`, `$54E1`, `$552F`, `$5538`, `$5555`, `$503D`;
text via `$4583` from `$57DE`, `$5793`, `$57D3`, `$5756`, `$5785`.

## Verification log

- openMSX probing note: in a `read_mem` watchpoint command,
  `$::wp_last_value` does not exist and the command fails silently, and
  instruction fetches also trigger it. Avoid Tcl globals named `::keys` or
  `::t`, which also broke a probe silently. Log `[reg PC]` and registers
  from breakpoints instead.

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

- 2026-10-11: keyboard play. Random schedules (`cmd/keygen`, seeds 1-4,
  8000-10000 ticks each) all match. Two more manifest gaps first showed
  as panics and were confirmed in openMSX as original behaviour: `$4D6B`
  reads its range table at `$4D9D` with indices far past its 5 entries
  (any RAM byte, via the `$4D90` quirk), and after a game over the demo
  input reader reads BIOS bytes at `$0000`. Random play never clears a
  stage, so `cmd/keysearch` searches inputs on the port; the schedules
  were then recorded on the original: 23101 ticks (4 clears) and 41821
  ticks (6 clears, bonus scene, stage 7) all match. `cmd/tracecov`
  reports what a set of traces exercises.

- 2026-10-11: audio. `cmd/play` applied the PSG registers from the audio
  thread once per Read, i.e. a few times a second: music garbled, short
  effects lost (reported by the user). Now each game frame renders its
  own 735 samples through `psg.Chip.Frame`. Checked against the
  original's audio recorded by openMSX (`scripts/refaudio.sh`) over 3000
  interrupts of keyboard play: on/off agrees in 2652 of 2663 sounding
  windows, and each side's strongest frequency is present in the other's
  spectrum in 2612 of 2652 (`cmd/audiocmp`).

## Resolved questions (2026-10-11)

- `$79EA` is the sound driver (ported, matched in every trace). Game
  logic reads its RAM: states 13 and 14 wait on channel 0's id (`$E012`)
  and state 14 on channel 1's counter (`$E01A`).
- There is no randomness. No `LD A,R`; object placement comes from the
  per-stage course tables, and the only alternation without input is the
  `$E185` counter in `$75C9`. Every trace matches with no entropy fed in.
- `$4EC2-$4ED5` holds the knocked-aside X offsets (`$4E3E`).
- The VDP registers never change after INIT: `WRTVDP` is only called from
  `$44A3`.
- All graphics streams are decoded at runtime by the port from Raw
  manifest entries, as the original does, and checked through VRAM by the
  comparator. Only the common set has decoded manifest entries, for
  inspection.

## Corrections

- 2026-10-11: `$E100` is a speed *period* (lower is faster): up
  accelerates, down brakes. The comments had it backwards; the code was
  right (the comparator matched either way).

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
- `$4D6B` indexes its 5-entry range table at `$4D9D` with object kinds up
  to 6, and through the `$4D90` quirk with any RAM byte, reading code as
  ranges anywhere up to `$4F9C`. Confirmed with the oracle (2026-10-11).
- After a game over, the demo input reader runs with `$E0EC` = 0 and
  reads the BIOS at `$0000` (`$F3`, `DI`). The port maps those two bytes.

## Dead code

`$4401`, `$4442`, `$48AE`: no CALL/JP and no 16-bit word in the ROM points
at them.
