package observability

import (
	"context"
	"log/slog"

	"github.com/github/github-mcp-server/pkg/observability/metrics"
)

// Exporters bundles observability primitives (logger + metrics) for dependency injection.
// The logger is Go's stdlib *slog.Logger — integrators provide their own slog.Handler.
type Exporters interface {
	Logger() *slog.Logger
	Metrics(context.Context) metrics.Metrics
}

type exporters struct {
	logger  *slog.Logger
	metrics metrics.Metrics
}

// NewExporters creates an Exporters bundle. Pass a configured *slog.Logger
// (with whatever slog.Handler you need) and a Metrics implementation.
func NewExporters(logger *slog.Logger, metrics metrics.Metrics) Exporters {
	return &exporters{
		logger:  logger,
		metrics: metrics,
	}
}

func (e *exporters) Logger() *slog.Logger {
	return e.logger
}

func (e *exporters) Metrics(_ context.Context) metrics.Metrics {
	return e.metrics
}
