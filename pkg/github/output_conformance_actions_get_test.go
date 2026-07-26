package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	gogithub "github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/pkg/translations"
)

// Endpoints actions_get reaches that helper_test.go has no constant for.
// Named with the tool prefix so they cannot collide with a constant another
// test file adds later.
const (
	actionsgetEndpointJobByID     = "GET /repos/{owner}/{repo}/actions/jobs/{job_id}"
	actionsgetEndpointArtifactZip = "GET /repos/{owner}/{repo}/actions/artifacts/{artifact_id}/zip"
)

// actionsgetRedirect reproduces the 302 + Location dance GitHub uses for the
// two endpoints that hand back a short-lived download URL rather than a body.
// go-github returns the Location verbatim without following it (maxRedirects
// only follows 301), so this is the whole of the real protocol.
func actionsgetRedirect(location string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", location)
		w.WriteHeader(http.StatusFound)
	}
}

// The end-to-end guarantee for actions_get: what the real handler emits must
// validate against the outputSchema the tool advertises. The structured-content
// mirror publishes the exact bytes of the single text block as
// structuredContent, so validating the text block here is equivalent to
// validating what a client receives.
//
// Every value of the tool's `method` enum is covered, and the enum itself is
// pinned below, so adding a method without adding a case fails the test rather
// than silently shipping an unvalidated branch.
func TestActionsGetOutputValidatesAgainstDeclaredSchema(t *testing.T) {
	t.Parallel()

	serverTool := ActionsGet(translations.NullTranslationHelper)
	resolved := resolveToolSchema(t, actionsGetOutputSchema)

	ts := gogithub.Timestamp{Time: time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)}
	user := &gogithub.User{
		Login:     gogithub.Ptr("octocat"),
		ID:        gogithub.Ptr(int64(583231)),
		NodeID:    gogithub.Ptr("MDQ6VXNlcjU4MzIzMQ=="),
		AvatarURL: gogithub.Ptr("https://avatars.githubusercontent.com/u/583231?v=4"),
		HTMLURL:   gogithub.Ptr("https://github.com/octocat"),
		URL:       gogithub.Ptr("https://api.github.com/users/octocat"),
		Type:      gogithub.Ptr("User"),
	}
	repo := &gogithub.Repository{
		ID:       gogithub.Ptr(int64(1296269)),
		NodeID:   gogithub.Ptr("MDEwOlJlcG9zaXRvcnkxMjk2MjY5"),
		Name:     gogithub.Ptr("repo"),
		FullName: gogithub.Ptr("owner/repo"),
		Private:  gogithub.Ptr(false),
		URL:      gogithub.Ptr("https://api.github.com/repos/owner/repo"),
		HTMLURL:  gogithub.Ptr("https://github.com/owner/repo"),
		Owner:    user,
	}

	// Every field the get_workflow branch declares, so a property whose type
	// drifts from the go-github struct is caught rather than skipped over.
	fullWorkflow := &gogithub.Workflow{
		ID:        gogithub.Ptr(int64(161335)),
		NodeID:    gogithub.Ptr("MDg6V29ya2Zsb3cxNjEzMzU="),
		Name:      gogithub.Ptr("CI"),
		Path:      gogithub.Ptr(".github/workflows/ci.yml"),
		State:     gogithub.Ptr("active"),
		CreatedAt: &ts,
		UpdatedAt: &ts,
		URL:       gogithub.Ptr("https://api.github.com/repos/owner/repo/actions/workflows/161335"),
		HTMLURL:   gogithub.Ptr("https://github.com/owner/repo/blob/main/.github/workflows/ci.yml"),
		BadgeURL:  gogithub.Ptr("https://github.com/owner/repo/workflows/CI/badge.svg"),
	}

	fullWorkflowRun := &gogithub.WorkflowRun{
		ID:                  gogithub.Ptr(int64(12345)),
		Name:                gogithub.Ptr("CI"),
		NodeID:              gogithub.Ptr("MDEyOldvcmtmbG93IFJ1bjEyMzQ1"),
		HeadBranch:          gogithub.Ptr("main"),
		HeadSHA:             gogithub.Ptr("acb5820ced9479c074f688cc328bf03f341a511d"),
		Path:                gogithub.Ptr(".github/workflows/ci.yml"),
		RunNumber:           gogithub.Ptr(42),
		RunAttempt:          gogithub.Ptr(1),
		Event:               gogithub.Ptr("push"),
		DisplayTitle:        gogithub.Ptr("Fix the thing"),
		Status:              gogithub.Ptr("completed"),
		Conclusion:          gogithub.Ptr("success"),
		WorkflowID:          gogithub.Ptr(int64(161335)),
		CheckSuiteID:        gogithub.Ptr(int64(42)),
		CheckSuiteNodeID:    gogithub.Ptr("MDEwOkNoZWNrU3VpdGU0Mg=="),
		URL:                 gogithub.Ptr("https://api.github.com/repos/owner/repo/actions/runs/12345"),
		HTMLURL:             gogithub.Ptr("https://github.com/owner/repo/actions/runs/12345"),
		PullRequests:        []*gogithub.PullRequest{{Number: gogithub.Ptr(3), Title: gogithub.Ptr("Fix the thing")}},
		CreatedAt:           &ts,
		UpdatedAt:           &ts,
		RunStartedAt:        &ts,
		JobsURL:             gogithub.Ptr("https://api.github.com/repos/owner/repo/actions/runs/12345/jobs"),
		LogsURL:             gogithub.Ptr("https://api.github.com/repos/owner/repo/actions/runs/12345/logs"),
		CheckSuiteURL:       gogithub.Ptr("https://api.github.com/repos/owner/repo/check-suites/42"),
		ArtifactsURL:        gogithub.Ptr("https://api.github.com/repos/owner/repo/actions/runs/12345/artifacts"),
		CancelURL:           gogithub.Ptr("https://api.github.com/repos/owner/repo/actions/runs/12345/cancel"),
		RerunURL:            gogithub.Ptr("https://api.github.com/repos/owner/repo/actions/runs/12345/rerun"),
		PreviousAttemptURL:  gogithub.Ptr("https://api.github.com/repos/owner/repo/actions/runs/12344"),
		WorkflowURL:         gogithub.Ptr("https://api.github.com/repos/owner/repo/actions/workflows/161335"),
		Repository:          repo,
		HeadRepository:      repo,
		Actor:               user,
		TriggeringActor:     user,
		ReferencedWorkflows: []*gogithub.ReferencedWorkflow{{Path: gogithub.Ptr("owner/repo/.github/workflows/reusable.yml@main"), SHA: gogithub.Ptr("acb5820"), Ref: gogithub.Ptr("refs/heads/main")}},
		HeadCommit: &gogithub.HeadCommit{
			ID:        gogithub.Ptr("acb5820ced9479c074f688cc328bf03f341a511d"),
			TreeID:    gogithub.Ptr("d23f6eedb1e1b9610bbc754ddb5197bfe7271223"),
			Message:   gogithub.Ptr("Fix the thing"),
			Timestamp: &ts,
			Author:    &gogithub.CommitAuthor{Name: gogithub.Ptr("Mona"), Email: gogithub.Ptr("mona@github.com")},
			Committer: &gogithub.CommitAuthor{Name: gogithub.Ptr("Mona"), Email: gogithub.Ptr("mona@github.com")},
		},
	}

	fullWorkflowJob := &gogithub.WorkflowJob{
		ID:          gogithub.Ptr(int64(399444496)),
		RunID:       gogithub.Ptr(int64(12345)),
		RunURL:      gogithub.Ptr("https://api.github.com/repos/owner/repo/actions/runs/12345"),
		NodeID:      gogithub.Ptr("MDg6Q2hlY2tSdW4zOTk0NDQ0OTY="),
		HeadBranch:  gogithub.Ptr("main"),
		HeadSHA:     gogithub.Ptr("acb5820ced9479c074f688cc328bf03f341a511d"),
		URL:         gogithub.Ptr("https://api.github.com/repos/owner/repo/actions/jobs/399444496"),
		HTMLURL:     gogithub.Ptr("https://github.com/owner/repo/runs/399444496"),
		Status:      gogithub.Ptr("completed"),
		Conclusion:  gogithub.Ptr("success"),
		CreatedAt:   &ts,
		StartedAt:   &ts,
		CompletedAt: &ts,
		Name:        gogithub.Ptr("build"),
		Steps: []*gogithub.TaskStep{{
			Name:        gogithub.Ptr("Set up job"),
			Status:      gogithub.Ptr("completed"),
			Conclusion:  gogithub.Ptr("success"),
			Number:      gogithub.Ptr(int64(1)),
			StartedAt:   &ts,
			CompletedAt: &ts,
		}},
		CheckRunURL:     gogithub.Ptr("https://api.github.com/repos/owner/repo/check-runs/399444496"),
		Labels:          []string{"ubuntu-latest"},
		RunnerID:        gogithub.Ptr(int64(1)),
		RunnerName:      gogithub.Ptr("my runner"),
		RunnerGroupID:   gogithub.Ptr(int64(2)),
		RunnerGroupName: gogithub.Ptr("my runner group"),
		RunAttempt:      gogithub.Ptr(int64(1)),
		WorkflowName:    gogithub.Ptr("CI"),
	}

	tests := []struct {
		name       string
		method     string
		resourceID string
		handlers   map[string]http.HandlerFunc
	}{
		{
			name:       "get_workflow",
			method:     actionsMethodGetWorkflow,
			resourceID: "161335",
			handlers: map[string]http.HandlerFunc{
				GetReposActionsWorkflowsByOwnerByRepoByWorkflowID: mockResponse(t, http.StatusOK, fullWorkflow),
			},
		},
		{
			// get_workflow accepts a file name as well as a numeric ID, which
			// dispatches to GetWorkflowByFileName instead of GetWorkflowByID.
			// Same declared branch, different code path into it.
			name:       "get_workflow by file name",
			method:     actionsMethodGetWorkflow,
			resourceID: "ci.yml",
			handlers: map[string]http.HandlerFunc{
				GetReposActionsWorkflowsByOwnerByRepoByWorkflowID: mockResponse(t, http.StatusOK, fullWorkflow),
			},
		},
		{
			// Sparsest payload the branch can legally accept: every field of
			// go-github's Workflow is a pointer with omitempty, so all but one
			// drop out of the marshalled object. `state` rather than `id` on
			// purpose, so a non-first singleton-required branch is exercised.
			// A fully empty {} is deliberately NOT accepted by the schema
			// (see TestPolymorphicOutputSchemasAreNotVacuous) and GitHub never
			// returns one for this endpoint.
			name:       "get_workflow with all optionals absent",
			method:     actionsMethodGetWorkflow,
			resourceID: "161335",
			handlers: map[string]http.HandlerFunc{
				GetReposActionsWorkflowsByOwnerByRepoByWorkflowID: mockResponse(t, http.StatusOK,
					&gogithub.Workflow{State: gogithub.Ptr("disabled_manually")}),
			},
		},
		{
			name:       "get_workflow_run",
			method:     actionsMethodGetWorkflowRun,
			resourceID: "12345",
			handlers: map[string]http.HandlerFunc{
				GetReposActionsRunsByOwnerByRepoByRunID: mockResponse(t, http.StatusOK, fullWorkflowRun),
			},
		},
		{
			// A run still in flight: `conclusion` is absent, which is the one
			// optional the schema documents as routinely missing.
			name:       "get_workflow_run in progress without conclusion",
			method:     actionsMethodGetWorkflowRun,
			resourceID: "12345",
			handlers: map[string]http.HandlerFunc{
				GetReposActionsRunsByOwnerByRepoByRunID: mockResponse(t, http.StatusOK, &gogithub.WorkflowRun{
					ID:           gogithub.Ptr(int64(12345)),
					Status:       gogithub.Ptr("in_progress"),
					RunStartedAt: &ts,
				}),
			},
		},
		{
			// Empty collections upstream: pull_requests and
			// referenced_workflows come back as [], and go-github's omitempty
			// drops them entirely on the way out. Proves the round trip still
			// conforms rather than emitting `null` for either.
			name:       "get_workflow_run with empty collections",
			method:     actionsMethodGetWorkflowRun,
			resourceID: "12345",
			handlers: map[string]http.HandlerFunc{
				GetReposActionsRunsByOwnerByRepoByRunID: mockResponse(t, http.StatusOK,
					`{"id":12345,"status":"queued","pull_requests":[],"referenced_workflows":[]}`),
			},
		},
		{
			name:       "get_workflow_run with all optionals absent",
			method:     actionsMethodGetWorkflowRun,
			resourceID: "12345",
			handlers: map[string]http.HandlerFunc{
				GetReposActionsRunsByOwnerByRepoByRunID: mockResponse(t, http.StatusOK,
					&gogithub.WorkflowRun{Status: gogithub.Ptr("queued")}),
			},
		},
		{
			name:       "get_workflow_job",
			method:     actionsMethodGetWorkflowJob,
			resourceID: "399444496",
			handlers: map[string]http.HandlerFunc{
				actionsgetEndpointJobByID: mockResponse(t, http.StatusOK, fullWorkflowJob),
			},
		},
		{
			// The other empty-collection case: a job with zero steps and zero
			// runner labels.
			name:       "get_workflow_job with empty steps and labels",
			method:     actionsMethodGetWorkflowJob,
			resourceID: "399444496",
			handlers: map[string]http.HandlerFunc{
				actionsgetEndpointJobByID: mockResponse(t, http.StatusOK,
					`{"id":399444496,"status":"queued","steps":[],"labels":[]}`),
			},
		},
		{
			name:       "get_workflow_job with all optionals absent",
			method:     actionsMethodGetWorkflowJob,
			resourceID: "399444496",
			handlers: map[string]http.HandlerFunc{
				actionsgetEndpointJobByID: mockResponse(t, http.StatusOK,
					&gogithub.WorkflowJob{Conclusion: gogithub.Ptr("cancelled")}),
			},
		},
		{
			name:       "download_workflow_run_artifact",
			method:     actionsMethodDownloadWorkflowArtifact,
			resourceID: "11",
			handlers: map[string]http.HandlerFunc{
				actionsgetEndpointArtifactZip: actionsgetRedirect("https://pipelines.actions.githubusercontent.com/artifact/11.zip"),
			},
		},
		{
			// Degenerate but reachable: a 302 carrying no Location parses to
			// the empty URL, so download_url is "". The branch's required set
			// still has to hold — the server builds this object itself, so
			// every key is present regardless.
			name:       "download_workflow_run_artifact with empty location",
			method:     actionsMethodDownloadWorkflowArtifact,
			resourceID: "11",
			handlers: map[string]http.HandlerFunc{
				actionsgetEndpointArtifactZip: actionsgetRedirect(""),
			},
		},
		{
			name:       "get_workflow_run_usage",
			method:     actionsMethodGetWorkflowRunUsage,
			resourceID: "12345",
			handlers: map[string]http.HandlerFunc{
				GetReposActionsRunsTimingByOwnerByRepoByRunID: mockResponse(t, http.StatusOK, &gogithub.WorkflowRunUsage{
					Billable: &gogithub.WorkflowRunBillMap{
						"UBUNTU": {
							TotalMS: gogithub.Ptr(int64(180000)),
							Jobs:    gogithub.Ptr(1),
							JobRuns: []*gogithub.WorkflowRunJobRun{{JobID: gogithub.Ptr(1), DurationMS: gogithub.Ptr(int64(180000))}},
						},
					},
					RunDurationMS: gogithub.Ptr(int64(500000)),
				}),
			},
		},
		{
			// The empty-collection case that actually survives marshalling:
			// `billable` is a *pointer* to a map, so omitempty keeps it and an
			// empty map emits `"billable":{}`. run_duration_ms is absent, so
			// the payload leans on the second singleton-required branch alone.
			name:       "get_workflow_run_usage with nothing billable",
			method:     actionsMethodGetWorkflowRunUsage,
			resourceID: "12345",
			handlers: map[string]http.HandlerFunc{
				GetReposActionsRunsTimingByOwnerByRepoByRunID: mockResponse(t, http.StatusOK,
					&gogithub.WorkflowRunUsage{Billable: &gogithub.WorkflowRunBillMap{}}),
			},
		},
		{
			// A billable entry with every optional absent, plus an empty
			// job_runs array upstream.
			name:       "get_workflow_run_usage with sparse billable entry",
			method:     actionsMethodGetWorkflowRunUsage,
			resourceID: "12345",
			handlers: map[string]http.HandlerFunc{
				GetReposActionsRunsTimingByOwnerByRepoByRunID: mockResponse(t, http.StatusOK,
					`{"billable":{"MACOS":{"job_runs":[]}}}`),
			},
		},
		{
			name:       "get_workflow_run_logs_url",
			method:     actionsMethodGetWorkflowRunLogsURL,
			resourceID: "12345",
			handlers: map[string]http.HandlerFunc{
				GetReposActionsRunsLogsByOwnerByRepoByRunID: actionsgetRedirect("https://pipelines.actions.githubusercontent.com/logs/12345.zip"),
			},
		},
		{
			name:       "get_workflow_run_logs_url with empty location",
			method:     actionsMethodGetWorkflowRunLogsURL,
			resourceID: "12345",
			handlers: map[string]http.HandlerFunc{
				GetReposActionsRunsLogsByOwnerByRepoByRunID: actionsgetRedirect(""),
			},
		},
	}

	// Pin the enum: a new method must gain a case above, not slip through.
	covered := map[string]bool{}
	for _, tt := range tests {
		covered[tt.method] = true
	}
	inputSchema, ok := serverTool.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "input schema should be a *jsonschema.Schema")
	for _, m := range inputSchema.Properties["method"].Enum {
		assert.True(t, covered[m.(string)],
			"method %q is advertised in the enum but has no output-conformance case", m)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(tt.handlers))}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(map[string]any{
				"method":      tt.method,
				"owner":       "owner",
				"repo":        "repo",
				"resource_id": tt.resourceID,
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, "handler should succeed")

			text := getTextResult(t, result)

			var payload any
			require.NoError(t, json.Unmarshal([]byte(text.Text), &payload),
				"handler output must be JSON for the mirror to publish it as structuredContent")

			require.NoError(t, resolved.Validate(payload),
				"real handler output for method=%s must conform to the advertised outputSchema", tt.method)
		})
	}
}

