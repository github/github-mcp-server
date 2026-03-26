package metrics

import "time"

// NoopMetrics is a no-op implementation of the Metrics interface.
type NoopMetrics struct{}

var _ Metrics = (*NoopMetrics)(nil)

// NewNoopMetrics returns a new NoopMetrics.
func NewNoopMetrics() *NoopMetrics {
	return &NoopMetrics{}
}

func (n *NoopMetrics) Increment(_ string, _ map[string]string)                       {}
func (n *NoopMetrics) Counter(_ string, _ map[string]string, _ int64)                {}
func (n *NoopMetrics) Distribution(_ string, _ map[string]string, _ float64)         {}
func (n *NoopMetrics) DistributionMs(_ string, _ map[string]string, _ time.Duration) {}
func (n *NoopMetrics) WithTags(_ map[string]string) Metrics                          { return n }
