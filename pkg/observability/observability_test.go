package observability

import (
	"context"
	"log/slog"
	"testing"

	"github.com/github/github-mcp-server/pkg/observability/metrics"
	"github.com/stretchr/testify/assert"
)

func TestNewExporters(t *testing.T) {
	logger := slog.Default()
	m := metrics.NewNoopMetrics()
	exp := NewExporters(logger, m)
	ctx := context.Background()

	assert.NotNil(t, exp)
	assert.Equal(t, logger, exp.Logger())
	assert.Equal(t, m, exp.Metrics(ctx))
}

func TestExporters_WithNilLogger(t *testing.T) {
	exp := NewExporters(nil, nil)
	ctx := context.Background()

	assert.NotNil(t, exp)
	assert.Nil(t, exp.Logger())
	assert.Nil(t, exp.Metrics(ctx))
}
