package ledger

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultSegmentSize   = 64 << 20
	DefaultDir           = "/var/lib/ledger"
	DefaultBatchTimeout  = 5 * time.Millisecond
	DefaultMaxBatchBytes = 64 * 1024
	DefaultMaxBatchCount = 128
)

// ErrBadDir is returned when Dir is not a usable absolute directory.
var ErrBadDir = errors.New("ledger: bad dir")

// Options tune the WAL.
type Options struct {
	Dir           string        // Dir holds the segment files.
	SegmentSize   int64         // SegmentSize is when we roll to a new file.
	Syncer        Syncer        // Syncer decides how we flush. Nil means fsync.
	BatchTimeout  time.Duration // How long to wait for batch, 0 is per-write sync
	MaxBatchBytes int           // Flush when batch bytes hit this
	MaxBatchCount int           // Flush when batch count hits this
}

// Option configures the WAL.
type Option func(*Options)

// WithDir sets the directory.
func WithDir(dir string) Option {
	return func(options *Options) {
		options.Dir = dir
	}
}

// WithSegmentSize sets the segment size.
func WithSegmentSize(size int64) Option {
	return func(options *Options) {
		options.SegmentSize = size
	}
}

// WithSyncer sets the syncer.
func WithSyncer(syncer Syncer) Option {
	return func(options *Options) {
		options.Syncer = syncer
	}
}

// WithBatchTimeout sets how long to wait to fill a batch.
func WithBatchTimeout(d time.Duration) Option {
	return func(options *Options) {
		options.BatchTimeout = d
	}
}

// WithMaxBatchBytes sets the max bytes per batch.
func WithMaxBatchBytes(n int) Option {
	return func(options *Options) {
		options.MaxBatchBytes = n
	}
}

// WithMaxBatchCount sets the max records per batch.
func WithMaxBatchCount(n int) Option {
	return func(options *Options) {
		options.MaxBatchCount = n
	}
}

func defaultOptions() Options {
	return Options{
		Dir:           DefaultDir,
		SegmentSize:   DefaultSegmentSize,
		Syncer:        defaultSyncer(),
		BatchTimeout:  DefaultBatchTimeout,
		MaxBatchBytes: DefaultMaxBatchBytes,
		MaxBatchCount: DefaultMaxBatchCount,
	}
}

// Validate checks the options before Open uses them.
func (options Options) Validate() error {
	return validateDir(options.Dir)
}

// validateDir checks the dir before we try to use it.
func validateDir(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return ErrBadDir
	}

	if strings.Contains(dir, "\x00") {
		return ErrBadDir
	}

	cleaned := filepath.Clean(dir)
	if cleaned == "." {
		return ErrBadDir
	}

	if !filepath.IsAbs(cleaned) {
		return ErrBadDir
	}

	if info, err := os.Stat(cleaned); err == nil && !info.IsDir() {
		return ErrBadDir
	}

	return nil
}

func applyOptions(opts ...Option) Options {
	options := defaultOptions()
	for _, fn := range opts {
		fn(&options)
	}

	if options.SegmentSize <= 0 {
		options.SegmentSize = DefaultSegmentSize
	}

	if options.Syncer == nil {
		options.Syncer = defaultSyncer()
	}

	if options.MaxBatchBytes <= 0 {
		options.MaxBatchBytes = DefaultMaxBatchBytes
	}

	if options.MaxBatchCount <= 0 {
		options.MaxBatchCount = DefaultMaxBatchCount
	}

	return options
}
