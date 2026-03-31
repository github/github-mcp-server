package observability

import (
	"context"
	"log/slog"
	"testing"

	"github.com/github/github-mcp-server/pkg/observability/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExporters(t *testing.T) {
	logger := slog.Default()
	m := metrics.NewNoopMetrics()
	exp, err := NewExporters(logger, m)
	ctx := context.Background()

	require.NoError(t, err)
	assert.NotNil(t, exp)
	assert.Equal(t, logger, exp.Logger())
	assert.Equal(t, m, exp.Metrics(ctx))
}

func TestNewExporters_WithNilLogger(t *testing.T) {
	_, err := NewExporters(nil, metrics.NewNoopMetrics())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "logger must not be nil")
}

func TestNewExporters_WithNilMetrics(t *testing.T) {
	_, err := NewExporters(slog.New(slog.DiscardHandler), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metrics must not be nil")
}

func TestNewExporters_WithDiscardLogger(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	m := metrics.NewNoopMetrics()
	exp, err := NewExporters(logger, m)

	require.NoError(t, err)
	assert.NotNil(t, exp)
	assert.Equal(t, logger, exp.Logger())
	assert.Equal(t, m, exp.Metrics(context.Background()))
}
