package octicons

import (
	"runtime"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var benchmarkDataURISink string
var benchmarkIconsSink [][]mcp.Icon

func BenchmarkDataURI(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchmarkDataURISink = DataURI("repo", ThemeLight)
	}
}

func BenchmarkIconsRegistration(b *testing.B) {
	inventories := map[string][]string{
		"narrow":  {"repo"},
		"default": RequiredIcons(),
	}
	for name, inventory := range inventories {
		b.Run(name, func(b *testing.B) {
			batch := make([][]mcp.Icon, len(inventory))
			b.ReportAllocs()
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
