package github

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/github/github-mcp-server/pkg/observability"
	"github.com/github/github-mcp-server/pkg/observability/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingMetrics is a metrics.Metrics implementation that captures emitted
// metrics so tests can assert on telemetry. It is safe for concurrent use.
type recordingMetrics struct {
	mu         sync.Mutex
	increments []recordedMetric
	counters   []recordedMetric
}

type recordedMetric struct {
	key   string
	tags  map[string]string
	value int64
}

func (m *recordingMetrics) Increment(key string, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.increments = append(m.increments, recordedMetric{key: key, tags: tags, value: 1})
}

func (m *recordingMetrics) Counter(key string, tags map[string]string, value int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters = append(m.counters, recordedMetric{key: key, tags: tags, value: value})
}

func (m *recordingMetrics) Distribution(_ string, _ map[string]string, _ float64)         {}
func (m *recordingMetrics) DistributionMs(_ string, _ map[string]string, _ time.Duration) {}
func (m *recordingMetrics) WithTags(_ map[string]string) metrics.Metrics                  { return m }

// counter returns the recorded counter for the given key, or false if absent.
func (m *recordingMetrics) counter(key string) (recordedMetric, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.counters {
		if c.key == key {
			return c, true
		}
	}
	return recordedMetric{}, false
}

// increment returns the recorded increment for the given key, or false if absent.
func (m *recordingMetrics) increment(key string) (recordedMetric, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.increments {
		if c.key == key {
			return c, true
		}
	}
	return recordedMetric{}, false
}

// depsWithRecordingMetrics returns BaseDeps wired with a recording metrics sink
// plus the sink for assertions.
func depsWithRecordingMetrics(t *testing.T, base BaseDeps) (BaseDeps, *recordingMetrics) {
	t.Helper()
	rec := &recordingMetrics{}
	exporters, err := observability.NewExporters(slog.New(slog.DiscardHandler), rec)
	require.NoError(t, err)
	base.Obsv = exporters
	return base, rec
}

func Test_recordFieldsUsage_Filtered(t *testing.T) {
	deps, rec := depsWithRecordingMetrics(t, BaseDeps{})

	recordFieldsUsage(context.Background(), deps, "search_code", true, 100, 30)

	call, ok := rec.increment(metricFieldsToolCall)
	require.True(t, ok)
	assert.Equal(t, "search_code", call.tags["tool"])
	assert.Equal(t, "true", call.tags["filtered"])

	full, ok := rec.counter(metricFieldsBytesFull)
	require.True(t, ok)
	assert.Equal(t, int64(100), full.value)
	assert.Equal(t, "search_code", full.tags["tool"])
	assert.NotContains(t, full.tags, "filtered")

	sent, ok := rec.counter(metricFieldsBytesSent)
	require.True(t, ok)
	assert.Equal(t, int64(30), sent.value)
}

func Test_recordFieldsUsage_NotFiltered(t *testing.T) {
	deps, rec := depsWithRecordingMetrics(t, BaseDeps{})

	recordFieldsUsage(context.Background(), deps, "search_code", false, 100, 100)

	call, ok := rec.increment(metricFieldsToolCall)
	require.True(t, ok)
	assert.Equal(t, "false", call.tags["filtered"])

	// No byte counters are emitted when the response was not filtered.
	_, ok = rec.counter(metricFieldsBytesFull)
	assert.False(t, ok)
	_, ok = rec.counter(metricFieldsBytesSent)
	assert.False(t, ok)
}

func Test_recordFieldsUsage_NilExporterDoesNotPanic(t *testing.T) {
	// BaseDeps with no Obsv falls back to a noop sink rather than panicking.
	assert.NotPanics(t, func() {
		recordFieldsUsage(context.Background(), BaseDeps{}, "search_code", true, 100, 30)
	})
}
