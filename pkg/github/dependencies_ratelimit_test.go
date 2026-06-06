package github

import (
	"log/slog"
	"testing"

	"github.com/github/github-mcp-server/pkg/observability"
	"github.com/github/github-mcp-server/pkg/observability/metrics"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequestDeps_InitializesRateLimitRegistry(t *testing.T) {
	t.Parallel()

	obs, err := observability.NewExporters(slog.New(slog.DiscardHandler), metrics.NewNoopMetrics())
	require.NoError(t, err)

	deps := NewRequestDeps(
		nil,
		"test",
		false,
		nil,
		translations.NullTranslationHelper,
		0,
		nil,
		obs,
	)

	require.NotNil(t, deps.rateLimits)

	stateA1 := deps.rateLimits.Get("token-a")
	stateA2 := deps.rateLimits.Get("token-a")
	stateB := deps.rateLimits.Get("token-b")

	assert.Same(t, stateA1, stateA2)
	assert.NotSame(t, stateA1, stateB)
}
