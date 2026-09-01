package wal

const (
	DefaultSegmentSize = 64 << 20          // DefaultSegmentSize is the size that triggers a new segment.
	DefaultDir         = "/var/lib/ledger" // DefaultDir is the production default. Tests use t.TempDir()
)

// Options tune the WAL.
type Options struct {
	Dir         string // Dir holds the segment files.
	SegmentSize int64  // SegmentSize is when we roll to a new file.
	Syncer      Syncer // Syncer decides how we flush. Nil means fsync.
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

func defaultOptions() Options {
	return Options{
		Dir:         DefaultDir,
		SegmentSize: DefaultSegmentSize,
		Syncer:      defaultSyncer(),
	}
}

func applyOptions(opts ...Option) Options {
	options := defaultOptions()
	for _, fn := range opts {
		fn(&options)
	}
	if options.Dir == "" {
		options.Dir = DefaultDir
	}
	if options.SegmentSize <= 0 {
		options.SegmentSize = DefaultSegmentSize
	}
	if options.Syncer == nil {
		options.Syncer = defaultSyncer()
	}
	return options
}
