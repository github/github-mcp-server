package log

import (
	"context"
	"log/slog"
)

type Logger interface {
	Log(ctx context.Context, level Level, msg string, fields ...slog.Attr)
	Debug(msg string, fields ...slog.Attr)
	Info(msg string, fields ...slog.Attr)
	Warn(msg string, fields ...slog.Attr)
	Error(msg string, fields ...slog.Attr)
	Fatal(msg string, fields ...slog.Attr)
	WithFields(fields ...slog.Attr) Logger
	WithError(err error) Logger
	Named(name string) Logger
	WithLevel(level Level) Logger
	Sync() error
	Level() Level
}
