package inventory

import (
	"context"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Benchmark comparing the old Enabled func approach vs the new EnableCondition approach.
// These benchmarks simulate remote server scenarios where tool filtering happens
// on every request, thousands of times.

// --- Simulated feature flag checker (represents a real feature flag service call) ---

func mockFeatureChecker(_ context.Context, flagName string) (bool, error) {
	// Simulate feature flag lookup - in production this might be a DB/cache lookup
	switch flagName {
	case "web_search", "code_search", "issues_v2":
		return true, nil
	case "disabled_feature":
		return false, nil
	default:
		return false, nil
	}
}

// --- OLD APPROACH: Using Enabled function ---

// This represents how tools are currently filtered with the Enabled func
func oldStyleToolFilter(
	ctx context.Context,
	tool *ServerTool,
	featureChecker FeatureFlagChecker,
	_ bool, // isCCA - unused, but kept for signature compatibility
	_ bool, // isUserWithPaidBing - unused, but kept for signature compatibility
	_ bool, // isCopilotChatHost - unused, but kept for signature compatibility
) (bool, error) {
	// 1. Check tool's own Enabled function
	if tool.Enabled != nil {
		enabled, err := tool.Enabled(ctx)
		if err != nil {
			return false, err
		}
		if !enabled {
			return false, nil
		}
	}

	// 2. Check feature flags
	if tool.FeatureFlagEnable != "" {
		enabled, err := featureChecker(ctx, tool.FeatureFlagEnable)
		if err != nil {
			return false, err
		}
		if !enabled {
			return false, nil
		}
	}
	if tool.FeatureFlagDisable != "" {
		enabled, err := featureChecker(ctx, tool.FeatureFlagDisable)
		if err != nil {
			return false, err
		}
		if enabled {
			return false, nil
		}
	}

	return true, nil
}

// --- NEW APPROACH: Using EnableCondition ---

// This represents the new composable condition approach
func newStyleToolFilter(ctx context.Context, tool *ServerTool) (bool, error) {
	if tool.EnableCondition != nil {
		return tool.EnableCondition.Evaluate(ctx)
	}
	return true, nil
}

// --- Test tools with various complexity levels ---

func createOldStyleTools() []*ServerTool {
	// Create tools with various enable patterns typical of remote server
	return []*ServerTool{
		// Simple feature flag only
		{
			Tool:              mcp.Tool{Name: "web_search"},
			FeatureFlagEnable: "web_search",
		},
		// Feature flag + policy check (user has paid bing)
		{
			Tool:              mcp.Tool{Name: "bing_search"},
			FeatureFlagEnable: "web_search",
			Enabled: func(ctx context.Context) (bool, error) {
				// Simulates checking if user has paid bing access
				return ctx.Value(oldCtxKeyUserHasPaidBing) == true, nil
			},
		},
		// CCA AND feature flag
		{
			Tool:              mcp.Tool{Name: "agent_search"},
			FeatureFlagEnable: "code_search",
			Enabled: func(ctx context.Context) (bool, error) {
				return ctx.Value(oldCtxKeyIsCCA) == true, nil
			},
		},
		// CCA bypass (CCA OR feature flag) - complex
		{
			Tool: mcp.Tool{Name: "copilot_workspace"},
			Enabled: func(ctx context.Context) (bool, error) {
				// CCA bypasses feature flag
				if ctx.Value(oldCtxKeyIsCCA) == true {
					return true, nil
				}
				// Otherwise check feature flag (we'd need to pass checker somehow)
				return ctx.Value(oldCtxKeyFeatureFlagEnabled) == true, nil
			},
		},
		// Copilot-chat host bypass
		{
			Tool: mcp.Tool{Name: "code_analysis"},
			Enabled: func(ctx context.Context) (bool, error) {
				// copilot-chat host bypasses feature flag
				if ctx.Value(oldCtxKeyIsCopilotChatHost) == true {
					return true, nil
				}
				return ctx.Value(oldCtxKeyFeatureFlagEnabled) == true, nil
			},
		},
	}
}

func createNewStyleTools() []*ServerTool {
	// Create equivalent tools using EnableCondition
	return []*ServerTool{
		// Simple feature flag only
		{
			Tool:            mcp.Tool{Name: "web_search"},
			EnableCondition: FeatureFlag("web_search"),
		},
		// Feature flag + policy check (user has paid bing)
		{
			Tool: mcp.Tool{Name: "bing_search"},
			EnableCondition: And(
				FeatureFlag("web_search"),
				ContextBool(ctxKeyUserHasPaidBing),
			),
		},
		// CCA AND feature flag
		{
			Tool: mcp.Tool{Name: "agent_search"},
			EnableCondition: And(
				ContextBool(ctxKeyIsCCA),
				FeatureFlag("code_search"),
			),
		},
		// CCA bypass (CCA OR feature flag) - Or combinator
		{
			Tool: mcp.Tool{Name: "copilot_workspace"},
			EnableCondition: Or(
				ContextBool(ctxKeyIsCCA),
				FeatureFlag("issues_v2"),
			),
		},
		// Copilot-chat host bypass
		{
			Tool: mcp.Tool{Name: "code_analysis"},
			EnableCondition: Or(
				ContextBool(ctxKeyIsCopilotChatHost),
				FeatureFlag("code_search"),
			),
		},
	}
}

// Context keys for benchmark - using string constants for ContextBool
const (
	ctxKeyIsCCA              = "is_cca"
	ctxKeyUserHasPaidBing    = "user_has_paid_bing"
	ctxKeyIsCopilotChatHost  = "is_copilot_chat_host"
	ctxKeyFeatureFlagEnabled = "feature_flag_enabled"
)

// Old-style context key type for WithValue comparisons
type oldStyleCtxKey string

const (
	oldCtxKeyIsCCA              oldStyleCtxKey = "is_cca"
	oldCtxKeyUserHasPaidBing    oldStyleCtxKey = "user_has_paid_bing"
	oldCtxKeyIsCopilotChatHost  oldStyleCtxKey = "is_copilot_chat_host"
	oldCtxKeyFeatureFlagEnabled oldStyleCtxKey = "feature_flag_enabled"
)

// --- BENCHMARKS ---

// BenchmarkOldStyleFiltering simulates the old approach with Enabled funcs
func BenchmarkOldStyleFiltering(b *testing.B) {
	tools := createOldStyleTools()

	// Setup context with actor info (simulates what remote server does)
	ctx := context.Background()
	ctx = context.WithValue(ctx, oldCtxKeyIsCCA, true)
	ctx = context.WithValue(ctx, oldCtxKeyUserHasPaidBing, true)
	ctx = context.WithValue(ctx, oldCtxKeyIsCopilotChatHost, false)
	ctx = context.WithValue(ctx, oldCtxKeyFeatureFlagEnabled, true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tool := range tools {
			_, _ = oldStyleToolFilter(ctx, tool, mockFeatureChecker, true, true, false)
		}
	}
}

// BenchmarkNewStyleFiltering simulates the new EnableCondition approach
func BenchmarkNewStyleFiltering(b *testing.B) {
	tools := createNewStyleTools()

	// Setup context with feature checker and pre-computed bools
	ctx := context.Background()
	ctx = ContextWithFeatureChecker(ctx, mockFeatureChecker)
	ctx = ContextWithBools(ctx, ContextBools{
		ctxKeyIsCCA:             true,
		ctxKeyUserHasPaidBing:   true,
		ctxKeyIsCopilotChatHost: false,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tool := range tools {
			_, _ = newStyleToolFilter(ctx, tool)
		}
	}
}

// BenchmarkManyToolsOldStyle - simulate filtering 50 tools (realistic toolset)
func BenchmarkManyToolsOldStyle(b *testing.B) {
	// Create 50 tools with mixed enable patterns
	tools := make([]*ServerTool, 50)
	for i := 0; i < 50; i++ {
		switch i % 5 {
		case 0:
			// Feature flag only
			tools[i] = &ServerTool{
				Tool:              mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				FeatureFlagEnable: "web_search",
			}
		case 1:
			// Feature flag + Enabled check
			tools[i] = &ServerTool{
				Tool:              mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				FeatureFlagEnable: "code_search",
				Enabled: func(ctx context.Context) (bool, error) {
					return ctx.Value(oldCtxKeyIsCCA) == true, nil
				},
			}
		case 2:
			// Enabled check only
			tools[i] = &ServerTool{
				Tool: mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				Enabled: func(ctx context.Context) (bool, error) {
					if ctx.Value(oldCtxKeyIsCCA) == true {
						return true, nil
					}
					return ctx.Value(oldCtxKeyFeatureFlagEnabled) == true, nil
				},
			}
		case 3:
			// No checks
			tools[i] = &ServerTool{
				Tool: mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
			}
		case 4:
			// Disable flag
			tools[i] = &ServerTool{
				Tool:               mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				FeatureFlagDisable: "disabled_feature",
			}
		}
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, oldCtxKeyIsCCA, true)
	ctx = context.WithValue(ctx, oldCtxKeyFeatureFlagEnabled, true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tool := range tools {
			_, _ = oldStyleToolFilter(ctx, tool, mockFeatureChecker, true, false, false)
		}
	}
}

// BenchmarkManyToolsNewStyle - simulate filtering 50 tools with EnableCondition
func BenchmarkManyToolsNewStyle(b *testing.B) {
	// Create 50 tools with mixed enable conditions
	tools := make([]*ServerTool, 50)
	for i := 0; i < 50; i++ {
		switch i % 5 {
		case 0:
			// Feature flag only
			tools[i] = &ServerTool{
				Tool:            mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: FeatureFlag("web_search"),
			}
		case 1:
			// Feature flag + context bool (AND)
			tools[i] = &ServerTool{
				Tool: mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: And(
					ContextBool(ctxKeyIsCCA),
					FeatureFlag("code_search"),
				),
			}
		case 2:
			// OR condition
			tools[i] = &ServerTool{
				Tool: mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: Or(
					ContextBool(ctxKeyIsCCA),
					FeatureFlag("issues_v2"),
				),
			}
		case 3:
			// No checks (Always enabled)
			tools[i] = &ServerTool{
				Tool:            mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: Always(),
			}
		case 4:
			// NOT condition
			tools[i] = &ServerTool{
				Tool:            mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: Not(FeatureFlag("disabled_feature")),
			}
		}
	}

	ctx := context.Background()
	ctx = ContextWithFeatureChecker(ctx, mockFeatureChecker)
	ctx = ContextWithBools(ctx, ContextBools{
		ctxKeyIsCCA: true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tool := range tools {
			_, _ = newStyleToolFilter(ctx, tool)
		}
	}
}

// BenchmarkRemoteServerSimulation - simulates 1000 requests filtering all tools
func BenchmarkRemoteServerSimulation_OldStyle(b *testing.B) {
	tools := createOldStyleTools()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate 1000 requests
		for req := 0; req < 1000; req++ {
			ctx := context.Background()
			// Each request has slightly different actor context
			ctx = context.WithValue(ctx, oldCtxKeyIsCCA, req%3 == 0)
			ctx = context.WithValue(ctx, oldCtxKeyUserHasPaidBing, req%2 == 0)
			ctx = context.WithValue(ctx, oldCtxKeyIsCopilotChatHost, req%7 == 0)
			ctx = context.WithValue(ctx, oldCtxKeyFeatureFlagEnabled, req%4 != 0)

			var enabledCount int
			for _, tool := range tools {
				enabled, _ := oldStyleToolFilter(ctx, tool, mockFeatureChecker,
					req%3 == 0, req%2 == 0, req%7 == 0)
				if enabled {
					enabledCount++
				}
			}
			_ = enabledCount
		}
	}
}

// BenchmarkRemoteServerSimulation_NewStyle - simulates 1000 requests with EnableCondition
func BenchmarkRemoteServerSimulation_NewStyle(b *testing.B) {
	tools := createNewStyleTools()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate 1000 requests
		for req := 0; req < 1000; req++ {
			ctx := context.Background()
			ctx = ContextWithFeatureChecker(ctx, mockFeatureChecker)
			// Each request has slightly different actor context
			ctx = ContextWithBools(ctx, ContextBools{
				ctxKeyIsCCA:             req%3 == 0,
				ctxKeyUserHasPaidBing:   req%2 == 0,
				ctxKeyIsCopilotChatHost: req%7 == 0,
			})

			var enabledCount int
			for _, tool := range tools {
				enabled, _ := newStyleToolFilter(ctx, tool)
				if enabled {
					enabledCount++
				}
			}
			_ = enabledCount
		}
	}
}

// BenchmarkShortCircuitEvaluation_OldStyle - tests OR pattern with short-circuit
func BenchmarkShortCircuitEvaluation_OldStyle(b *testing.B) {
	// Tool with expensive check that should be short-circuited
	tool := &ServerTool{
		Tool: mcp.Tool{Name: "expensive_tool"},
		Enabled: func(ctx context.Context) (bool, error) {
			// CCA check (fast, should short-circuit)
			if ctx.Value(oldCtxKeyIsCCA) == true {
				return true, nil
			}
			// Expensive check that shouldn't run if CCA is true
			for i := 0; i < 100; i++ {
				_ = i * i // Simulate work
			}
			return false, nil
		},
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, oldCtxKeyIsCCA, true) // Should short-circuit

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = oldStyleToolFilter(ctx, tool, mockFeatureChecker, true, false, false)
	}
}