// go-github decodes these endpoints into a *T, so a 200 whose body is JSON
// null leaves the pointer nil and json.Marshal emits the literal `null` —
// which cannot validate against an object-rooted schema. Same class of bug as
// TestGetSubIssuesNeverEmitsNull.
func TestActionsGetNeverEmitsNull(t *testing.T) {
	t.Parallel()

	nullBody := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`null`))
	}

	tests := []struct {
		name     string
		method   string
		endpoint string
	}{
		{"get_workflow_run", "get_workflow_run", GetReposActionsRunsByOwnerByRepoByRunID},
		{"get_workflow_job", "get_workflow_job", actionsgetEndpointJobByID},
		{"get_workflow_run_usage", "get_workflow_run_usage", GetReposActionsRunsTimingByOwnerByRepoByRunID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(
				map[string]http.HandlerFunc{tt.endpoint: nullBody}))}

			request := createMCPRequest(map[string]any{
				"method": tt.method, "owner": "octocat", "repo": "repo", "resource_id": float64(1),
			})
			serverTool := ActionsGet(translations.NullTranslationHelper)
			result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			text := getTextResult(t, result)
			assert.NotEqual(t, "null", strings.TrimSpace(text.Text),
				"a nil pointer must not be marshalled straight to the client")
			assert.True(t, result.IsError,
				"an empty body is an API anomaly and should surface as an error, not a null payload")
		})
	}
}
