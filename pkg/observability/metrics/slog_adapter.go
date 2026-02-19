package metrics

import (
	"fmt"
	"log/slog"
	"maps"
	"time"
)

// SlogMetrics implements Metrics by logging metric emissions via slog.
// Useful for debugging metrics in local development.
type SlogMetrics struct {
	logger *slog.Logger
	tags   map[string]string
}

var _ Metrics = (*SlogMetrics)(nil)

// NewSlogMetrics returns a new SlogMetrics that logs to the given slog.Logger.
func NewSlogMetrics(logger *slog.Logger) *SlogMetrics {
	return &SlogMetrics{logger: logger}
}

func (s *SlogMetrics) mergedTags(tags map[string]string) map[string]string {
	if len(s.tags) == 0 {
		return tags
	}
	if len(tags) == 0 {
		return s.tags
	}
	merged := make(map[string]string, len(s.tags)+len(tags))
	maps.Copy(merged, s.tags)
	maps.Copy(merged, tags)
	return merged
}

func (s *SlogMetrics) Increment(key string, tags map[string]string) {
	s.logger.Debug("metric.increment", slog.String("key", key), slog.Any("tags", s.mergedTags(tags)))
}

func (s *SlogMetrics) Counter(key string, tags map[string]string, value int64) {
	s.logger.Debug("metric.counter", slog.String("key", key), slog.Int64("value", value), slog.Any("tags", s.mergedTags(tags)))
}

func (s *SlogMetrics) Distribution(key string, tags map[string]string, value float64) {
	s.logger.Debug("metric.distribution", slog.String("key", key), slog.Float64("value", value), slog.Any("tags", s.mergedTags(tags)))
}

func (s *SlogMetrics) DistributionMs(key string, tags map[string]string, value time.Duration) {
	s.logger.Debug("metric.distribution_ms", slog.String("key", key), slog.String("value", fmt.Sprintf("%dms", value.Milliseconds())), slog.Any("tags", s.mergedTags(tags)))
}

func (s *SlogMetrics) WithTags(tags map[string]string) Metrics {
	merged := make(map[string]string, len(s.tags)+len(tags))
	maps.Copy(merged, s.tags)
	maps.Copy(merged, tags)
	return &SlogMetrics{logger: s.logger, tags: merged}
}
