package transport

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	DefaultMinRateLimitRemaining = 50
	DefaultMinRequestInterval    = 50 * time.Millisecond
	DefaultMaxRateLimitRetries   = 3
)

type RateLimitState struct {
	mu sync.Mutex

	remaining int // -1 means unknown
	reset     time.Time
	lastReq   time.Time
}

func NewRateLimitState() *RateLimitState {
	return &RateLimitState{remaining: -1}
}

type RateLimitRegistry struct {
	states sync.Map
}

func NewRateLimitRegistry() *RateLimitRegistry {
	return &RateLimitRegistry{}
}

func (r *RateLimitRegistry) Get(token string) *RateLimitState {
	if state, ok := r.states.Load(token); ok {
		return state.(*RateLimitState)
	}

	state := NewRateLimitState()
	actual, _ := r.states.LoadOrStore(token, state)
	return actual.(*RateLimitState)
}

type RateLimitTransport struct {
	Transport http.RoundTripper
	State     *RateLimitState

	MinInterval  time.Duration
	MinRemaining int
	MaxRetries   int
	Logger       *slog.Logger
}

func WrapWithRateLimit(base http.RoundTripper, state *RateLimitState) http.RoundTripper {
	if state == nil {
		state = NewRateLimitState()
	}

	return &RateLimitTransport{
		Transport:    base,
		State:        state,
		MinInterval:  DefaultMinRequestInterval,
		MinRemaining: DefaultMinRateLimitRemaining,
		MaxRetries:   DefaultMaxRateLimitRetries,
	}
}

func (t *RateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	maxRetries := t.MaxRetries
	if maxRetries < 0 {
		maxRetries = DefaultMaxRateLimitRetries
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		t.waitBeforeRequest(req.Context())

		resp, err := transport.RoundTrip(req)
		if err != nil {
			return resp, err
		}

		t.updateFromResponse(resp)

		if !isRateLimitedResponse(resp) || attempt == maxRetries {
			return resp, nil
		}

		wait := retryAfterDuration(resp)
		if t.Logger != nil {
			t.Logger.Warn(
				"GitHub API rate limit hit, waiting before retry",
				"attempt", attempt+1,
				"max_retries", maxRetries,
				"wait", wait.Round(time.Second),
				"status", resp.StatusCode,
			)
		}

		resp.Body.Close()
		waitForContext(req.Context(), wait)
	}

	return nil, nil
}

func (t *RateLimitTransport) waitBeforeRequest(ctx context.Context) {
	minInterval := t.MinInterval
	if minInterval <= 0 {
		minInterval = DefaultMinRequestInterval
	}

	minRemaining := t.MinRemaining
	if minRemaining <= 0 {
		minRemaining = DefaultMinRateLimitRemaining
	}

	t.State.mu.Lock()
	defer t.State.mu.Unlock()

	if wait := time.Until(t.State.lastReq.Add(minInterval)); wait > 0 {
		waitForContext(ctx, wait)
	}

	if t.State.remaining >= 0 && t.State.remaining < minRemaining && !t.State.reset.IsZero() {
		if wait := time.Until(t.State.reset) + time.Second; wait > 0 {
			if t.Logger != nil {
				t.Logger.Warn(
					"GitHub API rate limit nearly exhausted, waiting for reset",
					"remaining", t.State.remaining,
					"wait", wait.Round(time.Second),
				)
			}
			waitForContext(ctx, wait)
			t.State.remaining = -1
		}
	}

	t.State.lastReq = time.Now()
}

func (t *RateLimitTransport) updateFromResponse(resp *http.Response) {
	remaining, reset, ok := parseRateLimitHeaders(resp)
	if !ok {
		return
	}

	t.State.mu.Lock()
	defer t.State.mu.Unlock()
	t.State.remaining = remaining
	t.State.reset = reset
}

func parseRateLimitHeaders(resp *http.Response) (remaining int, reset time.Time, ok bool) {
	remainingStr := resp.Header.Get("X-RateLimit-Remaining")
	resetStr := resp.Header.Get("X-RateLimit-Reset")
	if remainingStr == "" || resetStr == "" {
		return 0, time.Time{}, false
	}

	remainingVal, err := strconv.Atoi(remainingStr)
	if err != nil {
		return 0, time.Time{}, false
	}

	resetUnix, err := strconv.ParseInt(resetStr, 10, 64)
	if err != nil {
		return 0, time.Time{}, false
	}

	return remainingVal, time.Unix(resetUnix, 0), true
}

func isRateLimitedResponse(resp *http.Response) bool {
	if resp == nil {
		return false
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusForbidden:
		return resp.Header.Get("Retry-After") != ""
	default:
		return false
	}
}

func retryAfterDuration(resp *http.Response) time.Duration {
	if resp == nil {
		return time.Second
	}

	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}

	if _, reset, ok := parseRateLimitHeaders(resp); ok && !reset.IsZero() {
		if wait := time.Until(reset) + time.Second; wait > 0 {
			return wait
		}
	}

	return time.Second
}

func waitForContext(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
