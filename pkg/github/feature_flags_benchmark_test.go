package github

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func BenchmarkFeatureInventory(b *testing.B) {
	for _, distribution := range featureBenchmarkDistributions() {
		b.Run(distribution.name, func(b *testing.B) {
			b.Run("build", func(b *testing.B) {
				var calls atomic.Int64
				b.ReportAllocs()
				for b.Loop() {
					_, err := featureBenchmarkBuilder(distribution, &calls).Build()
					if err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(calls.Load())/float64(b.N), "checks/op")
			})

			builder := featureBenchmarkBuilder(distribution, nil)
			b.Run("preconstructed-builder", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := builder.Build(); err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run("tools-list", func(b *testing.B) {
				inv, calls := featureBenchmarkInventory(b, distribution)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					_ = inv.ForMCPRequest(inventory.MCPMethodToolsList, "").ToolsForRegistration(context.Background())
				}
				b.ReportMetric(float64(calls.Load())/float64(b.N), "checks/op")
			})

			b.Run("read-only-tools-list", func(b *testing.B) {
				var calls atomic.Int64
				checker := func(_ context.Context, flag string) (bool, error) {
					calls.Add(1)
					return distribution.enabled["*"] || distribution.enabled[flag], nil
				}
				tools := AllTools(translations.NullTranslationHelper)
				inv, err := inventory.NewBuilder().
					SetTools(tools).
					SetResources(AllResources(translations.NullTranslationHelper)).
					SetPrompts(AllPrompts(translations.NullTranslationHelper)).
					WithToolsets([]string{"all"}).
					WithReadOnly(true).
					WithFeatureChecker(checker).
					Build()
				if err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					_ = inv.ForMCPRequest(inventory.MCPMethodToolsList, "").ToolsForRegistration(context.Background())
				}
				b.ReportMetric(float64(calls.Load())/float64(b.N), "checks/op")
			})

			b.Run("unflagged-tool-call", func(b *testing.B) {
				inv, calls := featureBenchmarkInventory(b, distribution)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					_ = inv.ForMCPRequest(inventory.MCPMethodToolsCall, "get_commit").ToolsForRegistration(context.Background())
				}
				b.ReportMetric(float64(calls.Load())/float64(b.N), "checks/op")
			})

			b.Run("gated-tool-call", func(b *testing.B) {
				inv, calls := featureBenchmarkInventory(b, distribution)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					_ = inv.ForMCPRequest(inventory.MCPMethodToolsCall, "get_file_blame").ToolsForRegistration(context.Background())
				}
				b.ReportMetric(float64(calls.Load())/float64(b.N), "checks/op")
			})

			b.Run("ui-tool-call", func(b *testing.B) {
				inv, calls := featureBenchmarkInventory(b, distribution)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					_ = inv.ForMCPRequest(inventory.MCPMethodToolsCall, "ui_get").ToolsForRegistration(context.Background())
				}
				b.ReportMetric(float64(calls.Load())/float64(b.N), "checks/op")
			})

			b.Run("direct-handler-checks", func(b *testing.B) {
				var calls atomic.Int64
				checker := func(_ context.Context, flag string) (bool, error) {
					calls.Add(1)
					return distribution.enabled["*"] || distribution.enabled[flag], nil
				}
				b.ReportAllocs()
				for b.Loop() {
					ctx := inventory.WithFeatureState(context.Background(), checker)
					_ = inventory.ResolveFeature(ctx, checker, inventory.FeatureFlag(FeatureFlagCSVOutput))
					_ = inventory.ResolveFeature(ctx, checker, inventory.FeatureFlag(FeatureFlagCSVOutput))
				}
				b.ReportMetric(float64(calls.Load())/float64(b.N), "checks/op")
			})

			b.Run("build-list-register", func(b *testing.B) {
				var calls atomic.Int64
				b.ReportAllocs()
				for b.Loop() {
					inv, err := featureBenchmarkBuilder(distribution, &calls).Build()
					if err != nil {
						b.Fatal(err)
					}
					inv = inv.ForMCPRequest(inventory.MCPMethodToolsList, "")
					server := mcp.NewServer(&mcp.Implementation{Name: "benchmark", Version: "v0"}, nil)
					inv.RegisterAll(context.Background(), server, nil)
				}
				b.ReportMetric(float64(calls.Load())/float64(b.N), "checks/op")
			})
		})
	}
}

type featureBenchmarkDistribution struct {
	name    string
	enabled map[string]bool
}

func featureBenchmarkDistributions() []featureBenchmarkDistribution {
	return []featureBenchmarkDistribution{
		{name: "all-false"},
		{
			name: "mixed",
			enabled: map[string]bool{
				MCPAppsFeatureFlag:           true,
				FeatureFlagFileBlame:         true,
				FeatureFlagIssuesGranular:    true,
				FeatureFlagIssueDependencies: true,
			},
		},
		{name: "all-true", enabled: map[string]bool{"*": true}},
	}
}

func featureBenchmarkBuilder(distribution featureBenchmarkDistribution, calls *atomic.Int64) *inventory.Builder {
	checker := func(_ context.Context, flag string) (bool, error) {
		if calls != nil {
			calls.Add(1)
		}
		return distribution.enabled["*"] || distribution.enabled[flag], nil
	}
	tools := AllTools(translations.NullTranslationHelper)
	for i, baseCount := 0, len(tools); len(tools) < 139; i++ {
		tool := tools[i%baseCount]
		tool.Tool.Name = fmt.Sprintf("%s_remote_%d", tool.Tool.Name, i)
		tools = append(tools, tool)
	}
	return inventory.NewBuilder().
		SetTools(tools).
		SetResources(AllResources(translations.NullTranslationHelper)).
		SetPrompts(AllPrompts(translations.NullTranslationHelper)).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(checker)
}

func featureBenchmarkInventory(b *testing.B, distribution featureBenchmarkDistribution) (*inventory.Inventory, *atomic.Int64) {
	b.Helper()
	var calls atomic.Int64
	inv, err := featureBenchmarkBuilder(distribution, &calls).Build()
	if err != nil {
		b.Fatal(err)
	}
	return inv, &calls
}
