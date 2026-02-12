package log

import (
	"context"
	"log/slog"
)

type NoopLogger struct{}

var _ Logger = (*NoopLogger)(nil)

func NewNoopLogger() *NoopLogger {
	return &NoopLogger{}
}

func (l *NoopLogger) Level() Level {
	return DebugLevel
}

func (l *NoopLogger) Log(_ context.Context, _ Level, _ string, _ ...slog.Attr) {
	// No-op
}

func (l *NoopLogger) Debug(_ string, _ ...slog.Attr) {
	// No-op
}

func (l *NoopLogger) Info(_ string, _ ...slog.Attr) {
	// No-op
}

func (l *NoopLogger) Warn(_ string, _ ...slog.Attr) {
	// No-op
}

func (l *NoopLogger) Error(_ string, _ ...slog.Attr) {
	// No-op
}

func (l *NoopLogger) Fatal(_ string, _ ...slog.Attr) {
	// No-op
}

func (l *NoopLogger) WithFields(_ ...slog.Attr) Logger {
	return l
}

func (l *NoopLogger) WithError(_ error) Logger {
	return l
}

func (l *NoopLogger) Named(_ string) Logger {
	return l
}

func (l *NoopLogger) WithLevel(_ Level) Logger {
	return l
}

func (l *NoopLogger) Sync() error {
	return nil
}
