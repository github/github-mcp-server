package observability

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/observability/log"
	"github.com/stretchr/testify/assert"
)

func TestNewExporters(t *testing.T) {
	logger := log.NewNoopLogger()
	exp := NewExporters(logger)

	assert.NotNil(t, exp)
	assert.Equal(t, logger, exp.Logger)
}

func TestExporters_WithNilLogger(t *testing.T) {
	exp := NewExporters(nil)

	assert.NotNil(t, exp)
	assert.Nil(t, exp.Logger)
}
