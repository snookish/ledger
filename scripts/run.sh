#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

log "Starting WAL demo"

log "Step 1: Building binary"
mkdir -p bin
go build -o bin/demo ./cmd/demo
log "Build done: bin/demo"

log "Step 2: Running demo"
WAL_DIR=/tmp/ledger-demo ./bin/demo
log "Demo finished"

log "Step 3: Listing segment files (main WAL)"
ls -lh /tmp/ledger-demo 2>&1 | while read -r line; do log "$line"; done

log "Step 4: Listing rotation segment files"
ls -lh /tmp/ledger-demo-rotate 2>&1 | while read -r line; do log "$line"; done

log "Cleaning up"
rm -rf /tmp/ledger-demo /tmp/ledger-demo-rotate
log "Done"
