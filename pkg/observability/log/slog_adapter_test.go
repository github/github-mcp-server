package log

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSlogLogger(t *testing.T) {
	slogger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	logger := NewSlogLogger(slogger, InfoLevel)

	assert.NotNil(t, logger)
	assert.Equal(t, InfoLevel, logger.Level())
}

func TestSlogLogger_Level(t *testing.T) {
	tests := []struct {
		name  string
		level Level
	}{
		{"debug", DebugLevel},
		{"info", InfoLevel},
		{"warn", WarnLevel},
		{"error", ErrorLevel},
		{"fatal", FatalLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			slogger := slog.New(slog.NewTextHandler(buf, nil))
			logger := NewSlogLogger(slogger, tt.level)

			assert.Equal(t, tt.level, logger.Level())
		})
	}
}

func TestSlogLogger_ConvenienceMethods(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger := NewSlogLogger(slogger, DebugLevel)

	tests := []struct {
		name    string
		logFunc func(string, ...Field)
		level   string
	}{
		{"Debug", logger.Debug, "DEBUG"},
		{"Info", logger.Info, "INFO"},
		{"Warn", logger.Warn, "WARN"},
		{"Error", logger.Error, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc("test message", String("key", "value"))

			output := buf.String()
			assert.Contains(t, output, "test message")
			assert.Contains(t, output, tt.level)
			assert.Contains(t, output, "key=value")
		})
	}
}

func TestSlogLogger_Fatal(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger := NewSlogLogger(slogger, DebugLevel)

	assert.Panics(t, func() {
		logger.Fatal("fatal message")
	})

	assert.Contains(t, buf.String(), "fatal message")
}

func TestSlogLogger_WithFields(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger := NewSlogLogger(slogger, InfoLevel)

	loggerWithFields := logger.WithFields(
		String("service", "test-service"),
		Int("port", 8080),
	)

	loggerWithFields.Info("message with fields")

	output := buf.String()
	assert.Contains(t, output, "message with fields")
	assert.Contains(t, output, "service")
	assert.Contains(t, output, "test-service")
	assert.Contains(t, output, "port")
	assert.Contains(t, output, "8080")

	buf.Reset()
	logger.Info("message without fields")
	output = buf.String()
	assert.NotContains(t, output, "service")
}

func TestSlogLogger_WithError(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger := NewSlogLogger(slogger, InfoLevel)

	testErr := errors.New("test error message")
	loggerWithError := logger.WithError(testErr)

	loggerWithError.Error("operation failed")

	output := buf.String()
	assert.Contains(t, output, "operation failed")
	assert.Contains(t, output, "error")
	assert.Contains(t, output, "test error message")
}

func TestSlogLogger_Named(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger := NewSlogLogger(slogger, InfoLevel)

	namedLogger := logger.Named("my-component")
	namedLogger.Info("component message")

	output := buf.String()
	assert.Contains(t, output, "component message")
	assert.Contains(t, output, "logger")
	assert.Contains(t, output, "my-component")
}

func TestSlogLogger_WithLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger := NewSlogLogger(slogger, InfoLevel)

	debugLogger := logger.WithLevel(DebugLevel)

	assert.Equal(t, InfoLevel, logger.Level())
	assert.Equal(t, DebugLevel, debugLogger.Level())
}

func TestSlogLogger_Sync(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, nil))
	logger := NewSlogLogger(slogger, InfoLevel)

	err := logger.Sync()
	assert.NoError(t, err)
}

func TestSlogLogger_Chaining(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger := NewSlogLogger(slogger, DebugLevel)

	chainedLogger := logger.
		Named("service").
		WithFields(String("version", "1.0")).
		WithLevel(InfoLevel)

	chainedLogger.Info("chained message")

	output := buf.String()
	assert.Contains(t, output, "chained message")
	assert.Contains(t, output, "service")
	assert.Contains(t, output, "version")
	assert.Contains(t, output, "1.0")
}

func TestConvertLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    Level
		expected slog.Level
	}{
		{"debug to slog debug", DebugLevel, slog.LevelDebug},
		{"info to slog info", InfoLevel, slog.LevelInfo},
		{"warn to slog warn", WarnLevel, slog.LevelWarn},
		{"error to slog error", ErrorLevel, slog.LevelError},
		{"fatal to slog error", FatalLevel, slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertLevel_Unknown(t *testing.T) {
	unknownLevel := Level{"unknown"}
	result := convertLevel(unknownLevel)
	assert.Equal(t, slog.LevelInfo, result)
}

func TestSlogLogger_ImplementsInterface(_ *testing.T) {
	var _ Logger = (*SlogLogger)(nil)
}

func TestSlogLogger_LogWithContext(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger := NewSlogLogger(slogger, DebugLevel)

	ctx := context.Background()
	logger.Log(ctx, InfoLevel, "context message", String("trace_id", "abc123"))

	output := buf.String()
	assert.Contains(t, output, "context message")
	assert.Contains(t, output, "trace_id")
	assert.Contains(t, output, "abc123")
}

func TestSlogLogger_WithFields_PreservesLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger := NewSlogLogger(slogger, WarnLevel)

	withFields := logger.WithFields(String("key", "value"))

	assert.Equal(t, WarnLevel, withFields.Level())

	withFields.Warn("should appear")
	assert.Contains(t, buf.String(), "should appear")
	assert.Contains(t, buf.String(), "key=value")
}

func TestSlogLogger_WithError_NilSafe(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger := NewSlogLogger(slogger, InfoLevel)

	require.NotPanics(t, func() {
		result := logger.WithError(nil)
		assert.NotNil(t, result)
		assert.Equal(t, logger, result)
	})
}

func TestSlogLogger_MultipleFields(t *testing.T) {
	buf := &bytes.Buffer{}
	slogger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger := NewSlogLogger(slogger, DebugLevel)

	logger.Info("multi-field message",
		String("string_field", "value"),
		Int("int_field", 42),
		Bool("bool_field", true),
		Float64("float_field", 3.14),
	)

	output := buf.String()
	assert.Contains(t, output, "multi-field message")
	assert.Contains(t, output, "string_field")
	assert.Contains(t, output, "int_field")
	assert.Contains(t, output, "bool_field")
	assert.Contains(t, output, "float_field")
}
