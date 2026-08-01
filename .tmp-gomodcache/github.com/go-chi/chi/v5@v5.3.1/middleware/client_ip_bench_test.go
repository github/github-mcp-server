package middleware

import (
	"fmt"
	"strings"
	"testing"
)

// BenchmarkWalkXFF measures the cost of walking a merged X-Forwarded-For
// chain end-to-end (visitor never stops). Exercises the worst case for
// ClientIPFromXFF when every entry is in the trusted prefix list, and
// bounds the absolute cost of a pathologically large attacker-supplied
// XFF header (subject to http.Server.MaxHeaderBytes).
//
// n varies the number of comma-separated entries in a single XFF header.
// All entries are valid IPs of identical length, so total header size
// scales linearly with n.
func BenchmarkWalkXFF(b *testing.B) {
	for _, n := range []int{1, 10, 100, 1000, 10000} {
		entries := make([]string, n)
		for i := range entries {
			entries[i] = "1.2.3.4"
		}
		headers := []string{strings.Join(entries, ", ")}

		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				walkXFF(headers, func(entry string) bool {
					return false // walk to the leftmost entry
				})
			}
		})
	}
}

// BenchmarkWalkXFF_RightmostStop measures the common case for both
// ClientIPFromXFF (no trusted prefixes => stop at the first / rightmost
// entry) and ClientIPFromXFFTrustedProxies(1). Should be near-constant
// regardless of n, since the walker stops after the first visit.
func BenchmarkWalkXFF_RightmostStop(b *testing.B) {
	for _, n := range []int{1, 10, 100, 1000, 10000} {
		entries := make([]string, n)
		for i := range entries {
			entries[i] = "1.2.3.4"
		}
		headers := []string{strings.Join(entries, ", ")}

		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				walkXFF(headers, func(entry string) bool {
					return true // stop at the rightmost entry
				})
			}
		})
	}
}
