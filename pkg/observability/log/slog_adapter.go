package log

import (
	"context"
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

func (l *SlogLogger) Log(ctx context.Context, level Level, msg string, fields ...slog.Attr) {
	slogLevel := convertLevel(level)
	l.logger.LogAttrs(ctx, slogLevel, msg, fields...)
}

func (l *SlogLogger) Debug(msg string, fields ...slog.Attr) {
	l.Log(context.Background(), DebugLevel, msg, fields...)
}

func (l *SlogLogger) Info(msg string, fields ...slog.Attr) {
	l.Log(context.Background(), InfoLevel, msg, fields...)
}

func (l *SlogLogger) Warn(msg string, fields ...slog.Attr) {
	l.Log(context.Background(), WarnLevel, msg, fields...)
}

func (l *SlogLogger) Error(msg string, fields ...slog.Attr) {
	l.Log(context.Background(), ErrorLevel, msg, fields...)
}

func (l *SlogLogger) Fatal(msg string, fields ...slog.Attr) {
	l.Log(context.Background(), FatalLevel, msg, fields...)
	panic("fatal log called")
}

func (l *SlogLogger) WithFields(fields ...slog.Attr) Logger {
	fieldKvPairs := make([]any, 0, len(fields)*2)
	for _, attr := range fields {
		k, v := attr.Key, attr.Value
		fieldKvPairs = append(fieldKvPairs, k, v.Any())
	}
	return &SlogLogger{
		logger: l.logger.With(fieldKvPairs...),
		level:  l.level,
	}
}

func (l *SlogLogger) WithError(err error) Logger {
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
	// Slog does not require syncing
	return nil
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
