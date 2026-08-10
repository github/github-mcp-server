package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	gogithub "github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/pkg/translations"
)

// The end-to-end guarantee for actions_list: what the real handler emits must
// validate against the outputSchema the tool advertises. The structured-content
// mirror publishes the exact bytes of the text block as structuredContent, so
// validating the text block here is equivalent to validating what a client
// receives.
//
// Every value of the tool's `method` enum gets a subtest, and each one covers
// the awkward payloads as well as the happy path: an empty collection (which
// go-github's omitempty tags reduce to {"total_count":0} with the item array
// absent) and a maximally sparse envelope/item (one field present, every other
// optional field gone) — those are what actually exercise the schema's required
// sets rather than a convenient, fully-populated fixture.
func TestActionsListOutputValidatesAgainstDeclaredSchema(t *testing.T) {
	t.Parallel()

	serverTool := ActionsList(translations.NullTranslationHelper)
	resolved := resolveToolSchema(t, actionsListOutputSchema)

	ts := gogithub.Timestamp{Time: time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)}

	user := &gogithub.User{
		Login:     gogithub.Ptr("octocat"),
		ID:        gogithub.Ptr(int64(1)),
		NodeID:    gogithub.Ptr("U_kgDOAAAAAQ"),
		Type:      gogithub.Ptr("User"),
		AvatarURL: gogithub.Ptr("https://avatars.githubusercontent.com/u/1"),
		HTMLURL:   gogithub.Ptr("https://github.com/octocat"),
	}
	repo := &gogithub.Repository{
		ID:       gogithub.Ptr(int64(10)),
		NodeID:   gogithub.Ptr("R_kgDOAAAACg"),
		Name:     gogithub.Ptr("repo"),
		FullName: gogithub.Ptr("octocat/repo"),
		Private:  gogithub.Ptr(false),
		HTMLURL:  gogithub.Ptr("https://github.com/octocat/repo"),
		URL:      gogithub.Ptr("https://api.github.com/repos/octocat/repo"),
		Owner:    user,
	}

	// Every documented field populated, including the nested $defs
	// (repository/user/headCommit/commitAuthor/referencedWorkflow/pullRequest).
	richRun := &gogithub.WorkflowRun{
		ID:               gogithub.Ptr(int64(123)),
		Name:             gogithub.Ptr("CI"),
		NodeID:           gogithub.Ptr("WFR_kwDOAAAAew"),
		HeadBranch:       gogithub.Ptr("main"),
		HeadSHA:          gogithub.Ptr("6dcb09b5b57875f334f61aebed695e2e4193db5e"),
		Path:             gogithub.Ptr(".github/workflows/ci.yml"),
		RunNumber:        gogithub.Ptr(7),
		RunAttempt:       gogithub.Ptr(1),
		Event:            gogithub.Ptr("push"),
		DisplayTitle:     gogithub.Ptr("Fix the flaky test"),
		Status:           gogithub.Ptr("completed"),
		Conclusion:       gogithub.Ptr("success"),
		WorkflowID:       gogithub.Ptr(int64(1)),
		CheckSuiteID:     gogithub.Ptr(int64(42)),
		CheckSuiteNodeID: gogithub.Ptr("CS_kwDOAAAAKg"),
		URL:              gogithub.Ptr("https://api.github.com/repos/octocat/repo/actions/runs/123"),
		HTMLURL:          gogithub.Ptr("https://github.com/octocat/repo/actions/runs/123"),
		PullRequests: []*gogithub.PullRequest{{
			ID:     gogithub.Ptr(int64(9)),
			Number: gogithub.Ptr(3),
			URL:    gogithub.Ptr("https://api.github.com/repos/octocat/repo/pulls/3"),
		}},
		CreatedAt:          &ts,
		UpdatedAt:          &ts,
		RunStartedAt:       &ts,
		JobsURL:            gogithub.Ptr("https://api.github.com/repos/octocat/repo/actions/runs/123/jobs"),
		LogsURL:            gogithub.Ptr("https://api.github.com/repos/octocat/repo/actions/runs/123/logs"),
		CheckSuiteURL:      gogithub.Ptr("https://api.github.com/repos/octocat/repo/check-suites/42"),
		ArtifactsURL:       gogithub.Ptr("https://api.github.com/repos/octocat/repo/actions/runs/123/artifacts"),
		CancelURL:          gogithub.Ptr("https://api.github.com/repos/octocat/repo/actions/runs/123/cancel"),
		RerunURL:           gogithub.Ptr("https://api.github.com/repos/octocat/repo/actions/runs/123/rerun"),
		PreviousAttemptURL: gogithub.Ptr("https://api.github.com/repos/octocat/repo/actions/runs/122"),
		HeadCommit: &gogithub.HeadCommit{
			ID:        gogithub.Ptr("6dcb09b5b57875f334f61aebed695e2e4193db5e"),
			SHA:       gogithub.Ptr("6dcb09b5b57875f334f61aebed695e2e4193db5e"),
			TreeID:    gogithub.Ptr("827efc6d56897b048c772eb4087f854f46256132"),
			Message:   gogithub.Ptr("Fix the flaky test"),
			Timestamp: &ts,
			URL:       gogithub.Ptr("https://github.com/octocat/repo/commit/6dcb09b"),
			Distinct:  gogithub.Ptr(true),
			Author:    &gogithub.CommitAuthor{Name: gogithub.Ptr("Mona"), Email: gogithub.Ptr("mona@github.com"), Date: &ts},
			Committer: &gogithub.CommitAuthor{Name: gogithub.Ptr("Mona"), Email: gogithub.Ptr("mona@github.com"), Date: &ts},
			Added:     []string{"a.go"},
			Removed:   []string{"b.go"},
			Modified:  []string{"c.go"},
		},
		WorkflowURL:     gogithub.Ptr("https://api.github.com/repos/octocat/repo/actions/workflows/1"),
		Repository:      repo,
		HeadRepository:  repo,
		Actor:           user,
		TriggeringActor: user,
		ReferencedWorkflows: []*gogithub.ReferencedWorkflow{{
			Path: gogithub.Ptr("octocat/reusable/.github/workflows/build.yml@main"),
			SHA:  gogithub.Ptr("6dcb09b5b57875f334f61aebed695e2e4193db5e"),
			Ref:  gogithub.Ptr("refs/heads/main"),
		}},
	}

	richJob := &gogithub.WorkflowJob{
		ID:          gogithub.Ptr(int64(555)),
		RunID:       gogithub.Ptr(int64(456)),
		RunURL:      gogithub.Ptr("https://api.github.com/repos/octocat/repo/actions/runs/456"),
		NodeID:      gogithub.Ptr("J_kwDOAAAAIw"),
		HeadBranch:  gogithub.Ptr("main"),
		HeadSHA:     gogithub.Ptr("6dcb09b5b57875f334f61aebed695e2e4193db5e"),
		URL:         gogithub.Ptr("https://api.github.com/repos/octocat/repo/actions/jobs/555"),
		HTMLURL:     gogithub.Ptr("https://github.com/octocat/repo/actions/runs/456/job/555"),
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
		CheckRunURL:     gogithub.Ptr("https://api.github.com/repos/octocat/repo/check-runs/555"),
		Labels:          []string{"ubuntu-latest"},
		RunnerID:        gogithub.Ptr(int64(1)),
		RunnerName:      gogithub.Ptr("hosted-runner"),
		RunnerGroupID:   gogithub.Ptr(int64(2)),
		RunnerGroupName: gogithub.Ptr("GitHub Actions"),
		RunAttempt:      gogithub.Ptr(int64(1)),
		WorkflowName:    gogithub.Ptr("CI"),
	}

	richArtifact := &gogithub.Artifact{
		ID:                 gogithub.Ptr(int64(777)),
		NodeID:             gogithub.Ptr("A_kwDOAAAAAw"),
		Name:               gogithub.Ptr("build-output"),
		SizeInBytes:        gogithub.Ptr(int64(1024)),
		URL:                gogithub.Ptr("https://api.github.com/repos/octocat/repo/actions/artifacts/777"),
		ArchiveDownloadURL: gogithub.Ptr("https://api.github.com/repos/octocat/repo/actions/artifacts/777/zip"),
		Expired:            gogithub.Ptr(false),
		CreatedAt:          &ts,
		UpdatedAt:          &ts,
		ExpiresAt:          &ts,
		Digest:             gogithub.Ptr("sha256:deadbeef"),
		WorkflowRun: &gogithub.ArtifactWorkflowRun{
			ID:               gogithub.Ptr(int64(456)),
			RepositoryID:     gogithub.Ptr(int64(10)),
			HeadRepositoryID: gogithub.Ptr(int64(10)),
			HeadBranch:       gogithub.Ptr("main"),
			HeadSHA:          gogithub.Ptr("6dcb09b5b57875f334f61aebed695e2e4193db5e"),
		},
	}

	// Local types, so this file cannot collide with helpers other tests in the
	// package define.
	type actionsListCase struct {
		name     string
		args     map[string]any
		handlers map[string]http.HandlerFunc
	}
	type actionsListMethodGroup struct {
		method string
		cases  []actionsListCase
	}

	listWorkflowsArgs := func() map[string]any {
		return map[string]any{"method": actionsMethodListWorkflows, "owner": "octocat", "repo": "repo"}
	}
	listRunsArgs := func(resourceID string) map[string]any {
		a := map[string]any{"method": actionsMethodListWorkflowRuns, "owner": "octocat", "repo": "repo"}
		if resourceID != "" {
			a["resource_id"] = resourceID
		}
		return a
	}
	listJobsArgs := func() map[string]any {
		return map[string]any{"method": actionsMethodListWorkflowJobs, "owner": "octocat", "repo": "repo", "resource_id": "456"}
	}
	listArtifactsArgs := func() map[string]any {
		return map[string]any{"method": actionsMethodListWorkflowArtifacts, "owner": "octocat", "repo": "repo", "resource_id": "456"}
	}

	groups := []actionsListMethodGroup{
		{
			method: actionsMethodListWorkflows,
			cases: []actionsListCase{
				{
					name: "populated",
					args: listWorkflowsArgs(),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsWorkflowsByOwnerByRepo: mockResponse(t, http.StatusOK, &gogithub.Workflows{
							TotalCount: gogithub.Ptr(1),
							Workflows: []*gogithub.Workflow{{
								ID:        gogithub.Ptr(int64(1)),
								NodeID:    gogithub.Ptr("W_kwDOAAAAAQ"),
								Name:      gogithub.Ptr("CI"),
								Path:      gogithub.Ptr(".github/workflows/ci.yml"),
								State:     gogithub.Ptr("active"),
								CreatedAt: &ts,
								UpdatedAt: &ts,
								URL:       gogithub.Ptr("https://api.github.com/repos/octocat/repo/actions/workflows/1"),
								HTMLURL:   gogithub.Ptr("https://github.com/octocat/repo/actions/workflows/ci.yml"),
								BadgeURL:  gogithub.Ptr("https://github.com/octocat/repo/workflows/CI/badge.svg"),
							}},
						}),
					},
				},
				{
					// Empty collection. omitempty drops the zero-length slice
					// entirely, so the wire payload is {"total_count":0} — the
					// envelope's `workflows` key is absent, not [].
					name: "empty collection",
					args: listWorkflowsArgs(),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsWorkflowsByOwnerByRepo: mockResponse(t, http.StatusOK, &gogithub.Workflows{
							TotalCount: gogithub.Ptr(0),
							Workflows:  []*gogithub.Workflow{},
						}),
					},
				},
				{
					// Maximally sparse: the envelope carries only the item
					// array, and the item carries a single field. Exercises the
					// anyOf-over-singleton-required sets rather than a fixture
					// that happens to satisfy all of them at once.
					name: "sparse, all optional fields absent",
					args: listWorkflowsArgs(),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsWorkflowsByOwnerByRepo: mockResponse(t, http.StatusOK, &gogithub.Workflows{
							Workflows: []*gogithub.Workflow{{ID: gogithub.Ptr(int64(1))}},
						}),
					},
				},
			},
		},
		{
			method: actionsMethodListWorkflowRuns,
			cases: []actionsListCase{
				{
					name: "populated, by workflow file name",
					args: listRunsArgs("ci.yml"),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsWorkflowsRunsByOwnerByRepoByWorkflowID: mockResponse(t, http.StatusOK, &gogithub.WorkflowRuns{
							TotalCount:   gogithub.Ptr(1),
							WorkflowRuns: []*gogithub.WorkflowRun{richRun},
						}),
					},
				},
				{
					// No resource_id routes to a different endpoint
					// (ListRepositoryWorkflowRuns) but the same envelope.
					name: "populated, whole repository",
					args: listRunsArgs(""),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsRunsByOwnerByRepo: mockResponse(t, http.StatusOK, &gogithub.WorkflowRuns{
							TotalCount:   gogithub.Ptr(1),
							WorkflowRuns: []*gogithub.WorkflowRun{richRun},
						}),
					},
				},
				{
					name: "empty collection",
					args: listRunsArgs("ci.yml"),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsWorkflowsRunsByOwnerByRepoByWorkflowID: mockResponse(t, http.StatusOK, &gogithub.WorkflowRuns{
							TotalCount:   gogithub.Ptr(0),
							WorkflowRuns: []*gogithub.WorkflowRun{},
						}),
					},
				},
				{
					name: "sparse, all optional fields absent",
					args: listRunsArgs("ci.yml"),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsWorkflowsRunsByOwnerByRepoByWorkflowID: mockResponse(t, http.StatusOK, &gogithub.WorkflowRuns{
							WorkflowRuns: []*gogithub.WorkflowRun{{ID: gogithub.Ptr(int64(123))}},
						}),
					},
				},
				{
					// An in-progress run has no conclusion; the schema must not
					// have quietly made it required.
					name: "run without a conclusion",
					args: listRunsArgs("ci.yml"),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsWorkflowsRunsByOwnerByRepoByWorkflowID: mockResponse(t, http.StatusOK, &gogithub.WorkflowRuns{
							TotalCount: gogithub.Ptr(1),
							WorkflowRuns: []*gogithub.WorkflowRun{{
								ID:     gogithub.Ptr(int64(456)),
								Status: gogithub.Ptr("in_progress"),
							}},
						}),
					},
				},
			},
		},
		{
			method: actionsMethodListWorkflowJobs,
			cases: []actionsListCase{
				{
					// Double-nested: the handler wraps the github.Jobs envelope
					// under an outer `jobs` key.
					name: "populated",
					args: listJobsArgs(),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsRunsJobsByOwnerByRepoByRunID: mockResponse(t, http.StatusOK, &gogithub.Jobs{
							TotalCount: gogithub.Ptr(1),
							Jobs:       []*gogithub.WorkflowJob{richJob},
						}),
					},
				},
				{
					name: "empty collection",
					args: listJobsArgs(),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsRunsJobsByOwnerByRepoByRunID: mockResponse(t, http.StatusOK, &gogithub.Jobs{
							TotalCount: gogithub.Ptr(0),
							Jobs:       []*gogithub.WorkflowJob{},
						}),
					},
				},
				{
					// Every field of the inner envelope absent: {"jobs":{}}.
					// The schema keeps this reachable via maxProperties:0.
					name: "empty inner envelope",
					args: listJobsArgs(),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsRunsJobsByOwnerByRepoByRunID: mockResponse(t, http.StatusOK, &gogithub.Jobs{}),
					},
				},
				{
					// go-github decodes into a **Jobs, so a `null` body leaves
					// the pointer nil and the handler emits {"jobs":null}.
					name: "null envelope from an empty body",
					args: listJobsArgs(),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsRunsJobsByOwnerByRepoByRunID: func(w http.ResponseWriter, _ *http.Request) {
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write([]byte(`null`))
						},
					},
				},
				{
					name: "sparse, all optional fields absent",
					args: listJobsArgs(),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsRunsJobsByOwnerByRepoByRunID: mockResponse(t, http.StatusOK, &gogithub.Jobs{
							Jobs: []*gogithub.WorkflowJob{{ID: gogithub.Ptr(int64(555))}},
						}),
					},
				},
			},
		},
		{
			method: actionsMethodListWorkflowArtifacts,
			cases: []actionsListCase{
				{
					name: "populated",
					args: listArtifactsArgs(),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsRunsArtifactsByOwnerByRepoByRunID: mockResponse(t, http.StatusOK, &gogithub.ArtifactList{
							TotalCount: gogithub.Ptr(int64(1)),
							Artifacts:  []*gogithub.Artifact{richArtifact},
						}),
					},
				},
				{
					name: "empty collection",
					args: listArtifactsArgs(),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsRunsArtifactsByOwnerByRepoByRunID: mockResponse(t, http.StatusOK, &gogithub.ArtifactList{
							TotalCount: gogithub.Ptr(int64(0)),
							Artifacts:  []*gogithub.Artifact{},
						}),
					},
				},
				{
					// digest is null for artifacts from upload-artifact v3 and
					// older, which is the same wire shape as absent here.
					name: "sparse, all optional fields absent",
					args: listArtifactsArgs(),
					handlers: map[string]http.HandlerFunc{
						GetReposActionsRunsArtifactsByOwnerByRepoByRunID: mockResponse(t, http.StatusOK, &gogithub.ArtifactList{
							Artifacts: []*gogithub.Artifact{{ID: gogithub.Ptr(int64(777))}},
						}),
					},
				},
			},
		},
	}

	// Guards against a method being added to the enum without a subtest here.
	require.Len(t, groups, 4, "every value of the actions_list method enum needs a subtest")

	for _, group := range groups {
		t.Run(group.method, func(t *testing.T) {
			for _, tc := range group.cases {
				t.Run(tc.name, func(t *testing.T) {
					deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(tc.handlers))}
					handler := serverTool.Handler(deps)

					request := createMCPRequest(tc.args)
					result, err := handler(ContextWithDeps(context.Background(), deps), &request)
					require.NoError(t, err)
					require.False(t, result.IsError,
						"handler must succeed, otherwise this test would validate an error string instead of real output")

					text := getTextResult(t, result)
					var payload any
					require.NoError(t, json.Unmarshal([]byte(text.Text), &payload),
						"handler output must be JSON for the mirror to publish it as structuredContent: %s", text.Text)

					require.NoError(t, resolved.Validate(payload),
						"real handler output for method=%s must conform to the advertised outputSchema: %s",
						group.method, text.Text)
				})
			}
		})
	}
}

// The enum the tool advertises is the contract this file claims to cover; if a
// method is added, the conformance table above must grow with it.
func TestActionsListMethodEnumIsFullyCovered(t *testing.T) {
	t.Parallel()

	inputSchema := ActionsList(translations.NullTranslationHelper).Tool.InputSchema.(*jsonschema.Schema)
	methodEnum := inputSchema.Properties["method"].Enum

	covered := map[string]bool{
		actionsMethodListWorkflows:         true,
		actionsMethodListWorkflowRuns:      true,
		actionsMethodListWorkflowJobs:      true,
		actionsMethodListWorkflowArtifacts: true,
	}

	require.Len(t, methodEnum, len(covered))
	for _, m := range methodEnum {
		name, ok := m.(string)
		require.True(t, ok)
		require.True(t, covered[name],
			"method %q is in the actions_list enum but has no output-conformance subtest", name)
	}
}
