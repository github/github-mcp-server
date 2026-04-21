package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeContentType(t *testing.T) {
	tests := []struct {
		name            string
		inputCT         string
		expectedCT      string
	}{
		{
			name:       "exact application/json unchanged",
			inputCT:    "application/json",
			expectedCT: "application/json",
		},
		{
			name:       "strips charset=utf-8",
			inputCT:    "application/json; charset=utf-8",
			expectedCT: "application/json",
		},
		{
			name:       "strips charset=UTF-8",
			inputCT:    "application/json; charset=UTF-8",
			expectedCT: "application/json",
		},
		{
			name:       "strips multiple parameters",
			inputCT:    "application/json; charset=utf-8; boundary=something",
			expectedCT: "application/json",
		},
		{
			name:       "non-json content type left unchanged",
			inputCT:    "text/plain; charset=utf-8",
			expectedCT: "text/plain; charset=utf-8",
		},
		{
			name:       "text/event-stream left unchanged",
			inputCT:    "text/event-stream",
			expectedCT: "text/event-stream",
		},
		{
			name:       "empty content type left unchanged",
			inputCT:    "",
			expectedCT: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedCT string
			inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				capturedCT = r.Header.Get("Content-Type")
			})

			handler := NormalizeContentType(inner)
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.inputCT != "" {
				req.Header.Set("Content-Type", tt.inputCT)
			}

			handler.ServeHTTP(httptest.NewRecorder(), req)

			assert.Equal(t, tt.expectedCT, capturedCT)
		})
	}
}
