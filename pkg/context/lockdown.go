package context

import "context"

// lockdownCtxKey is a context key for lockdown mode information
type lockdownCtxKey struct{}

// WithLockdownMode adds lockdown mode information to the context
func WithLockdownMode(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, lockdownCtxKey{}, enabled)
}

// IsLockdownMode retrieves lockdown mode information from the context
func IsLockdownMode(ctx context.Context) bool {
	if enabled, ok := ctx.Value(lockdownCtxKey{}).(bool); ok {
		return enabled
	}
	return false
}
