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