// BenchmarkShortCircuitEvaluation_NewStyle - tests OR with short-circuit
func BenchmarkShortCircuitEvaluation_NewStyle(b *testing.B) {
	// Expensive condition that should be short-circuited
	expensiveCondition := &customCondition{
		eval: func(_ context.Context) (bool, error) {
			for i := 0; i < 100; i++ {
				_ = i * i // Simulate work
			}
			return false, nil
		},
	}

	tool := &ServerTool{
		Tool: mcp.Tool{Name: "expensive_tool"},
		EnableCondition: Or(
			ContextBool(ctxKeyIsCCA), // Should short-circuit before expensive
			expensiveCondition,
		),
	}

	ctx := context.Background()
	ctx = ContextWithBools(ctx, ContextBools{ctxKeyIsCCA: true}) // Should short-circuit

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = newStyleToolFilter(ctx, tool)
	}
}

// customCondition for testing custom expensive conditions
type customCondition struct {
	eval func(ctx context.Context) (bool, error)
}

func (c *customCondition) Evaluate(ctx context.Context) (bool, error) {
	return c.eval(ctx)
}

// BenchmarkComplexConditionTree - tests deep condition tree evaluation
func BenchmarkComplexConditionTree_NewStyle(b *testing.B) {
	// Deep condition tree:
	// (CCA OR (FeatureFlag AND UserPaidBing)) AND NOT DisabledFeature
	tool := &ServerTool{
		Tool: mcp.Tool{Name: "complex_tool"},
		EnableCondition: And(
			Or(
				ContextBool(ctxKeyIsCCA),
				And(
					FeatureFlag("web_search"),
					ContextBool(ctxKeyUserHasPaidBing),
				),
			),
			Not(FeatureFlag("disabled_feature")),
		),
	}

	ctx := context.Background()
	ctx = ContextWithFeatureChecker(ctx, mockFeatureChecker)
	ctx = ContextWithBools(ctx, ContextBools{
		ctxKeyIsCCA:           false,
		ctxKeyUserHasPaidBing: true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = newStyleToolFilter(ctx, tool)
	}
}

// BenchmarkRemoteServerSimulation_NewStyle_Optimized - reuses context (realistic scenario)
// In production, you'd compute context bools once at request start, not per-tool
func BenchmarkRemoteServerSimulation_NewStyle_Optimized(b *testing.B) {
	tools := createNewStyleTools()

	// Pre-create context templates for different request types
	// This is more realistic - you'd compute bools once at start of request
	baseCtx := ContextWithFeatureChecker(context.Background(), mockFeatureChecker)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate 1000 requests
		for req := 0; req < 1000; req++ {
			// Create context once per request (realistic)
			ctx := ContextWithBools(baseCtx, ContextBools{
				ctxKeyIsCCA:             req%3 == 0,
				ctxKeyUserHasPaidBing:   req%2 == 0,
				ctxKeyIsCopilotChatHost: req%7 == 0,
			})

			var enabledCount int
			for _, tool := range tools {
				enabled, _ := newStyleToolFilter(ctx, tool)
				if enabled {
					enabledCount++
				}
			}
			_ = enabledCount
		}
	}
}

