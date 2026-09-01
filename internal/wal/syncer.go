package wal

import "os"

// Syncer flushes a file. Swap it to change durability.
type Syncer interface {
	Sync(*os.File) error
}

// fsyncSyncer uses file.Sync (fsync).
type fsyncSyncer struct{}

// NewFsyncSyncer returns a Syncer that calls fsync.
func NewFsyncSyncer() Syncer {
	return fsyncSyncer{}
}

func (fsyncSyncer) Sync(f *os.File) error {
	return f.Sync()
}

// noopSyncer skips flushing. Useful for tests.
type noopSyncer struct{}

// NewNoopSyncer returns a Syncer that does nothing.
func NewNoopSyncer() Syncer {
	return noopSyncer{}
}

func (noopSyncer) Sync(_ *os.File) error {
	return nil
}

// defaultSyncer picks fsync when no Syncer is provided.
func defaultSyncer() Syncer {
	return NewFsyncSyncer()
}
