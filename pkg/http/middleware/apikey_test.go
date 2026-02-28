package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithAPIKey(t *testing.T) {
	tests := []struct {
		name               string
		configuredKey      string
		headerValue        string
		expectedStatusCode int
		expectNextCalled   bool
	}{
		{
			name:               "empty configured key skips validation",
			configuredKey:      "",
			headerValue:        "",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
		{
			name:               "valid key passes through",
			configuredKey:      "my-secret-key",
			headerValue:        "my-secret-key",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
		{
			name:               "missing key returns 401",
			configuredKey:      "my-secret-key",
			headerValue:        "",
			expectedStatusCode: http.StatusUnauthorized,
			expectNextCalled:   false,
		},
		{
			name:               "wrong key returns 403",
			configuredKey:      "my-secret-key",
			headerValue:        "wrong-key",
			expectedStatusCode: http.StatusForbidden,
			expectNextCalled:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := WithAPIKey(tt.configuredKey)(next)

			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.headerValue != "" {
				req.Header.Set("X-API-Key", tt.headerValue)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code)
			assert.Equal(t, tt.expectNextCalled, nextCalled)
		})
	}
}
