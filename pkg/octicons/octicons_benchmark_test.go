package octicons

import (
	"runtime"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var benchmarkDataURISink string
var benchmarkIconsSink [][]mcp.Icon

func BenchmarkDataURICache(b *testing.B) {
	b.Run("cold", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var cache dataURICache
			benchmarkDataURISink = cache.load("repo", ThemeLight)
		}
	})

	b.Run("warm", func(b *testing.B) {
		var cache dataURICache
		benchmarkDataURISink = cache.load("repo", ThemeLight)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			benchmarkDataURISink = cache.load("repo", ThemeLight)
		}
	})
}

// Run each ColdStart inventory separately with -benchtime=1x in a fresh
// `go test` process so package-level cache state cannot cross-contaminate it.
func BenchmarkIconsRegistrationColdStart(b *testing.B) {
	benchmarkIconsRegistration(b, false)
}

func BenchmarkIconsRegistrationWarm(b *testing.B) {
	benchmarkIconsRegistration(b, true)
}

func benchmarkIconsRegistration(b *testing.B, warm bool) {
	inventories := map[string][]string{
		"narrow":  {"repo"},
		"default": RequiredIcons(),
	}
	for name, inventory := range inventories {
		b.Run(name, func(b *testing.B) {
			if warm {
				for _, icon := range inventory {
					_ = Icons(icon)
				}
			} else {
				dataURIs.mu.Lock()
				dataURIs.values = nil
				dataURIs.mu.Unlock()
			}

			batch := make([][]mcp.Icon, len(inventory))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for index, icon := range inventory {
					batch[index] = Icons(icon)
				}
			}
			b.StopTimer()
			benchmarkIconsSink = batch
			runtime.KeepAlive(batch)
		})
	}
}
