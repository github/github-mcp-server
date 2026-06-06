package transport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimitTransport_UpdatesStateFromHeaders(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("X-RateLimit-Remaining", "100")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	state := NewRateLimitState()
	client := &http.Client{Transport: WrapWithRateLimit(server.Client().Transport, state)}

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	state.mu.Lock()
	defer state.mu.Unlock()
	assert.Equal(t, 100, state.remaining)
	assert.False(t, state.reset.IsZero())
	assert.Equal(t, int32(1), calls.Load())
}

func TestRateLimitTransport_WaitsWhenRemainingLow(t *testing.T) {
	t.Parallel()

	state := NewRateLimitState()
	state.mu.Lock()
	state.remaining = 10
	state.reset = time.Now().Add(200 * time.Millisecond)
	state.mu.Unlock()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	transport := &RateLimitTransport{
		Transport:    server.Client().Transport,
		State:        state,
		MinInterval:  0,
		MinRemaining: 50,
		MaxRetries:   0,
	}
	client := &http.Client{Transport: transport}

	start := time.Now()
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.GreaterOrEqual(t, time.Since(start), 150*time.Millisecond)
}

func TestRateLimitTransport_EnforcesMinInterval(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	state := NewRateLimitState()
	transport := &RateLimitTransport{
		Transport:    server.Client().Transport,
		State:        state,
		MinInterval:  100 * time.Millisecond,
		MinRemaining: 0,
		MaxRetries:   0,
	}
	client := &http.Client{Transport: transport}

	start := time.Now()
	resp1, err := client.Get(server.URL)
	require.NoError(t, err)
	require.NoError(t, resp1.Body.Close())
	resp2, err := client.Get(server.URL)
	require.NoError(t, err)
	require.NoError(t, resp2.Body.Close())
	assert.GreaterOrEqual(t, time.Since(start), 100*time.Millisecond)
}

func TestRateLimitTransport_RetriesOn429(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	transport := &RateLimitTransport{
		Transport:    server.Client().Transport,
		State:        NewRateLimitState(),
		MinInterval:  0,
		MinRemaining: 0,
		MaxRetries:   1,
	}
	resp, err := (&http.Client{Transport: transport}).Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(2), calls.Load())
}

func TestRateLimitTransport_DoesNotRetryOther403(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	transport := &RateLimitTransport{
		Transport:    server.Client().Transport,
		State:        NewRateLimitState(),
		MinInterval:  0,
		MinRemaining: 0,
		MaxRetries:   2,
	}
	resp, err := (&http.Client{Transport: transport}).Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Equal(t, int32(1), calls.Load())
}

func TestRateLimitRegistry_SharesStatePerToken(t *testing.T) {
	t.Parallel()

	registry := NewRateLimitRegistry()
	stateA1 := registry.Get("token-a")
	stateA2 := registry.Get("token-a")
	stateB := registry.Get("token-b")
	assert.Same(t, stateA1, stateA2)
	assert.NotSame(t, stateA1, stateB)
}

func TestParseRateLimitHeaders(t *testing.T) {
	t.Parallel()

	reset := time.Now().Add(time.Minute).Unix()
	resp := &http.Response{Header: make(http.Header), Body: io.NopCloser(http.NoBody)}
	resp.Header.Set("X-RateLimit-Remaining", "42")
	resp.Header.Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))

	remaining, resetTime, ok := parseRateLimitHeaders(resp)
	require.True(t, ok)
	assert.Equal(t, 42, remaining)
	assert.Equal(t, time.Unix(reset, 0), resetTime)
}

func TestWaitForContext_RespectsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	waitForContext(ctx, time.Second)
	assert.Less(t, time.Since(start), 100*time.Millisecond)
}
