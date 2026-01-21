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

// toolsetCtxKey is a context key for the active toolset
type toolsetCtxKey struct{}

// WithToolset adds the active toolset to the context
func WithToolset(ctx context.Context, toolset string) context.Context {
	return context.WithValue(ctx, toolsetCtxKey{}, toolset)
}

// GetToolset retrieves the active toolset from the context
func GetToolset(ctx context.Context) string {
	if toolset, ok := ctx.Value(toolsetCtxKey{}).(string); ok {
		return toolset
	}
	return ""
}
