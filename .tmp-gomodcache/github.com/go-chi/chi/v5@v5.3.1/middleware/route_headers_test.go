package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRouteHeaders(t *testing.T) {
	t.Run("empty router should call next handler exactly once", func(t *testing.T) {
		var callCount atomic.Int32

		hr := RouteHeaders()

		handler := hr.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount.Add(1)
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if callCount.Load() != 1 {
			t.Errorf("expected next handler to be called exactly once, but was called %d times", callCount.Load())
		}
	})

	t.Run("matching header should route to correct middleware", func(t *testing.T) {
		var matchedRoute string

		hr := RouteHeaders().
			Route("Host", "example.com", func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					matchedRoute = "example.com"
					next.ServeHTTP(w, r)
				})
			}).
			Route("Host", "other.com", func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					matchedRoute = "other.com"
					next.ServeHTTP(w, r)
				})
			})

		handler := hr.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "example.com"
		req.Header.Set("Host", "example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if matchedRoute != "example.com" {
			t.Errorf("expected matched route to be 'example.com', got '%s'", matchedRoute)
		}
	})

	t.Run("wildcard pattern should match", func(t *testing.T) {
		var matched bool

		hr := RouteHeaders().
			Route("Host", "*.example.com", func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					matched = true
					next.ServeHTTP(w, r)
				})
			})

		handler := hr.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Host", "api.example.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !matched {
			t.Error("expected wildcard pattern to match")
		}
	})

	t.Run("default route should be used when no match", func(t *testing.T) {
		var usedDefault bool

		hr := RouteHeaders().
			Route("Host", "example.com", func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r)
				})
			}).
			RouteDefault(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					usedDefault = true
					next.ServeHTTP(w, r)
				})
			})

		handler := hr.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Host", "other.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !usedDefault {
			t.Error("expected default route to be used when no match")
		}
	})

	t.Run("RouteAny should match any of the provided patterns", func(t *testing.T) {
		var matched bool

		hr := RouteHeaders().
			RouteAny("Content-Type", []string{"application/json", "application/xml"}, func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					matched = true
					next.ServeHTTP(w, r)
				})
			})

		handler := hr.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Test with application/json
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !matched {
			t.Error("expected RouteAny to match 'application/json'")
		}

		// Reset and test with application/xml
		matched = false
		req = httptest.NewRequest("POST", "/", nil)
		req.Header.Set("Content-Type", "application/xml")
		rec = httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !matched {
			t.Error("expected RouteAny to match 'application/xml'")
		}
	})

	t.Run("no match and no default should call next handler", func(t *testing.T) {
		var nextCalled bool

		hr := RouteHeaders().
			Route("Host", "example.com", func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r)
				})
			})

		handler := hr.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Host", "other.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !nextCalled {
			t.Error("expected next handler to be called when no match and no default")
		}
	})
}

func TestPattern(t *testing.T) {
	tests := []struct {
		pattern  string
		value    string
		expected bool
	}{
		{"example.com", "example.com", true},
		{"example.com", "other.com", false},
		{"*.example.com", "api.example.com", true},
		{"*.example.com", "example.com", false},
		{"api.*", "api.example.com", true},
		{"*", "anything", true},
		{"prefix*suffix", "prefixmiddlesuffix", true},
		{"prefix*suffix", "prefixsuffix", true},
		{"prefix*suffix", "wrongmiddlesuffix", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.value, func(t *testing.T) {
			p := NewPattern(tt.pattern)
			if got := p.Match(tt.value); got != tt.expected {
				t.Errorf("Pattern(%q).Match(%q) = %v, want %v", tt.pattern, tt.value, got, tt.expected)
			}
		})
	}
}
