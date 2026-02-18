package metrics

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSlogMetrics_ImplementsInterface(_ *testing.T) {
	var _ Metrics = (*SlogMetrics)(nil)
}

func newTestSlogMetrics() (*SlogMetrics, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return NewSlogMetrics(logger), buf
}

func TestSlogMetrics_Increment(t *testing.T) {
	m, buf := newTestSlogMetrics()
	m.Increment("req.count", map[string]string{"tool": "search"})

	output := buf.String()
	assert.Contains(t, output, "metric.increment")
	assert.Contains(t, output, "req.count")
	assert.Contains(t, output, "search")
}

func TestSlogMetrics_Counter(t *testing.T) {
	m, buf := newTestSlogMetrics()
	m.Counter("api.calls", map[string]string{"status": "200"}, 5)

	output := buf.String()
	assert.Contains(t, output, "metric.counter")
	assert.Contains(t, output, "api.calls")
	assert.Contains(t, output, "5")
}

func TestSlogMetrics_Distribution(t *testing.T) {
	m, buf := newTestSlogMetrics()
	m.Distribution("latency", map[string]string{"endpoint": "/api"}, 42.5)

	output := buf.String()
	assert.Contains(t, output, "metric.distribution")
	assert.Contains(t, output, "latency")
	assert.Contains(t, output, "42.5")
}

func TestSlogMetrics_DistributionMs(t *testing.T) {
	m, buf := newTestSlogMetrics()
	m.DistributionMs("duration", map[string]string{"op": "fetch"}, 150*time.Millisecond)

	output := buf.String()
	assert.Contains(t, output, "metric.distribution_ms")
	assert.Contains(t, output, "duration")
	assert.Contains(t, output, "150ms")
}

func TestSlogMetrics_WithTags(t *testing.T) {
	m, buf := newTestSlogMetrics()
	tagged := m.WithTags(map[string]string{"env": "prod"})

	tagged.Increment("req.count", map[string]string{"tool": "search"})

	output := buf.String()
	assert.Contains(t, output, "env")
	assert.Contains(t, output, "prod")
	assert.Contains(t, output, "search")
}

func TestSlogMetrics_WithTags_Chaining(t *testing.T) {
	m, buf := newTestSlogMetrics()
	tagged := m.WithTags(map[string]string{"env": "prod"}).WithTags(map[string]string{"region": "us"})

	tagged.Increment("req.count", nil)

	output := buf.String()
	assert.Contains(t, output, "env")
	assert.Contains(t, output, "prod")
	assert.Contains(t, output, "region")
	assert.Contains(t, output, "us")
}

func TestSlogMetrics_WithTags_DoesNotMutateOriginal(t *testing.T) {
	m, buf := newTestSlogMetrics()
	_ = m.WithTags(map[string]string{"env": "prod"})

	m.Increment("req.count", nil)

	output := buf.String()
	assert.NotContains(t, output, "prod")
}

func TestSlogMetrics_NilTags(t *testing.T) {
	m, buf := newTestSlogMetrics()

	assert.NotPanics(t, func() {
		m.Increment("key", nil)
		m.Counter("key", nil, 1)
		m.Distribution("key", nil, 1.5)
		m.DistributionMs("key", nil, time.Second)
	})

	output := buf.String()
	assert.Contains(t, output, "metric.increment")
}
