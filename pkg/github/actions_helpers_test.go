package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-github/v79/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the unexported Actions helpers that are dispatched by the
// consolidated actions_* tools (ActionsGet, ActionsList, ActionsRunTrigger).
// The exported tool-definition tests only validate schemas, leaving these
// helpers uncovered, so they are tested here directly against a mocked client.

func notFoundHandler(message string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"` + message + `"}`))
	}
}

func newActionsTestClient(t *testing.T, handlers map[string]http.HandlerFunc) *github.Client {
	t.Helper()
	return github.NewClient(MockHTTPClientWithHandlers(handlers))
}

func TestGetWorkflowJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"GET /repos/owner/repo/actions/jobs/123": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"id":123,"name":"build"}`))
			},
		})

		result, _, err := getWorkflowJob(context.Background(), client, "owner", "repo", 123)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var response map[string]any
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
		assert.Equal(t, float64(123), response["id"])
		assert.Equal(t, "build", response["name"])
	})

	t.Run("api error", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"GET /repos/owner/repo/actions/jobs/123": notFoundHandler("job not found"),
		})

		result, _, err := getWorkflowJob(context.Background(), client, "owner", "repo", 123)
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestListWorkflowJobs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"GET /repos/owner/repo/actions/runs/456/jobs": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"total_count":1,"jobs":[{"id":1,"name":"test"}]}`))
			},
		})

		result, _, err := listWorkflowJobs(context.Background(), client, map[string]any{}, "owner", "repo", 456, PaginationParams{Page: 1, PerPage: 30})
		require.NoError(t, err)
		require.False(t, result.IsError)

		var response map[string]any
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
		assert.Contains(t, response, "jobs")
	})

	t.Run("invalid filter argument", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{})

		// workflow_jobs_filter must be an object; a string triggers a parameter error.
		result, _, err := listWorkflowJobs(context.Background(), client, map[string]any{"workflow_jobs_filter": "not-an-object"}, "owner", "repo", 456, PaginationParams{})
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})

	t.Run("api error", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"GET /repos/owner/repo/actions/runs/456/jobs": notFoundHandler("run not found"),
		})

		result, _, err := listWorkflowJobs(context.Background(), client, map[string]any{}, "owner", "repo", 456, PaginationParams{})
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestListWorkflowArtifacts(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"GET /repos/owner/repo/actions/runs/789/artifacts": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"total_count":1,"artifacts":[{"id":11,"name":"logs"}]}`))
			},
		})

		result, _, err := listWorkflowArtifacts(context.Background(), client, "owner", "repo", 789, PaginationParams{Page: 1, PerPage: 30})
		require.NoError(t, err)
		require.False(t, result.IsError)

		var response map[string]any
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
		assert.Contains(t, response, "artifacts")
	})

	t.Run("api error", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"GET /repos/owner/repo/actions/runs/789/artifacts": notFoundHandler("run not found"),
		})

		result, _, err := listWorkflowArtifacts(context.Background(), client, "owner", "repo", 789, PaginationParams{})
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestDownloadWorkflowArtifact(t *testing.T) {
	t.Run("success returns download url", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"GET /repos/owner/repo/actions/artifacts/55/zip": func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", "https://example.com/download/55")
				w.WriteHeader(http.StatusFound)
			},
		})

		result, _, err := downloadWorkflowArtifact(context.Background(), client, "owner", "repo", 55)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var response map[string]any
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
		assert.Equal(t, "https://example.com/download/55", response["download_url"])
		assert.Equal(t, float64(55), response["artifact_id"])
	})

	t.Run("api error", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"GET /repos/owner/repo/actions/artifacts/55/zip": notFoundHandler("artifact not found"),
		})

		result, _, err := downloadWorkflowArtifact(context.Background(), client, "owner", "repo", 55)
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestGetWorkflowRunLogsURL(t *testing.T) {
	t.Run("success returns logs url", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"GET /repos/owner/repo/actions/runs/999/logs": func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", "https://example.com/logs/999")
				w.WriteHeader(http.StatusFound)
			},
		})

		result, _, err := getWorkflowRunLogsURL(context.Background(), client, "owner", "repo", 999)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var response map[string]any
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
		assert.Equal(t, "https://example.com/logs/999", response["logs_url"])
		assert.Contains(t, response, "optimization_tip")
	})

	t.Run("api error", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"GET /repos/owner/repo/actions/runs/999/logs": notFoundHandler("run not found"),
		})

		result, _, err := getWorkflowRunLogsURL(context.Background(), client, "owner", "repo", 999)
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestGetWorkflowRunUsage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"GET /repos/owner/repo/actions/runs/321/timing": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"run_duration_ms":12345}`))
			},
		})

		result, _, err := getWorkflowRunUsage(context.Background(), client, "owner", "repo", 321)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var response map[string]any
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
		assert.Equal(t, float64(12345), response["run_duration_ms"])
	})

	t.Run("api error", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"GET /repos/owner/repo/actions/runs/321/timing": notFoundHandler("run not found"),
		})

		result, _, err := getWorkflowRunUsage(context.Background(), client, "owner", "repo", 321)
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestRerunWorkflowRun(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"POST /repos/owner/repo/actions/runs/77/rerun": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
			},
		})

		result, _, err := rerunWorkflowRun(context.Background(), client, "owner", "repo", 77)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var response map[string]any
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
		assert.Equal(t, "Workflow run has been queued for re-run", response["message"])
		assert.Equal(t, float64(77), response["run_id"])
	})

	t.Run("api error", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"POST /repos/owner/repo/actions/runs/77/rerun": notFoundHandler("run not found"),
		})

		result, _, err := rerunWorkflowRun(context.Background(), client, "owner", "repo", 77)
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestRerunFailedJobs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"POST /repos/owner/repo/actions/runs/88/rerun-failed-jobs": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
			},
		})

		result, _, err := rerunFailedJobs(context.Background(), client, "owner", "repo", 88)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var response map[string]any
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
		assert.Equal(t, "Failed jobs have been queued for re-run", response["message"])
		assert.Equal(t, float64(88), response["run_id"])
	})

	t.Run("api error", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"POST /repos/owner/repo/actions/runs/88/rerun-failed-jobs": notFoundHandler("run not found"),
		})

		result, _, err := rerunFailedJobs(context.Background(), client, "owner", "repo", 88)
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestDeleteWorkflowRunLogs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"DELETE /repos/owner/repo/actions/runs/99/logs": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
		})

		result, _, err := deleteWorkflowRunLogs(context.Background(), client, "owner", "repo", 99)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var response map[string]any
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
		assert.Equal(t, "Workflow run logs have been deleted", response["message"])
		assert.Equal(t, float64(99), response["run_id"])
	})

	t.Run("api error", func(t *testing.T) {
		client := newActionsTestClient(t, map[string]http.HandlerFunc{
			"DELETE /repos/owner/repo/actions/runs/99/logs": notFoundHandler("run not found"),
		})

		result, _, err := deleteWorkflowRunLogs(context.Background(), client, "owner", "repo", 99)
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}
