package observability

import (
	"github.com/github/github-mcp-server/pkg/observability/log"
)

type Exporters struct {
	Logger log.Logger
}

func NewExporters(logger log.Logger) *Exporters {
	return &Exporters{
		Logger: logger,
	}
}
