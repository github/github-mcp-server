package observability

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/observability/log"
	"github.com/stretchr/testify/assert"
)

func TestNewExporters(t *testing.T) {
	logger := log.NewNoopLogger()
	exp := NewExporters(logger)
	ctx := context.Background()

	assert.NotNil(t, exp)
	assert.Equal(t, logger, exp.Logger(ctx))
}

func TestExporters_WithNilLogger(t *testing.T) {
	exp := NewExporters(nil)
	ctx := context.Background()

	assert.NotNil(t, exp)
	assert.Nil(t, exp.Logger(ctx))
}
