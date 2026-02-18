package log

import (
	"context"
)

type Logger interface {
	Log(ctx context.Context, level Level, msg string, fields ...Field)
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)
	WithFields(fields ...Field) Logger
	WithError(err error) Logger
	Named(name string) Logger
	WithLevel(level Level) Logger
	Sync() error
	Level() Level
}
