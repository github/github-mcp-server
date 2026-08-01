package middleware

import (
	"net/http"
	"strconv"
	"time"
)

const (
	errCapacityExceeded = "Server capacity exceeded."
	errTimedOut         = "Timed out while waiting for a pending request to complete."
	errContextCanceled  = "Context was canceled."
)

var (
	defaultBacklogTimeout = time.Second * 60
)

// ThrottleOpts represents a set of throttling options.
type ThrottleOpts struct {
	RetryAfterFn   func(ctxDone bool) time.Duration
	Limit          int
	BacklogLimit   int
	BacklogTimeout time.Duration
	StatusCode     int
}

// Throttle is a middleware that limits number of currently processed requests
// at a time across all users. Note: Throttle is not a rate-limiter per user,
// instead it just puts a ceiling on the number of current in-flight requests
// being processed from the point from where the Throttle middleware is mounted.
func Throttle(limit int) func(http.Handler) http.Handler {
	return ThrottleWithOpts(ThrottleOpts{Limit: limit, BacklogTimeout: defaultBacklogTimeout})
}

// ThrottleBacklog is a middleware that limits number of currently processed
// requests at a time and provides a backlog for holding a finite number of
// pending requests.
func ThrottleBacklog(limit, backlogLimit int, backlogTimeout time.Duration) func(http.Handler) http.Handler {
	return ThrottleWithOpts(ThrottleOpts{Limit: limit, BacklogLimit: backlogLimit, BacklogTimeout: backlogTimeout})
}

// ThrottleWithOpts is a middleware that limits number of currently processed requests using passed ThrottleOpts.
func ThrottleWithOpts(opts ThrottleOpts) func(http.Handler) http.Handler {
	if opts.Limit < 1 {
		panic("chi/middleware: Throttle expects limit > 0")
	}

	if opts.BacklogLimit < 0 {
		panic("chi/middleware: Throttle expects backlogLimit to be positive")
	}

	statusCode := opts.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusTooManyRequests
	}

	t := throttler{
		tokens:         make(chan token, opts.Limit),
		backlogTokens:  make(chan token, opts.Limit+opts.BacklogLimit),
		backlogTimeout: opts.BacklogTimeout,
		statusCode:     statusCode,
		retryAfterFn:   opts.RetryAfterFn,
	}

	// Filling tokens.
	for i := 0; i < opts.Limit+opts.BacklogLimit; i++ {
		if i < opts.Limit {
			t.tokens <- token{}
		}
		t.backlogTokens <- token{}
	}

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			select {

			case <-ctx.Done():
				t.setRetryAfterHeaderIfNeeded(w, true)
				http.Error(w, errContextCanceled, t.statusCode)
				return

			case btok := <-t.backlogTokens:
				defer func() {
					t.backlogTokens <- btok
				}()

				// Try to get a processing token immediately first
				select {
				case tok := <-t.tokens:
					defer func() {
						t.tokens <- tok
					}()
					next.ServeHTTP(w, r)
					return
				default:
					// No immediate token available, need to wait with timer
				}

				timer := time.NewTimer(t.backlogTimeout)
				select {
				case <-timer.C:
					t.setRetryAfterHeaderIfNeeded(w, false)
					http.Error(w, errTimedOut, t.statusCode)
					return
				case <-ctx.Done():
					timer.Stop()
					t.setRetryAfterHeaderIfNeeded(w, true)
					http.Error(w, errContextCanceled, t.statusCode)
					return
				case tok := <-t.tokens:
					defer func() {
						timer.Stop()
						t.tokens <- tok
					}()
					next.ServeHTTP(w, r)
				}
				return

			default:
				t.setRetryAfterHeaderIfNeeded(w, false)
				http.Error(w, errCapacityExceeded, t.statusCode)
				return
			}
		}

		return http.HandlerFunc(fn)
	}
}

// token represents a request that is being processed.
type token struct{}

// throttler limits number of currently processed requests at a time.
type throttler struct {
	tokens         chan token
	backlogTokens  chan token
	retryAfterFn   func(ctxDone bool) time.Duration
	backlogTimeout time.Duration
	statusCode     int
}

// setRetryAfterHeaderIfNeeded sets Retry-After HTTP header if corresponding retryAfterFn option of throttler is initialized.
func (t throttler) setRetryAfterHeaderIfNeeded(w http.ResponseWriter, ctxDone bool) {
	if t.retryAfterFn == nil {
		return
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(t.retryAfterFn(ctxDone).Seconds())))
}