// BenchmarkContextSetup compares context setup costs
func BenchmarkContextSetup_OldStyle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		ctx = context.WithValue(ctx, oldCtxKeyIsCCA, true)
		ctx = context.WithValue(ctx, oldCtxKeyUserHasPaidBing, true)
		ctx = context.WithValue(ctx, oldCtxKeyIsCopilotChatHost, false)
		ctx = context.WithValue(ctx, oldCtxKeyFeatureFlagEnabled, true)
		_ = ctx
	}
}

func BenchmarkContextSetup_NewStyle(b *testing.B) {
	baseCtx := ContextWithFeatureChecker(context.Background(), mockFeatureChecker)
	for i := 0; i < b.N; i++ {
		ctx := ContextWithBools(baseCtx, ContextBools{
			ctxKeyIsCCA:             true,
			ctxKeyUserHasPaidBing:   true,
			ctxKeyIsCopilotChatHost: false,
		})
		_ = ctx
	}
}

// BenchmarkPureEvaluation - tests ONLY condition evaluation, no context setup
func BenchmarkPureEvaluation_OldStyle(b *testing.B) {
	tools := createOldStyleTools()
	ctx := context.Background()
	ctx = context.WithValue(ctx, oldCtxKeyIsCCA, true)
	ctx = context.WithValue(ctx, oldCtxKeyUserHasPaidBing, true)
	ctx = context.WithValue(ctx, oldCtxKeyIsCopilotChatHost, false)
	ctx = context.WithValue(ctx, oldCtxKeyFeatureFlagEnabled, true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tool := range tools {
			_, _ = oldStyleToolFilter(ctx, tool, mockFeatureChecker, true, true, false)
		}
	}
}

