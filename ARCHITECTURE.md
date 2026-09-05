## The big picture

You have a folder on disk. That folder holds segment files named `00000001.log`, `00000002.log`, and so on. Each segment is just a sequence of `32KB` blocks. A record never crosses a block boundary. If it doesn't fit, we split it. If there are only a few bytes left in a block, we pad with zeros and start the next block.

That is basically it. Everything else is there to make a crash at the worst possible byte still give you a clean prefix when you read it back.

```
Your app  ->  WAL.Append(payload)  ->  blocks on disk  ->  Reader.ReadAll -> your app again
                |                        |
                |  32KB blocks           |  readdir + sort + block scan
                |  7B header + payload   |  CRC check, reassemble
                |  one fsync per batch   |  stop at first bad record
```

## The pieces

**WAL** (`ledger.go`) is the front door. You call `Open`, `Append`, `Close`. It owns the current file, the writer, and the segment manager. It does not do the block math itself it delegates.

**blockWriter** (`block_writer.go`) owns the block offset. It knows how much space is left in the current block, when to pad, and what kind a fragment should be (`Full`, `First`, `Middle`, `Last`). All the arithmetic about `remaining < 7` lives here, so a fix in one place fixes every caller.

**segmentManager** (`segment_manager.go`) owns file names and rotation. It lists the directory, sorts it, picks `max + 1` for the next file, and does `fsync(dir)` so the new file survives a crash even before you write to it. The rotation check `if size + need > SegmentSize, pad, sync, close, create next` lives here. That means the crash window between creating a new segment and recording that it exists is handled in one spot.

**Options** (`options.go`) is where configuration lives. Defaults, `WithDir` / `WithSegmentSize` / `WithSyncer` / `WithBatchTimeout`, and validation all sit together. If you pass a blank dir we tell you `ErrBadDir` instead of silently using `/var/lib/ledger`. `BatchTimeout=0` means every `Append` does its own `fsync` that is our baseline for benchmarks.

**Syncer** (`syncer.go`, `syncer_darwin.go`, `syncer_linux.go`) is how we flush. The interface is just `Sync(*os.File) error` and we have a few adapters: `Fsync`, `Fdatasync` (Linux, data only, a bit cheaper), `Full` (Darwin `F_FULLFSYNC` via `fcntl 51` to flush the drive cache), and `Noop` for tests. Only `darwin` and `linux` have real implementations.

**Reader** (`reader.go`) is the recovery path. It lists segments in order, reads each file block by block, checks the header, verifies the `CRC`, and stitches `First + Middle* + Last` back into the original record. In strict mode the first bad header stops the whole replay and you get the prefix. In salvage mode `WithVerify(false)` it skips the bad block and keeps going.

## A write, step by step

Say you call `Append(ctx, []byte("hello"))`:

1. If batching is on (`BatchTimeout > 0`), your request goes into a channel. A background goroutine (`group.go: runGroupCommit`) collects requests for up to `5ms` or until `64KB` / `128` records. One request becomes the leader, grabs the lock, writes every payload in the batch with `blockWriter`, does a single `Sync` through the `Syncer`, and wakes everyone with the same error. If batching is off (`WithBatchTimeout(0)`), `Append` just grabs the lock itself and does `write + fsync`.

2. Inside the lock, `appendLocked` (`write.go`) asks `blockWriter` if there is room for a header. If not, it pads the tail with zeros. Then it asks `segmentManager` if we need to rotate. If the current file would get too big, we pad the old block, `Sync` it, close it, create the next `00000002.log`, and `fsync` the directory.

3. Now we know how much we can fit in the current block (`availablePayload`). If the payload fits, we pick `Full` (or `First`/`Last` if it was split before) and write one header plus payload. If it does not fit, we write `First`, then `Middle` pieces, then `Last`. Each piece gets its own `7` byte header with its own masked `CRC`.

4. The header is `4` bytes masked `CRC32C` (little-endian) + `2` bytes length + `1` byte kind. The CRC is over `kind + payload` and masked as `((crc >> 15) | (crc << 17)) + 0xa282ead8` so a file of zeros never looks valid. The header goes first so a truncation mid-header is obvious, the reader sees a short header at the end and stops.

5. `Sync` is called. On Linux that is `fsync` or `fdatasync`, on Darwin it is `F_FULLFSYNC`. If the drive lies or `nobarrier` is set, this may only reach the kernel, but the tail will still be dropped on the next read — you lose the last unflushed records, you do not get a reordered log.

## A read, step by step

`NewReader(dir).ReadAll(ctx)` does:

1. `listSegments` `readdir`, filter `*.log`, `sort.Strings`. Zero-padded names make lexical sort equal numeric sort.

2. For each segment, read the whole file and walk it `32KB` at a time. Inside each block, walk the headers. If the `7` byte header is all zeros, it is padding jump to the next block. If `length > remaining` or `kind` is not one of `0..4` or the `CRC` does not match after unmasking, strict mode stops and returns what it has so far. Salvage mode breaks out of that block and continues.

3. Fragments are stitched. `Full` goes straight to your callback. `First` starts a buffer, `Middle` appends, `Last` appends and then calls your callback with the whole reassembled record. An incomplete fragment at EOF is discarded.

That is why a crash that truncates at any byte, or a torn `512B` sector, just means you get a shorter prefix, never a half record.

## Where things can go wrong, and what we promise

_If the machine loses power right as you are writing_, the file may be cut short or the last sector may be half old, half new. We detect both and drop the tail.

_If the drive's write cache lies_, `fsync` may not have reached the platter. Then you may lose the last batch that was acknowledged, but you will never see a record from the middle disappear or two records swap order.

We do **not** promise to prevent torn sectors, to survive a lost directory entry without `fsync(dir)`, or to deduplicate replay. If you `Append` and your ack is lost, you will retry and the same payload will appear twice. Your consumer should handle that with an idempotency key in the payload.

## Tuning

You will usually just do:

```go
w, _ := ledger.Open(ctx, ledger.WithDir("/var/lib/ledger"))
```

If you are on Linux and you append sequentially, `NewFdatasyncSyncer()` is a bit cheaper than `NewFsyncSyncer()` because it skips metadata. On Darwin, `NewFullSyncer()` is the strongest. For benchmarks, `NewNoopSyncer()` skips the flush so you can see the cost of everything else.

Batching is on by default (`5ms`, `64KB`, `128`). Under concurrent load it is a lot faster because one `fsync` serves many `Append`s. Turn it off with `WithBatchTimeout(0)` to see the per-write baseline.

## Segments and cleanup

When the current segment would get too big, we roll to the next number. `fsync(dir)` makes sure the new empty file is on disk before we return from the `Append` that triggered the roll.

When your consumer has durably handled a prefix, call `TrimOldest()`. It deletes the oldest segment if there is more than one, never the current one, and `fsync`s the directory again. Do not call it until you really do not need that prefix.
