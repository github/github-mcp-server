package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNoopMetrics_ImplementsInterface(t *testing.T) {
	var _ Metrics = (*NoopMetrics)(nil)
}

func TestNoopMetrics_NoPanics(t *testing.T) {
	m := NewNoopMetrics()

	assert.NotPanics(t, func() {
		m.Increment("key", map[string]string{"a": "b"})
		m.Counter("key", map[string]string{"a": "b"}, 1)
		m.Distribution("key", map[string]string{"a": "b"}, 1.5)
		m.DistributionMs("key", map[string]string{"a": "b"}, time.Second)
	})
}

func TestNoopMetrics_NilTags(t *testing.T) {
	m := NewNoopMetrics()

	assert.NotPanics(t, func() {
		m.Increment("key", nil)
		m.Counter("key", nil, 1)
		m.Distribution("key", nil, 1.5)
		m.DistributionMs("key", nil, time.Second)
	})
}

func TestNoopMetrics_WithTags(t *testing.T) {
	m := NewNoopMetrics()
	tagged := m.WithTags(map[string]string{"env": "prod"})

	assert.NotNil(t, tagged)
	assert.Equal(t, m, tagged)
}
