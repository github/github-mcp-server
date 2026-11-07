package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/github/github-mcp-server/v2/pkg/buffer"
	"github.com/google/go-github/v92/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadLogContentCancelledBeforeRequest(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("log\n"))
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _, resp, err := downloadLogContent(ctx, srv.URL, 10, 10)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, hits.Load())
}

func TestDownloadLogContentCancellationDuringDownload(t *testing.T) {
	for _, sendHeaders := range []bool{false, true} {
		t.Run(strconv.FormatBool(sendHeaders), func(t *testing.T) {
			started := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if sendHeaders {
					_, _ = w.Write([]byte("partial log\n"))
					w.(http.Flusher).Flush()
				}
				close(started)
				<-r.Context().Done()
			}))
			defer srv.Close()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, _, resp, err := downloadLogContent(ctx, srv.URL, 10, 10)
				if resp != nil && resp.Body != nil {
					defer resp.Body.Close()
				}
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("download did not start")
			}
			cancel()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(5 * time.Second):
				t.Fatal("cancelled download did not stop")
			}
		})
	}
}

func TestDownloadLogContentHonorsDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("partial log\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_, _, resp, err := downloadLogContent(ctx, srv.URL, 10, 10)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestFailedJobLogsPaginates(t *testing.T) {
	for _, failSecondPage := range []bool{false, true} {
		t.Run(strconv.FormatBool(failSecondPage), func(t *testing.T) {
			var pages []string
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposActionsRunsJobsByOwnerByRepoByRunID: func(w http.ResponseWriter, r *http.Request) {
					page := r.URL.Query().Get("page")
					pages = append(pages, page)
					assert.Equal(t, "latest", r.URL.Query().Get("filter"))
					if page == "" {
						w.Header().Set("Link", `<https://api.github.com`+r.URL.Path+`?page=2>; rel="next"`)
					} else if failSecondPage {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte(`{"message":"page failed"}`))
						return
					}
					conclusion, id := "success", int64(1)
					if page == "2" {
						conclusion, id = "failure", 2
					}
					_ = json.NewEncoder(w).Encode(&github.Jobs{TotalCount: new(2), Jobs: []*github.WorkflowJob{
						{ID: &id, Conclusion: &conclusion},
					}})
				},
				GetReposActionsJobsLogsByOwnerByRepoByJobID: func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Location", "https://example.com/job/2")
					w.WriteHeader(http.StatusFound)
				},
			}))
			result, payload, err := handleFailedJobLogs(t.Context(), client, "owner", "repo", 1, false, 10, 10)
			require.NoError(t, err)
			assert.Equal(t, []string{"", "2"}, pages)
			if failSecondPage {
				assert.True(t, result.IsError)
				assert.Nil(t, payload)
				return
			}
			require.False(t, result.IsError)
			require.NotNil(t, payload.Failed)
			assert.Equal(t, 2, payload.Failed.TotalJobs)
			assert.Equal(t, 1, payload.Failed.FailedJobs)
			require.Len(t, *payload.Failed.Logs, 1)
			assert.Equal(t, int64(2), (*payload.Failed.Logs)[0].URL.JobID)
		})
	}
}

func TestFailedJobLogsCapsCombinedContent(t *testing.T) {
	line := strings.Repeat("界", 3000) + "\n"
	var downloads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		for range 100 {
			_, _ = w.Write([]byte(line))
		}
	}))
	defer srv.Close()
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposActionsRunsJobsByOwnerByRepoByRunID: func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(&github.Jobs{TotalCount: new(3), Jobs: []*github.WorkflowJob{
				{ID: new(int64(1)), Conclusion: new("failure")},
				{ID: new(int64(2)), Conclusion: new("failure")},
				{ID: new(int64(3)), Conclusion: new("failure")},
			}})
		},
		GetReposActionsJobsLogsByOwnerByRepoByJobID: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", srv.URL)
			w.WriteHeader(http.StatusFound)
		},
	}))
	result, payload, err := handleFailedJobLogs(t.Context(), client, "owner", "repo", 1, true, 500, 5000)
	require.NoError(t, err)
	require.False(t, result.IsError)
	logs := *payload.Failed.Logs
	require.Len(t, logs, 3)
	totalBytes := 0
	for _, log := range logs {
		if log.Content != nil {
			totalBytes += len(log.Content.LogsContent)
			assert.True(t, utf8.ValidString(log.Content.LogsContent))
		}
	}
	assert.LessOrEqual(t, totalBytes, buffer.MaxLogBytes)
	assert.Contains(t, logs[1].Content.LogsContent, "[TRUNCATED]")
	require.NotNil(t, logs[2].Error)
	assert.Contains(t, logs[2].Error.Error, "limit reached")
	assert.Equal(t, int32(2), downloads.Load())
}
