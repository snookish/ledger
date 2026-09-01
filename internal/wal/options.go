package wal

// DefaultSegmentSize is the size that triggers a new segment.
const DefaultSegmentSize = 64 << 20 // 64MB

// Options tune the WAL.
type Options struct {
	SegmentSize int64  // SegmentSize is when we roll to a new file.
	Syncer      Syncer // Syncer decides how we flush. Nil means fsync.
}

// Option configures the WAL.
type Option func(*Options)

// WithSegmentSize sets the segment size.
func WithSegmentSize(n int64) Option {
	return func(o *Options) {
		o.SegmentSize = n
	}
}

// WithSyncer sets the syncer.
func WithSyncer(s Syncer) Option {
	return func(o *Options) {
		o.Syncer = s
	}
}

func defaultOptions() Options {
	return Options{
		SegmentSize: DefaultSegmentSize,
		Syncer:      defaultSyncer(),
	}
}

func applyOptions(opts ...Option) Options {
	o := defaultOptions()
	for _, fn := range opts {
		fn(&o)
	}
	if o.SegmentSize <= 0 {
		o.SegmentSize = DefaultSegmentSize
	}
	if o.Syncer == nil {
		o.Syncer = defaultSyncer()
	}
	return o
}
