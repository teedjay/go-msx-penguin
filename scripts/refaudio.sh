#!/bin/sh
# Records the original's audio with scripts/refaudio.tcl (headless).
#   scripts/refaudio.sh OUT.wav IRQS [KEYS]
set -e
here=$(cd "$(dirname "$0")" && pwd)
rom="$here/../Antarctic Adventure (1984) Konami [RC-701] [9196].rom"
out="${1%.wav}"
if [ -n "$3" ]; then export REC_KEYS="$3"; fi
REC_OUT="$out" REC_IRQS="$2" \
  openmsx -machine C-BIOS_MSX1_JP -cart "$rom" -script "$here/refaudio.tcl" >/dev/null 2>&1
