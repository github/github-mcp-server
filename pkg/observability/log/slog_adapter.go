package log

import (
	"context"
	"fmt"
	"log/slog"
)

type SlogLogger struct {
	logger *slog.Logger
	level  Level
}

func NewSlogLogger(logger *slog.Logger, level Level) *SlogLogger {
	return &SlogLogger{
		logger: logger,
		level:  level,
	}
}

func (l *SlogLogger) Level() Level {
	return l.level
}

func (l *SlogLogger) Log(ctx context.Context, level Level, msg string, fields ...Field) {
	slogLevel := convertLevel(level)
	l.logger.LogAttrs(ctx, slogLevel, msg, fieldsToSlogAttrs(fields)...)
}

func (l *SlogLogger) Debug(msg string, fields ...Field) {
	l.Log(context.Background(), DebugLevel, msg, fields...)
}

func (l *SlogLogger) Info(msg string, fields ...Field) {
	l.Log(context.Background(), InfoLevel, msg, fields...)
}

func (l *SlogLogger) Warn(msg string, fields ...Field) {
	l.Log(context.Background(), WarnLevel, msg, fields...)
}

func (l *SlogLogger) Error(msg string, fields ...Field) {
	l.Log(context.Background(), ErrorLevel, msg, fields...)
}

func (l *SlogLogger) Fatal(msg string, fields ...Field) {
	l.Log(context.Background(), FatalLevel, msg, fields...)
	_ = l.Sync()
	panic("fatal: " + msg)
}

func (l *SlogLogger) WithFields(fields ...Field) Logger {
	args := make([]any, 0, len(fields)*2)
	for _, f := range fields {
		args = append(args, f.Key, f.Value)
	}
	return &SlogLogger{
		logger: l.logger.With(args...),
		level:  l.level,
	}
}

func (l *SlogLogger) WithError(err error) Logger {
	if err == nil {
		return l
	}
	return &SlogLogger{
		logger: l.logger.With("error", err.Error()),
		level:  l.level,
	}
}

func (l *SlogLogger) Named(name string) Logger {
	return &SlogLogger{
		logger: l.logger.With("logger", name),
		level:  l.level,
	}
}

func (l *SlogLogger) WithLevel(level Level) Logger {
	return &SlogLogger{
		logger: l.logger,
		level:  level,
	}
}

func (l *SlogLogger) Sync() error {
	return nil
}

func fieldsToSlogAttrs(fields []Field) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(fields))
	for _, f := range fields {
		attrs = append(attrs, slog.Any(f.Key, f.Value))
	}
	return attrs
}

func convertLevel(level Level) slog.Level {
	switch level {
	case DebugLevel:
		return slog.LevelDebug
	case InfoLevel:
		return slog.LevelInfo
	case WarnLevel:
		return slog.LevelWarn
	case ErrorLevel:
		return slog.LevelError
	case FatalLevel:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// FormatValue formats any value as a string, used by adapters for display.
func FormatValue(v any) string {
	return fmt.Sprintf("%v", v)
}
