package fs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkHashTree(b *testing.B) {
	const expected = "9f944e70c49c8e0ab10993ec2dd0caabebcb9dfd0b9e6fd309b9fc8c56d12346"
	data := os.Getenv("DATA")

	// Calculate size of dir for metrics later
	size := 0
	if err := filepath.Walk(data, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += int(info.Size())
		}
		return err
	}); err != nil {
		b.Fatalf("Failed to calculate size of input tree: %s", err)
	}
	// Run one hash initially to ensure any fs caching is warm.
	NewPathHasher(".", false, sha256.New, "sha256", 1).Hash(data, false, false)
	b.ResetTimer()

	for _, parallelism := range []int{1, 2, 4, 8, 16, 24} {
		b.Run(fmt.Sprintf("%dWay", parallelism), func(b *testing.B) {
			start := time.Now()
			for i := 0; i < b.N; i++ {
				// N.B. We force off xattrs to avoid it trying to short-circuit anything.
				hasher := NewPathHasher(".", false, sha256.New, "sha256", parallelism)
				if hash, err := hasher.Hash(data, false, false); err != nil {
					b.Fatalf("Failed to hash path %s: %s", data, err)
				} else if enc := hex.EncodeToString(hash); enc != expected {
					b.Fatalf("Unexpected hash; was %s, expected %s", enc, expected)
				}
			}
			s := time.Since(start).Seconds()
			mb := float64(size * b.N) / (1024.0 * 1024.0)
			b.ReportMetric(mb/s, "MB/s")
			b.ReportMetric(mb/(s * float64(parallelism)), "MB/s/thread")
		})
	}
}
