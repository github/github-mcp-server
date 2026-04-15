package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeContentType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no parameters unchanged",
			input:    "application/json",
			expected: "application/json",
		},
		{
			name:     "charset parameter stripped",
			input:    "application/json; charset=utf-8",
			expected: "application/json",
		},
		{
			name:     "multiple parameters stripped",
			input:    "application/json; charset=utf-8; boundary=foo",
			expected: "application/json",
		},
		{
			name:     "empty header unchanged",
			input:    "",
			expected: "",
		},
		{
			name:     "other content type with params stripped",
			input:    "text/plain; charset=utf-8",
			expected: "text/plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			handler := NormalizeContentType(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.Header.Get(headers.ContentTypeHeader)
			}))

			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.input != "" {
				req.Header.Set(headers.ContentTypeHeader, tt.input)
			}
			handler.ServeHTTP(httptest.NewRecorder(), req)

			assert.Equal(t, tt.expected, got)
		})
	}
}
