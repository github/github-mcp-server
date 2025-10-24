package log

import (
	"bytes"
	"strings"
	"testing"

	"log/slog"

	"github.com/stretchr/testify/assert"
)

func TestLoggedReadWriter(t *testing.T) {
	t.Run("Read method logs and passes data", func(t *testing.T) {
		// Setup
		inputData := "test input data"
		reader := strings.NewReader(inputData)

		// Create logger with buffer to capture output
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{ReplaceAttr: removeTimeAttr}))

		lrw := NewIOLogger(reader, nil, logger)

		// Test Read
		buf := make([]byte, 100)
		n, err := lrw.Read(buf)

		// Assertions
		assert.NoError(t, err)
		assert.Equal(t, len(inputData), n)
		assert.Equal(t, inputData, string(buf[:n]))
		assert.Contains(t, logBuffer.String(), "[stdin]")
		assert.Contains(t, logBuffer.String(), inputData)
	})

	t.Run("Write method logs and passes data", func(t *testing.T) {
		// Setup
		outputData := "test output data"
		var writeBuffer bytes.Buffer

		// Create logger with buffer to capture output
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{ReplaceAttr: removeTimeAttr}))

		lrw := NewIOLogger(nil, &writeBuffer, logger)

		// Test Write
		n, err := lrw.Write([]byte(outputData))

		// Assertions
		assert.NoError(t, err)
		assert.Equal(t, len(outputData), n)
		assert.Equal(t, outputData, writeBuffer.String())
		assert.Contains(t, logBuffer.String(), "[stdout]")
		assert.Contains(t, logBuffer.String(), outputData)
	})
}

func removeTimeAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey && len(groups) == 0 {
		return slog.Attr{}
	}
	return a
}
