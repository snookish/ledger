//go:build linux

package ledger

import (
	"os"
	"syscall"
)

// FdatasyncSyncer uses fdatasync, cheaper for sequential writes.
type fdatasyncSyncer struct{}

// NewFdatasyncSyncer returns a Syncer that calls fdatasync.
func NewFdatasyncSyncer() Syncer {
	return fdatasyncSyncer{}
}

func (fdatasyncSyncer) Sync(file *os.File) error {
	return syscall.Fdatasync(int(file.Fd()))
}

// FullSyncer on Linux just does fsync, no separate full flush.
type fullSyncer struct{}

// NewFullSyncer returns a Syncer that uses fsync on Linux.
func NewFullSyncer() Syncer {
	return fullSyncer{}
}

func (fullSyncer) Sync(file *os.File) error {
	return syscall.Fsync(int(file.Fd()))
}

// NewFullFSyncSyncer is an alias for NewFullSyncer.
func NewFullFSyncSyncer() Syncer {
	return NewFullSyncer()
}
