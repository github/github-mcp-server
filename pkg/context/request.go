package context

import "context"

// readonlyCtxKey is a context key for read-only mode
type readonlyCtxKey struct{}

// WithReadonly adds read-only mode state to the context
func WithReadonly(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, readonlyCtxKey{}, enabled)
}

// IsReadonly retrieves the read-only mode state from the context
func IsReadonly(ctx context.Context) bool {
	if enabled, ok := ctx.Value(readonlyCtxKey{}).(bool); ok {
		return enabled
	}
	return false
}

// toolsetsCtxKey is a context key for the active toolsets
type toolsetsCtxKey struct{}

// WithToolsets adds the active toolsets to the context
func WithToolsets(ctx context.Context, toolsets []string) context.Context {
	return context.WithValue(ctx, toolsetsCtxKey{}, toolsets)
}

// GetToolsets retrieves the active toolsets from the context
func GetToolsets(ctx context.Context) []string {
	if toolsets, ok := ctx.Value(toolsetsCtxKey{}).([]string); ok {
		return toolsets
	}
	return nil
}

// toolsCtxKey is a context key for tools
type toolsCtxKey struct{}

// WithTools adds the tools to the context
func WithTools(ctx context.Context, tools []string) context.Context {
	return context.WithValue(ctx, toolsCtxKey{}, tools)
}

// GetTools retrieves the tools from the context
func GetTools(ctx context.Context) []string {
	if tools, ok := ctx.Value(toolsCtxKey{}).([]string); ok {
		return tools
	}
	return nil
}

// lockdownCtxKey is a context key for lockdown mode
type lockdownCtxKey struct{}

// WithLockdownMode adds lockdown mode state to the context
func WithLockdownMode(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, lockdownCtxKey{}, enabled)
}

// IsLockdownMode retrieves the lockdown mode state from the context
func IsLockdownMode(ctx context.Context) bool {
	if enabled, ok := ctx.Value(lockdownCtxKey{}).(bool); ok {
		return enabled
	}
	return false
}

// insidersCtxKey is a context key for insiders mode
type insidersCtxKey struct{}

// WithInsidersMode adds insiders mode state to the context
func WithInsidersMode(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, insidersCtxKey{}, enabled)
}

// IsInsidersMode retrieves the insiders mode state from the context
func IsInsidersMode(ctx context.Context) bool {
	if enabled, ok := ctx.Value(insidersCtxKey{}).(bool); ok {
		return enabled
	}
	return false
}

// disallowedToolsCtxKey is a context key for disallowed tools
type disallowedToolsCtxKey struct{}

// WithDisallowedTools adds the disallowed tools to the context
func WithDisallowedTools(ctx context.Context, tools []string) context.Context {
	return context.WithValue(ctx, disallowedToolsCtxKey{}, tools)
}

// GetDisallowedTools retrieves the disallowed tools from the context
func GetDisallowedTools(ctx context.Context) []string {
	if tools, ok := ctx.Value(disallowedToolsCtxKey{}).([]string); ok {
		return tools
	}
	return nil
}

// headerFeaturesCtxKey is a context key for raw header feature flags
type headerFeaturesCtxKey struct{}

// WithHeaderFeatures stores the raw feature flags from the X-MCP-Features header into context
func WithHeaderFeatures(ctx context.Context, features []string) context.Context {
	return context.WithValue(ctx, headerFeaturesCtxKey{}, features)
}

// GetHeaderFeatures retrieves the raw feature flags from context
func GetHeaderFeatures(ctx context.Context) []string {
	if features, ok := ctx.Value(headerFeaturesCtxKey{}).([]string); ok {
		return features
	}
	return nil
}