func BenchmarkPureEvaluation_NewStyle(b *testing.B) {
	tools := createNewStyleTools()
	ctx := context.Background()
	ctx = ContextWithFeatureChecker(ctx, mockFeatureChecker)
	ctx = ContextWithBools(ctx, ContextBools{
		ctxKeyIsCCA:             true,
		ctxKeyUserHasPaidBing:   true,
		ctxKeyIsCopilotChatHost: false,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tool := range tools {
			_, _ = newStyleToolFilter(ctx, tool)
		}
	}
}

// BenchmarkDirectContextValue vs MapLookup - isolate the lookup cost
func BenchmarkDirectContextValue(b *testing.B) {
	ctx := context.WithValue(context.Background(), oldCtxKeyIsCCA, true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ctx.Value(oldCtxKeyIsCCA) == true
	}
}

func BenchmarkMapLookup(b *testing.B) {
	ctx := ContextWithBools(context.Background(), ContextBools{ctxKeyIsCCA: true})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ContextBoolFromContext(ctx, ctxKeyIsCCA)
	}
}

// --- Compiled Bitmask Benchmarks ---

// BenchmarkCompiledFiltering - tests the bitmask-optimized condition evaluation
func BenchmarkCompiledFiltering(b *testing.B) {
	tools := createNewStyleTools()

	// Compile all conditions
	tcs := NewToolConditionSet(tools)

	// Build mask once (simulates start of request)
	mask := tcs.BuildMask(context.Background(), ContextBools{
		ctxKeyIsCCA:             true,
		ctxKeyUserHasPaidBing:   true,
		ctxKeyIsCopilotChatHost: false,
	}, map[string]bool{
		"web_search":  true,
		"code_search": true,
		"issues_v2":   true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tcs.FilterEnabled(mask)
	}
}

// BenchmarkCompiledManyTools - 50 tools with compiled conditions
func BenchmarkCompiledManyTools(b *testing.B) {
	tools := make([]*ServerTool, 50)
	for i := 0; i < 50; i++ {
		switch i % 5 {
		case 0:
			tools[i] = &ServerTool{
				Tool:            mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: FeatureFlag("web_search"),
			}
		case 1:
			tools[i] = &ServerTool{
				Tool: mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: And(
					ContextBool(ctxKeyIsCCA),
					FeatureFlag("code_search"),
				),
			}
		case 2:
			tools[i] = &ServerTool{
				Tool: mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: Or(
					ContextBool(ctxKeyIsCCA),
					FeatureFlag("issues_v2"),
				),
			}
		case 3:
			tools[i] = &ServerTool{
				Tool:            mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: Always(),
			}
		case 4:
			tools[i] = &ServerTool{
				Tool:            mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: Not(FeatureFlag("disabled_feature")),
			}
		}
	}

	tcs := NewToolConditionSet(tools)

	mask := tcs.BuildMask(context.Background(), ContextBools{
		ctxKeyIsCCA: true,
	}, map[string]bool{
		"web_search":       true,
		"code_search":      true,
		"issues_v2":        true,
		"disabled_feature": false,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tcs.FilterEnabled(mask)
	}
}

// BenchmarkCompiledRemoteServer - 1000 requests × 5 tools with compiled conditions
func BenchmarkCompiledRemoteServer(b *testing.B) {
	tools := createNewStyleTools()
	tcs := NewToolConditionSet(tools)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for req := 0; req < 1000; req++ {
			// Build mask once per request
			mask := tcs.BuildMask(context.Background(), ContextBools{
				ctxKeyIsCCA:             req%3 == 0,
				ctxKeyUserHasPaidBing:   req%2 == 0,
				ctxKeyIsCopilotChatHost: req%7 == 0,
			}, map[string]bool{
				"web_search":  true,
				"code_search": true,
				"issues_v2":   true,
			})

			_ = tcs.FilterEnabled(mask)
		}
	}
}

// BenchmarkCompiledRealisticScale - 1000 requests × 50 tools
func BenchmarkCompiledRealisticScale(b *testing.B) {
	tools := make([]*ServerTool, 50)
	for i := 0; i < 50; i++ {
		switch i % 5 {
		case 0:
			tools[i] = &ServerTool{
				Tool:            mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: FeatureFlag("web_search"),
			}
		case 1:
			tools[i] = &ServerTool{
				Tool: mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: And(
					ContextBool(ctxKeyIsCCA),
					FeatureFlag("code_search"),
				),
			}
		case 2:
			tools[i] = &ServerTool{
				Tool: mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: Or(
					ContextBool(ctxKeyIsCCA),
					FeatureFlag("issues_v2"),
				),
			}
		case 3:
			tools[i] = &ServerTool{
				Tool:            mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: Always(),
			}
		case 4:
			tools[i] = &ServerTool{
				Tool:            mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: Not(FeatureFlag("disabled_feature")),
			}
		}
	}

	tcs := NewToolConditionSet(tools)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for req := 0; req < 1000; req++ {
			mask := tcs.BuildMask(context.Background(), ContextBools{
				ctxKeyIsCCA:             req%3 == 0,
				ctxKeyUserHasPaidBing:   req%2 == 0,
				ctxKeyIsCopilotChatHost: req%7 == 0,
			}, map[string]bool{
				"web_search":       true,
				"code_search":      true,
				"issues_v2":        true,
				"disabled_feature": false,
			})

			_ = tcs.FilterEnabled(mask)
		}
	}
}

