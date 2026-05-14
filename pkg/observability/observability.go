package observability

import (
	"context"
	"errors"
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
// Neither may be nil; use slog.New(slog.DiscardHandler) and metrics.NewNoopMetrics()
// if logging or metrics are unwanted.
func NewExporters(logger *slog.Logger, m metrics.Metrics) (Exporters, error) {
	if logger == nil {
		return nil, errors.New("logger must not be nil: use slog.New(slog.DiscardHandler) to discard logs")
	}
	if m == nil {
		return nil, errors.New("metrics must not be nil: use metrics.NewNoopMetrics() to discard metrics")
	}
	return &exporters{
		logger:  logger,
		metrics: m,
	}, nil
}

func (e *exporters) Logger() *slog.Logger {
	return e.logger
}

func (e *exporters) Metrics(_ context.Context) metrics.Metrics {
	return e.metrics
}
