package observability

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/observability/log"
	"github.com/github/github-mcp-server/pkg/observability/metrics"
	"github.com/stretchr/testify/assert"
)

func TestNewExporters(t *testing.T) {
	logger := log.NewNoopLogger()
	m := metrics.NewNoopMetrics()
	exp := NewExporters(logger, m)
	ctx := context.Background()

	assert.NotNil(t, exp)
	assert.Equal(t, logger, exp.Logger(ctx))
	assert.Equal(t, m, exp.Metrics(ctx))
}

func TestExporters_WithNilLogger(t *testing.T) {
	exp := NewExporters(nil, nil)
	ctx := context.Background()

	assert.NotNil(t, exp)
	assert.Nil(t, exp.Logger(ctx))
	assert.Nil(t, exp.Metrics(ctx))
}
