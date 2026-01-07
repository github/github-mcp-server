package observability

import (
	"context"

	"github.com/github/github-mcp-server/pkg/observability/log"
)

type Exporters interface {
	Logger(context.Context) log.Logger
}

type ObservabilityExporters struct {
	logger log.Logger
}

func NewExporters(logger log.Logger) Exporters {
	return &ObservabilityExporters{
		logger: logger,
	}
}

func (e *ObservabilityExporters) Logger(_ context.Context) log.Logger {
	return e.logger
}
