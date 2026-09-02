# ledger

A small write-ahead log that stays correct after a crash. Every record goes to disk first so you can replay it after a restart and end up in the same place.

## What it does

- 32KB blocks, 7 byte header `4B CRC | 2B len | 1B kind`, masked CRC32C (Castagnoli)
- Records that don't fit are split into `Full / First / Middle / Last` and reassembled on read
- Crash is handled by design: truncation at any byte leaves a prefix of valid records, the tail is dropped

## Install

```bash
go get github.com/snookish/ledger
```

## Quick start

```bash
go test ./...          # unit + race
make test              # same, with -race -v
make cover             # coverage + coverage.html to open in a browser
```

Open `coverage.html` after `make cover` to see line coverage.
