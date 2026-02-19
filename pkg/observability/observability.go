package observability

import (
	"context"

	"github.com/github/github-mcp-server/pkg/observability/log"
	"github.com/github/github-mcp-server/pkg/observability/metrics"
)

type Exporters interface {
	Logger(context.Context) log.Logger
	Metrics(context.Context) metrics.Metrics
}

type exporters struct {
	logger  log.Logger
	metrics metrics.Metrics
}

func NewExporters(logger log.Logger, metrics metrics.Metrics) Exporters {
	return &exporters{
		logger:  logger,
		metrics: metrics,
	}
}

func (e *exporters) Logger(_ context.Context) log.Logger {
	return e.logger
}

func (e *exporters) Metrics(_ context.Context) metrics.Metrics {
	return e.metrics
}
