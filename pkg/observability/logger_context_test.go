package observability

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggerContext_RoundTrip(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	ctx := ContextWithLogger(context.Background(), logger)
	assert.Equal(t, logger, LoggerFromContext(ctx))
}

func TestLoggerFromContext_Empty(t *testing.T) {
	assert.Nil(t, LoggerFromContext(context.Background()))
	// Defensive: nil context should not panic. Use a typed nil so staticcheck's
	// SA1012 (which flags untyped nil Context literals) stays quiet.
	var nilCtx context.Context
	assert.Nil(t, LoggerFromContext(nilCtx))
}

func TestContextWithLogger_Nil(t *testing.T) {
	// Storing a nil logger should not mask later reads.
	ctx := ContextWithLogger(context.Background(), nil)
	assert.Nil(t, LoggerFromContext(ctx))
}