// BenchmarkPureEvaluation_Compiled - just the evaluation, no mask building
func BenchmarkPureEvaluation_Compiled(b *testing.B) {
	tools := createNewStyleTools()
	tcs := NewToolConditionSet(tools)

	mask := tcs.BuildMask(context.Background(), ContextBools{
		ctxKeyIsCCA:             true,
		ctxKeyUserHasPaidBing:   true,
		ctxKeyIsCopilotChatHost: false,
	}, map[string]bool{
		"web_search":  true,
		"code_search": true,
		"issues_v2":   true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tcs.FilterEnabled(mask)
	}
}

// BenchmarkMaskBuilding - just the mask building overhead
func BenchmarkMaskBuilding(b *testing.B) {
	tools := createNewStyleTools()
	tcs := NewToolConditionSet(tools)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tcs.BuildMask(ctx, ContextBools{
			ctxKeyIsCCA:             true,
			ctxKeyUserHasPaidBing:   true,
			ctxKeyIsCopilotChatHost: false,
		}, map[string]bool{
			"web_search":  true,
			"code_search": true,
			"issues_v2":   true,
		})
	}
}

// --- Realistic Scale Benchmarks ---

// BenchmarkRealisticScale_OldStyle - 1000 requests × 50 tools each
func BenchmarkRealisticScale_OldStyle(b *testing.B) {
	tools := make([]*ServerTool, 50)
	for i := 0; i < 50; i++ {
		switch i % 5 {
		case 0:
			tools[i] = &ServerTool{
				Tool:              mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				FeatureFlagEnable: "web_search",
			}
		case 1:
			tools[i] = &ServerTool{
				Tool:              mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				FeatureFlagEnable: "code_search",
				Enabled: func(ctx context.Context) (bool, error) {
					return ctx.Value(oldCtxKeyIsCCA) == true, nil
				},
			}
		case 2:
			tools[i] = &ServerTool{
				Tool: mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				Enabled: func(ctx context.Context) (bool, error) {
					if ctx.Value(oldCtxKeyIsCCA) == true {
						return true, nil
					}
					return ctx.Value(oldCtxKeyFeatureFlagEnabled) == true, nil
				},
			}
		case 3:
			tools[i] = &ServerTool{Tool: mcp.Tool{Name: fmt.Sprintf("tool_%d", i)}}
		case 4:
			tools[i] = &ServerTool{
				Tool:               mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				FeatureFlagDisable: "disabled_feature",
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for req := 0; req < 1000; req++ {
			ctx := context.Background()
			ctx = context.WithValue(ctx, oldCtxKeyIsCCA, req%3 == 0)
			ctx = context.WithValue(ctx, oldCtxKeyUserHasPaidBing, req%2 == 0)
			ctx = context.WithValue(ctx, oldCtxKeyIsCopilotChatHost, req%7 == 0)
			ctx = context.WithValue(ctx, oldCtxKeyFeatureFlagEnabled, req%4 != 0)

			var enabledCount int
			for _, tool := range tools {
				enabled, _ := oldStyleToolFilter(ctx, tool, mockFeatureChecker, req%3 == 0, req%2 == 0, req%7 == 0)
				if enabled {
					enabledCount++
				}
			}
			_ = enabledCount
		}
	}
}

// BenchmarkRealisticScale_NewStyle - 1000 requests × 50 tools each
func BenchmarkRealisticScale_NewStyle(b *testing.B) {
	tools := make([]*ServerTool, 50)
	for i := 0; i < 50; i++ {
		switch i % 5 {
		case 0:
			tools[i] = &ServerTool{
				Tool:            mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: FeatureFlag("web_search"),
			}
		case 1:
			tools[i] = &ServerTool{
				Tool: mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: And(
					ContextBool(ctxKeyIsCCA),
					FeatureFlag("code_search"),
				),
			}
		case 2:
			tools[i] = &ServerTool{
				Tool: mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: Or(
					ContextBool(ctxKeyIsCCA),
					FeatureFlag("issues_v2"),
				),
			}
		case 3:
			tools[i] = &ServerTool{
				Tool:            mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: Always(),
			}
		case 4:
			tools[i] = &ServerTool{
				Tool:            mcp.Tool{Name: fmt.Sprintf("tool_%d", i)},
				EnableCondition: Not(FeatureFlag("disabled_feature")),
			}
		}
	}

	baseCtx := ContextWithFeatureChecker(context.Background(), mockFeatureChecker)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for req := 0; req < 1000; req++ {
			ctx := ContextWithBools(baseCtx, ContextBools{
				ctxKeyIsCCA:             req%3 == 0,
				ctxKeyUserHasPaidBing:   req%2 == 0,
				ctxKeyIsCopilotChatHost: req%7 == 0,
			})

			var enabledCount int
			for _, tool := range tools {
				enabled, _ := newStyleToolFilter(ctx, tool)
				if enabled {
					enabledCount++
				}
			}
			_ = enabledCount
		}
	}
}

// BenchmarkIntegratedInventory_Compiled - tests the full Inventory.AvailableTools() path
// This is the integrated path using compiled bitmask conditions
func BenchmarkIntegratedInventory_Compiled(b *testing.B) {
	// Create tools with EnableConditions
	tools := make([]ServerTool, 50)
	toolset := ToolsetMetadata{ID: "test", Default: true}
	for i := 0; i < 50; i++ {
		tools[i].Tool = mcp.Tool{Name: fmt.Sprintf("tool_%d", i)}
		tools[i].Toolset = toolset
		switch i % 5 {
		case 0:
			tools[i].EnableCondition = FeatureFlag("web_search")
		case 1:
			tools[i].EnableCondition = And(
				ContextBool(ctxKeyIsCCA),
				FeatureFlag("code_search"),
			)
		case 2:
			tools[i].EnableCondition = Or(
				ContextBool(ctxKeyIsCCA),
				FeatureFlag("issues_v2"),
			)
		case 3:
			tools[i].EnableCondition = Always()
		case 4:
			tools[i].EnableCondition = Not(FeatureFlag("disabled_feature"))
		}
	}

	// Build inventory (compiles conditions at build time)
	inv := NewBuilder().
		SetTools(tools).
		Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for req := 0; req < 1000; req++ {
			ctx := context.Background()
			ctx = ContextWithFeatureChecker(ctx, mockFeatureChecker)
			ctx = ContextWithBools(ctx, ContextBools{
				ctxKeyIsCCA:             req%3 == 0,
				ctxKeyUserHasPaidBing:   req%2 == 0,
				ctxKeyIsCopilotChatHost: req%7 == 0,
			})

			_ = inv.AvailableTools(ctx)
		}
	}
}

