//go:build darwin

package ledger

import (
	"os"
	"syscall"
)

// FdatasyncSyncer on Darwin falls back to fsync, no fdatasync syscall.
type fdatasyncSyncer struct{}

// NewFdatasyncSyncer returns a Syncer that tries fdatasync.
func NewFdatasyncSyncer() Syncer {
	return fdatasyncSyncer{}
}

func (fdatasyncSyncer) Sync(file *os.File) error {
	return file.Sync()
}

// FullSyncer uses F_FULLFSYNC (fcntl 51) to flush drive cache.
type fullSyncer struct{}

// NewFullSyncer returns a Syncer that uses F_FULLFSYNC on Darwin.
func NewFullSyncer() Syncer {
	return fullSyncer{}
}

func (fullSyncer) Sync(file *os.File) error {
	// Ask drive to flush its cache, fallback to normal fsync
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, file.Fd(), uintptr(syscall.F_FULLFSYNC), 0)
	if errno != 0 {
		return file.Sync()
	}
	return nil
}

// NewFullFSyncSyncer is an alias for NewFullSyncer.
func NewFullFSyncSyncer() Syncer {
	return NewFullSyncer()
}
