package ledger

import (
	"context"
	"testing"
)

func BenchmarkAppendSizes(b *testing.B) {
	sizes := []int{128, 1024, 4096}
	for _, size := range sizes {
		payload := make([]byte, size)
		for i := range payload {
			payload[i] = byte('a' + i%26)
		}

		b.Run(func() string {
			if size == 128 {
				return "128B"
			}
			if size == 1024 {
				return "1KB"
			}
			return "4KB"
		}(), func(b *testing.B) {
			ctx := context.Background()
			dir := b.TempDir()
			wal, err := Open(ctx, WithDir(dir), WithSyncer(NewNoopSyncer()))
			if err != nil {
				b.Fatal(err)
			}
			defer wal.Close(ctx)

			b.ResetTimer()
			for b.Loop() {
				if _, err := wal.Append(ctx, payload); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
