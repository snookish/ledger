#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

log "Starting ledger examples"

log "Building examples"
mkdir -p bin
go build -o bin/basic ./examples/basic
go build -o bin/concurrent ./examples/concurrent
go build -o bin/rotation ./examples/rotation
go build -o bin/recovery ./examples/recovery
log "Build done"

for ex in basic concurrent rotation recovery; do
  log "Running $ex"
  LEDGER_DIR=/tmp/ledger-$ex ./bin/$ex 2>&1 | while read -r line; do log "[$ex] $line"; done
done

log "Cleaning up"
rm -rf /tmp/ledger-basic /tmp/ledger-concurrent /tmp/ledger-rotation /tmp/ledger-recovery /tmp/ledger-demo* 2>/dev/null || true
log "Done"
