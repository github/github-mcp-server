package observability

import (
	"context"
	"log/slog"
)

// loggerContextKey is the context key for request-scoped *slog.Logger.
// Using a private type prevents collisions with other packages.
type loggerContextKey struct{}

// ContextWithLogger returns a new context that carries the supplied logger.
// Use this to attach a logger enriched with request-scoped attributes
// (e.g. tool name, request id) so downstream code can pick it up via
// LoggerFromContext.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

// LoggerFromContext returns the request-scoped logger stored in ctx by
// ContextWithLogger, or nil if none was set. Callers should fall back to
// a base logger in that case.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return nil
	}
	logger, _ := ctx.Value(loggerContextKey{}).(*slog.Logger)
	return logger
}
