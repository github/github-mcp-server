package metrics

import "time"

// Metrics is a backend-agnostic interface for emitting metrics.
// Implementations can route to DataDog, log to slog, or discard (noop).
type Metrics interface {
	Increment(key string, tags map[string]string)
	Counter(key string, tags map[string]string, value int64)
	Distribution(key string, tags map[string]string, value float64)
	DistributionMs(key string, tags map[string]string, value time.Duration)
	WithTags(tags map[string]string) Metrics
}
