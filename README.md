# go-msx-penguin

A native Go reimplementation of a 16KB MSX1 cartridge game (a penguin
racing across the ice). Each routine of the original was read and
rewritten as ordinary Go against a small hardware-abstraction layer. No
Z80 code runs at runtime: this is a port, not an emulator.

Status: complete and playable. Every routine is ported, and the port
matches the original byte for byte (RAM, VRAM, video and sound chip
registers, every tick) over the attract demo and recorded keyboard games
through stage 7. See `docs/analysis.md` for what has been verified and
`CLAUDE.md` for how the work is done.

## Requirements

- Go 1.27 or newer
- Your own copy of the original cartridge ROM, saved in the repository root
  as `Antarctic Adventure (1984) Konami [RC-701] [9196].rom`
  (16384 bytes, SHA-1 `027b653a5c7b91256a124a2accdad2478b10770d`).
- Optional, for verification only: openMSX with the bundled C-BIOS machines.

## Building

```bash
go run ./cmd/extract          # rebuild assets/ from your ROM (git-ignored)
go build ./... && go test ./...
```

The `assets` package embeds the extracted files, so run `cmd/extract`
before building anything else.

## Playing

```bash
go run ./cmd/play
```

On the title, press **1** for joystick (a gamepad) or **2** for
keyboard. Keyboard: **up** accelerates, **down** brakes, **left/right**
steer, **space** jumps. **Esc** quits. **Level select:** on the title screen press a letter before starting,
**A** for level 1 (normal) up to **Z** for level 26 (the ten courses
repeat from level 11). The choice shows in the top right corner.
`-level N` preselects it from the command line.

**Extra time:** press **T** during a game to add 100 to the time.

`-record file.keys` saves your
key presses in a form that can be replayed into the original (see
`CLAUDE.md`) to check the port against it; a game started with the
level select cannot be replayed into the original.

## Legal

This is an unofficial, non-commercial project, not affiliated with or
endorsed by the rights holder of the original game.

- **Distributed:** original Go source, tools, documentation, and
  screenshots of this engine running.
- **Not distributed:** the ROM, and any data extracted or decoded from it
  (graphics, sound, text, tables, disassembly listings, reference traces).
  The extraction tool rebuilds those locally from your own ROM; they are
  excluded from version control.
- The engine executes no original code.

Rights holders with concerns are invited to open an issue. See `NOTICE`.
