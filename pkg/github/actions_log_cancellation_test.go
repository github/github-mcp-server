package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadLogContentCancellation(t *testing.T) {
	for _, stage := range []string{"before_request", "waiting_for_headers", "streaming_response"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			started := make(chan struct{})
			release := make(chan struct{})
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if stage == "before_request" {
					_, _ = w.Write([]byte("log line\n"))
					return
				}
				if stage == "streaming_response" {
					_, _ = w.Write([]byte("partial log line\n"))
					w.(http.Flusher).Flush()
				}
				close(started)
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			defer server.Close()

			if stage == "before_request" {
				cancel()
			}
			done := make(chan error, 1)
			completed := false
			defer func() {
				close(release)
				if !completed {
					<-done
				}
			}()
			go func() {
				_, _, _, err := downloadLogContent(ctx, server.URL, 10, 10) //nolint:bodyclose // downloadLogContent closes its response body.
				done <- err
			}()
			if stage != "before_request" {
				select {
				case <-started:
				case <-time.After(time.Second):
					t.Fatal("log download did not reach the local server")
				}
				cancel()
			}
			select {
			case err := <-done:
				completed = true
				if stage == "before_request" {
					assert.Zero(t, requests.Load())
				}
				require.Error(t, err)
				assert.ErrorIs(t, err, context.Canceled)
			case <-time.After(time.Second):
				t.Error("log download did not stop after cancellation")
			}
		})
	}
}

func TestDownloadLogContentUncanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("first\nsecond\nthird\n"))
	}))
	defer server.Close()
	content, totalLines, response, err := downloadLogContent(t.Context(), server.URL, 2, 10) //nolint:bodyclose // downloadLogContent closes its response body.
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, 3, totalLines)
	assert.Equal(t, "second\nthird", content)
}
