package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestContentEncodingMiddleware(t *testing.T) {
	t.Parallel()

	// support for:
	// Content-Encoding: gzip
	// Content-Encoding: deflate
	// Content-Encoding: gzip, deflate
	// Content-Encoding: deflate, gzip
	middleware := AllowContentEncoding("deflate", "gzip")

	tests := []struct {
		name           string
		encodings      []string
		expectedStatus int
	}{
		{
			name:           "Support no encoding",
			encodings:      []string{},
			expectedStatus: 200,
		},
		{
			name:           "Support gzip encoding",
			encodings:      []string{"gzip"},
			expectedStatus: 200,
		},
		{
			name:           "No support for br encoding",
			encodings:      []string{"br"},
			expectedStatus: 415,
		},
		{
			name:           "Support for gzip and deflate encoding",
			encodings:      []string{"gzip", "deflate"},
			expectedStatus: 200,
		},
		{
			name:           "Support for deflate and gzip encoding",
			encodings:      []string{"deflate", "gzip"},
			expectedStatus: 200,
		},
		{
			name:           "No support for deflate and br encoding",
			encodings:      []string{"deflate", "br"},
			expectedStatus: 415,
		},
	}

	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := []byte("This is my content. There are many like this but this one is mine")
			r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
			for _, encoding := range tt.encodings {
				r.Header.Set("Content-Encoding", encoding)
			}

			w := httptest.NewRecorder()
			router := chi.NewRouter()
			router.Use(middleware)
			router.Post("/", func(w http.ResponseWriter, r *http.Request) {})

			router.ServeHTTP(w, r)
			res := w.Result()
			if res.StatusCode != tt.expectedStatus {
				t.Errorf("response is incorrect, got %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}
