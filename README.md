# ledger

A small write-ahead log that stays correct after a crash. You write records to memory, they go to disk first and if the machine dies in the middle of a write you still get back a clean prefix. No half records, no torn sectors.

## How it works

Think of the log as a folder of segment files. Each file is `00000001.log`, `00000002.log`, and so on. We create a new one when the current gets to `64MB` (you can change that).

Inside each file we chop things into `32KB` blocks. Every record gets a `7` byte header: `4` bytes of masked `CRC32C`, `2` bytes of length, `1` byte of kind. The kind tells us if this is a whole record (`Full`) or a piece of a big one (`First` / `Middle` / `Last`). If there are fewer than `7` bytes left in a block we just pad with zeros and start the next block. Big records are split transparently you never notice.

The checksum is `CRC32C` (Castagnoli) over `kind + payload` and it is masked so a file full of zeros never looks valid. If the last sector was torn (`512B` half old, half new) the CRC will not match and we drop the tail. Truncation at any byte is the same. We stop at the first bad header and return what came before.

## Install

```bash
go get github.com/snookish/ledger
```

Requires Go `1.26+`.

## Quick start

```go
package main

import (
	"context"
	"errors"
	"log"

	"github.com/snookish/ledger"
)

func main() {
	ctx := context.Background()

	wal, err := ledger.Open(ctx, ledger.WithDir("/var/lib/ledger"))
	if err != nil {
		log.Fatal(err) // ErrBadDir, context error, or Mkdir/permission error
	}
	defer wal.Close(ctx)

	lsn, err := wal.Append(ctx, []byte(`{"type":"order_created","id":"ord_1"}`))
	if err != nil {
		log.Fatal(err) // ErrClosed, context error, or Sync/write error
	}
	log.Printf("wrote lsn %d", lsn)

	// Read it back
	reader := ledger.NewReader("/var/lib/ledger")
	records, err := reader.ReadAll(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, rec := range records {
		log.Printf("%s", rec)
	}
}
```

## Errors

All methods take a `ctx` and can return `context.Canceled`. Check sentinels with `errors.Is`.

```go
wal, err := ledger.Open(ctx, ledger.WithDir(dir))
if errors.Is(err, ledger.ErrBadDir) {
	log.Fatal("bad dir - blank, null byte, not absolute, or file not dir")
}

lsn, err := wal.Append(ctx, payload)
if errors.Is(err, ledger.ErrClosed) {
	log.Fatal("already closed")
}

if err := wal.Close(ctx); err != nil {
	log.Fatal(err) // Pad, sync, or close failed
}

if err := wal.TrimOldest(); err != nil {
	log.Fatal(err) // Remove or fsync dir failed
}
```

## Options

```go
ledger.Open(ctx,
	ledger.WithDir("/var/lib/ledger"),           // Default /var/lib/ledger, tests use t.TempDir()
	ledger.WithSegmentSize(64<<20),              // Default 64MB
	ledger.WithSyncer(ledger.NewFsyncSyncer()),  // Fsync/Fdatasync/Full/Noop, default Fsync (Full on MacOS)
	ledger.WithBatchTimeout(5*time.Millisecond), // Default 5ms, 0 = per-write sync
	ledger.WithMaxBatchBytes(64*1024),           // Flush batch when bytes hit this
	ledger.WithMaxBatchCount(128),               // Flush batch when count hits this
)
```

Batching is group commit. Many concurrent `Append` calls share one `fsync`. `Append` still looks synchronous to the caller. The leader writes and syncs, followers wait on the same result. Set `BatchTimeout=0` to compare per-write `fsync` in benchmarks.

Syncers:

- `NewFsyncSyncer()` - `fsync` on Linux, `fsync` on MacOS
- `NewFdatasyncSyncer()` - `fdatasync` on Linux (data only, cheaper for sequential), falls back to `fsync` on MacOS
- `NewFullSyncer()` / `NewFullFSyncSyncer()` - `F_FULLFSYNC` via `fcntl 51` on MacOS (flush drive cache), `fsync` on Linux

Only `MacOS` and `linux` are supported for the real syncers.

## Durability

On a correctly configured host (drive honors flush, filesystem with barriers), `Append` returns only after `Sync` i.e `fsync` on Linux, `F_FULLFSYNC` on MacOS. So the prefix of complete records survives process, OS and power failure.

If the drive lies or `nobarrier` is set, the durable prefix is bounded by the last successful sync. The tail you just wrote may be lost, but records in the middle are never reordered or duplicated by the log itself.

A torn `512B` sector is always detected via `CRC` or header check and dropped as part of the tail. The WAL does not prevent torn sectors, does not survive directory loss without `fsync(dir)`, and does not deduplicate replay. Your consumer should handle duplicates idempotently.

## Recovery

`Reader` scans segments `%08d.log` in sorted order, block by block (`32KB`). If `remaining < 7` it is padding. `KindZero` is skipped. `length > remaining` or bad `CRC` stops strict replay and returns the prefix. `WithVerify(false)` skips the bad block for salvage. Recovery time is linear with log size.

## Segments and trimming

Segments are `%08d.log`, discovered by `readdir + sort`, next is `max + 1`, and `fsync(dir)` makes the new file survive a crash before any record is written.

When your consumer has safely persisted a prefix, drop the oldest file:

```go
if err := wal.TrimOldest(); err != nil {
	log.Fatal(err)
}
```

It never deletes the current segment. Call it only after you really do not need that prefix again.

## Examples

```bash
make examples
```

- `examples/basic` - open, append JSON, read back
- `examples/concurrent` - 20 goroutines appending via group commit
- `examples/rotation` - 64KB segments with `TrimOldest`
- `examples/recovery` - truncate and torn sector, show prefix recovery

## Development

```bash
make test              # same, with -race -v
make cover             # coverage + coverage.html to open in a browser
make vet               # go vet
make lint              # golangci-lint
make clean             # rm bin/ coverage.out coverage.html
```

Open `coverage.html` after `make cover` to see line coverage.
