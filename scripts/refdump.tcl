# Records the original's state at the start of every game tick, for
# cmd/refcompare. Runs inside openMSX as an offline reference only.
#
#   REFDUMP_OUT=/tmp/attract.trace REFDUMP_TICKS=2000 \
#   [REFDUMP_KEYS=/tmp/play.keys] \
#   openmsx -machine C-BIOS_MSX1_JP -cart <rom> -script scripts/refdump.tcl
#
# Tick start is $4083 in the interrupt handler: after the busy flag at
# $E005 is set and before the input reader ($40A4) runs, so scheduled keys
# applied here are seen by this tick. Interrupts are counted at the
# H.KEYI handler entry $4045.
#
# File starts with a header written at INIT's final EI ($4042): "INIT",
# u32 interrupts taken during INIT (C-BIOS routines re-enable them).
# Then one record per tick (little endian): "TICK", u32 tick, u32 irqs so far,
#   u32 n, 64 x u32 offsets, 8 VDP regs, 16 PSG regs, RAM $E000-$E7FF,
#   VRAM $0000-$3FFF.
# The offsets place each interrupt taken since the previous tick start
# (the last one starts this tick): the number of VRAM data writes (OUT to
# port $98) the previous tick had made when it arrived. VRAM sequences run
# with interrupts disabled, so this pins every nested interrupt between
# the right two writes.
# Keys file: one "tick:row:mask:down|up" per line, '#' comments.

set renderer none
set throttle off
set mute on

set ::rd_out [open $::env(REFDUMP_OUT) w]
fconfigure $::rd_out -translation binary
set ::rd_max $::env(REFDUMP_TICKS)
set ::rd_tick 0
set ::rd_irqs 0
set ::rd_keys [dict create]
set ::rd_vw 0
set ::rd_offs {}

if {[info exists ::env(REFDUMP_KEYS)] && $::env(REFDUMP_KEYS) ne ""} {
    set f [open $::env(REFDUMP_KEYS) r]
    foreach line [split [read $f] "\n"] {
        set line [string trim [lindex [split $line "#"] 0]]
        if {$line eq ""} continue
        lassign [split $line ":"] t row mask dir
        dict lappend ::rd_keys $t [list $row $mask $dir]
    }
    close $f
}

proc rd_irq {} {
    incr ::rd_irqs
    lappend ::rd_offs $::rd_vw
}

proc rd_vwrite {} { incr ::rd_vw }

proc rd_tick {} {
    if {[dict exists $::rd_keys $::rd_tick]} {
        foreach k [dict get $::rd_keys $::rd_tick] {
            lassign $k row mask dir
            if {$dir eq "down"} { keymatrixdown $row $mask } else { keymatrixup $row $mask }
        }
    }
    set n [llength $::rd_offs]
    if {$n > 64} { error "refdump: $n interrupts in one tick (max 64)" }
    set offs $::rd_offs
    while {[llength $offs] < 64} { lappend offs 0 }
    puts -nonewline $::rd_out [binary format a4iii TICK $::rd_tick $::rd_irqs $n]
    puts -nonewline $::rd_out [binary format i64 $offs]
    set ::rd_offs {}
    puts -nonewline $::rd_out [debug read_block "VDP regs" 0 8]
    puts -nonewline $::rd_out [debug read_block "PSG regs" 0 16]
    puts -nonewline $::rd_out [debug read_block memory 0xE000 0x800]
    puts -nonewline $::rd_out [debug read_block VRAM 0 0x4000]
    set ::rd_vw 0
    incr ::rd_tick
    if {$::rd_tick >= $::rd_max} {
        close $::rd_out
        exit
    }
}

proc rd_init {} {
    puts -nonewline $::rd_out [binary format a4i INIT $::rd_irqs]
    set ::rd_offs {}
}

debug set_watchpoint write_io 0x98 {} rd_vwrite
debug set_bp 0x4042 {} rd_init
debug set_bp 0x4045 {} rd_irq
debug set_bp 0x4083 {} rd_tick