// BenchmarkIntegratedInventory_OldStyle - tests equivalent Inventory path with old Enabled func
func BenchmarkIntegratedInventory_OldStyle(b *testing.B) {
	// Create tools with old-style Enabled functions
	tools := make([]ServerTool, 50)
	toolset := ToolsetMetadata{ID: "test", Default: true}
	for i := 0; i < 50; i++ {
		tools[i].Tool = mcp.Tool{Name: fmt.Sprintf("tool_%d", i)}
		tools[i].Toolset = toolset
		switch i % 5 {
		case 0:
			tools[i].FeatureFlagEnable = "web_search"
		case 1:
			tools[i].FeatureFlagEnable = "code_search"
			tools[i].Enabled = func(ctx context.Context) (bool, error) {
				return ctx.Value(oldCtxKeyIsCCA) == true, nil
			}
		case 2:
			tools[i].Enabled = func(ctx context.Context) (bool, error) {
				if ctx.Value(oldCtxKeyIsCCA) == true {
					return true, nil
				}
				return ctx.Value(oldCtxKeyFeatureFlagEnabled) == true, nil
			}
		case 3:
			// Always enabled - no condition
		case 4:
			tools[i].FeatureFlagDisable = "disabled_feature"
		}
	}

	// Build inventory
	inv := NewBuilder().
		SetTools(tools).
		WithFeatureChecker(mockFeatureChecker).
		Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for req := 0; req < 1000; req++ {
			ctx := context.Background()
			ctx = context.WithValue(ctx, oldCtxKeyIsCCA, req%3 == 0)
			ctx = context.WithValue(ctx, oldCtxKeyUserHasPaidBing, req%2 == 0)
			ctx = context.WithValue(ctx, oldCtxKeyIsCopilotChatHost, req%7 == 0)
			ctx = context.WithValue(ctx, oldCtxKeyFeatureFlagEnabled, req%4 != 0)

			_ = inv.AvailableTools(ctx)
		}
	}
}
