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
			}

			batch := make([][]mcp.Icon, len(inventory))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if !warm {
					b.StopTimer()
					dataURIs.mu.Lock()
					dataURIs.values = nil
					dataURIs.mu.Unlock()
					b.StartTimer()
				}
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
