# Records the original's audio in openMSX, for cmd/audiocmp. Recording
# starts at the first interrupt (INIT's) and stops after REC_IRQS, with an
# optional key schedule applied at tick starts like scripts/refdump.tcl.
#
#   scripts/refaudio.sh OUT.wav IRQS [KEYS]
#
# Tcl globals are prefixed: names like ::keys or ::t break silently.
set renderer none
set throttle off
set ::rc_n 0
set ::rc_tick 0
set ::rc_keys [dict create]
if {[info exists ::env(REC_KEYS)]} {
    set f [open $::env(REC_KEYS) r]
    foreach line [split [read $f] "\n"] {
        set line [string trim [lindex [split $line "#"] 0]]
        if {$line eq ""} continue
        lassign [split $line ":"] kt row mask dir
        dict lappend ::rc_keys $kt [list $row $mask $dir]
    }
    close $f
}
proc rc_irq {} {
    incr ::rc_n
    if {$::rc_n == 1} { record start -audioonly $::env(REC_OUT) }
    if {$::rc_n == $::env(REC_IRQS)} { record stop; exit }
}
proc rc_tickproc {} {
    if {[dict exists $::rc_keys $::rc_tick]} {
        foreach k [dict get $::rc_keys $::rc_tick] {
            lassign $k row mask dir
            if {$dir eq "down"} { keymatrixdown $row $mask } else { keymatrixup $row $mask }
        }
    }
    incr ::rc_tick
}
debug set_bp 0x4045 {} rc_irq
debug set_bp 0x4083 {} rc_tickproc
