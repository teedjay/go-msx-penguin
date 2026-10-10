#!/bin/sh
# Runs scripts/refdump.tcl headless with a watchdog (macOS has no timeout).
#   scripts/refdump.sh OUT TICKS [KEYS] [SECONDS]
set -e
here=$(cd "$(dirname "$0")" && pwd)
rom="$here/../Antarctic Adventure (1984) Konami [RC-701] [9196].rom"
REFDUMP_OUT="$1" REFDUMP_TICKS="$2" REFDUMP_KEYS="${3:-}" \
  openmsx -machine C-BIOS_MSX1_JP -cart "$rom" -script "$here/refdump.tcl" >/dev/null 2>&1 &
pid=$!
limit=${4:-300}
while kill -0 $pid 2>/dev/null; do
  sleep 1
  limit=$((limit - 1))
  if [ $limit -le 0 ]; then kill $pid; echo "refdump: watchdog killed openMSX" >&2; exit 1; fi
done
wait $pid
