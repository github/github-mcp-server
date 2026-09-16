package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// issueEventsDeps builds the dependencies for an issue_read call, optionally with lockdown mode
// enabled and backed by a permission server where "maintainer" has push access and every other
// login does not.
func issueEventsDeps(t *testing.T, mockedClient *http.Client, lockdownEnabled bool) BaseDeps {
	t.Helper()

	var restClient *github.Client
	if lockdownEnabled {
		restClient = mockRESTPermissionServer(t, "read", map[string]string{
			"maintainer": "write",
			"outsider":   "read",
		})
	}

	return BaseDeps{
		Client:          mustNewGHClient(t, mockedClient),
		GQLClient:       defaultGQLClient,
		RepoAccessCache: stubRepoAccessCache(restClient, 15*time.Minute),
		Flags:           stubFeatureFlags(map[string]bool{"lockdown-mode": lockdownEnabled}),
	}
}

func Test_IssueReadWithEvents_ToolDefinition(t *testing.T) {
	t.Parallel()

	serverTool := IssueReadWithEvents(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name+"_ff_"+FeatureFlagIssueEvents, tool))

	assert.Equal(t, "issue_read", tool.Name)
	assert.Equal(t, FeatureFlagIssueEvents, serverTool.FeatureFlagEnable)
	assert.True(t, tool.Annotations.ReadOnlyHint)

	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "event_id")
	assert.Subset(t, schema.Properties["method"].Enum, []any{"get_events", "get_timeline", "get_event"})

	// issue_number leaves the required list on this variant so get_event, which is addressed by
	// event_id, does not have to invent one.
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo"})
}

// Test_IssueRead_WithoutEventsFlag asserts the ungated tool is untouched by this feature: it
// neither advertises nor serves the event history methods.
func Test_IssueRead_WithoutEventsFlag(t *testing.T) {
	t.Parallel()

	serverTool := IssueRead(translations.NullTranslationHelper)
	schema := serverTool.Tool.InputSchema.(*jsonschema.Schema)

	assert.Equal(t, []string{FeatureFlagIssueEvents}, serverTool.FeatureFlagDisable)
	assert.Empty(t, serverTool.FeatureFlagEnable)
	assert.NotContains(t, schema.Properties, "event_id")
	assert.NotContains(t, schema.Properties["method"].Enum, "get_events")
	assert.NotContains(t, schema.Properties["method"].Enum, "get_timeline")
	assert.NotContains(t, schema.Properties["method"].Enum, "get_event")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "issue_number"})

	for _, method := range []string{"get_events", "get_timeline", "get_event"} {
		t.Run(method, func(t *testing.T) {
			deps := issueEventsDeps(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}), false)
			request := createMCPRequest(map[string]any{
				"method":       method,
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"event_id":     float64(1),
			})

			result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			assert.Contains(t, getErrorResult(t, result).Text, "unknown method: "+method)
		})
	}
}

func Test_GetIssueEvents(t *testing.T) {
	serverTool := IssueReadWithEvents(translations.NullTranslationHelper)

	mockEvents := []*github.IssueEvent{
		{
			ID:        github.Ptr(int64(123)),
			Event:     github.Ptr("labeled"),
			Actor:     &github.User{Login: github.Ptr("maintainer")},
			Label:     &github.Label{Name: "bug"},
			CreatedAt: &github.Timestamp{Time: time.Date(2025, 5, 22, 10, 0, 0, 0, time.UTC)},
		},
		{
			ID:       github.Ptr(int64(456)),
			Event:    github.Ptr("renamed"),
			Actor:    &github.User{Login: github.Ptr("outsider")},
			Rename:   &github.Rename{From: github.Ptr("old title"), To: github.Ptr("new title")},
			CommitID: github.Ptr("abc123"),
		},
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		requestArgs     map[string]any
		lockdownEnabled bool
		expectError     bool
		expectedErrMsg  string
		expectedEvents  []MinimalIssueEvent
	}{
		{
			name: "successful events retrieval",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesEventsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockEvents),
			}),
			requestArgs: map[string]any{
				"method":       "get_events",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectedEvents: []MinimalIssueEvent{
				{
					ID:        123,
					Event:     "labeled",
					Label:     "bug",
					CreatedAt: "2025-05-22T10:00:00Z",
				},
				{
					ID:          456,
					Event:       "renamed",
					CommitID:    "abc123",
					RenamedFrom: "old title",
					RenamedTo:   "new title",
				},
			},
		},
		{
			name: "successful events retrieval with pagination",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesEventsByOwnerByRepoByIssueNumber: expectQueryParams(t, map[string]string{
					"page":     "2",
					"per_page": "10",
				}).andThen(
					mockResponse(t, http.StatusOK, mockEvents),
				),
			}),
			requestArgs: map[string]any{
				"method":       "get_events",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"page":         float64(2),
				"perPage":      float64(10),
			},
			expectedEvents: []MinimalIssueEvent{{ID: 123}, {ID: 456}},
		},
		{
			name: "issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesEventsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			}),
			requestArgs: map[string]any{
				"method":       "get_events",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get issue events",
		},
		{
			name: "missing issue_number",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesEventsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockEvents),
			}),
			requestArgs: map[string]any{
				"method": "get_events",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectedErrMsg: "missing required parameter: issue_number",
		},
		{
			name: "lockdown filters events from actors without push access",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesEventsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockEvents),
			}),
			requestArgs: map[string]any{
				"method":       "get_events",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			lockdownEnabled: true,
			expectedEvents: []MinimalIssueEvent{
				{ID: 123, Event: "labeled", Label: "bug", CreatedAt: "2025-05-22T10:00:00Z"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := issueEventsDeps(t, tc.mockedClient, tc.lockdownEnabled)
			request := createMCPRequest(tc.requestArgs)

			result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)

			if tc.expectedEvents == nil {
				assert.Contains(t, getErrorResult(t, result).Text, tc.expectedErrMsg)
				return
			}

			var returned []MinimalIssueEvent
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
			require.Len(t, returned, len(tc.expectedEvents))
			for i, expected := range tc.expectedEvents {
				assert.Equal(t, expected.ID, returned[i].ID)
				if expected.Event != "" {
					assert.Equal(t, expected.Event, returned[i].Event)
					assert.Equal(t, expected.Label, returned[i].Label)
					assert.Equal(t, expected.CommitID, returned[i].CommitID)
					assert.Equal(t, expected.RenamedFrom, returned[i].RenamedFrom)
					assert.Equal(t, expected.RenamedTo, returned[i].RenamedTo)
					assert.Equal(t, expected.CreatedAt, returned[i].CreatedAt)
				}
			}
		})
	}
}

func Test_GetIssueTimeline(t *testing.T) {
	serverTool := IssueReadWithEvents(translations.NullTranslationHelper)

	mockTimeline := []*github.Timeline{
		{
			ID:        github.Ptr(int64(123)),
			Event:     github.Ptr("commented"),
			User:      &github.User{Login: github.Ptr("maintainer")},
			Body:      github.Ptr("A comment on the issue"),
			CreatedAt: &github.Timestamp{Time: time.Date(2025, 5, 22, 10, 0, 0, 0, time.UTC)},
		},
		{
			ID:    github.Ptr(int64(456)),
			Event: github.Ptr("cross-referenced"),
			Actor: &github.User{Login: github.Ptr("maintainer")},
			Source: &github.Source{
				Type:  github.Ptr("issue"),
				Actor: &github.User{Login: github.Ptr("maintainer")},
				Issue: &github.Issue{
					Number:  github.Ptr(7),
					Title:   github.Ptr("Referring issue"),
					State:   github.Ptr("open"),
					HTMLURL: github.Ptr("https://github.com/owner/repo/issues/7"),
				},
			},
		},
		{
			ID:      github.Ptr(int64(789)),
			Event:   github.Ptr("committed"),
			Actor:   &github.User{Login: github.Ptr("outsider")},
			SHA:     github.Ptr("def456"),
			Message: github.Ptr("Fix the thing"),
		},
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		requestArgs     map[string]any
		lockdownEnabled bool
		expectError     bool
		expectedErrMsg  string
		expectedIDs     []int64
	}{
		{
			name: "successful timeline retrieval",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesTimelineByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockTimeline),
			}),
			requestArgs: map[string]any{
				"method":       "get_timeline",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectedIDs: []int64{123, 456, 789},
		},
		{
			name: "successful timeline retrieval with pagination",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesTimelineByOwnerByRepoByIssueNumber: expectQueryParams(t, map[string]string{
					"page":     "3",
					"per_page": "25",
				}).andThen(
					mockResponse(t, http.StatusOK, mockTimeline),
				),
			}),
			requestArgs: map[string]any{
				"method":       "get_timeline",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"page":         float64(3),
				"perPage":      float64(25),
			},
			expectedIDs: []int64{123, 456, 789},
		},
		{
			name: "issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesTimelineByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			}),
			requestArgs: map[string]any{
				"method":       "get_timeline",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get issue timeline",
		},
		{
			// The `commented` entry is attributed via User rather than Actor, so this also
			// covers the User fallback in eventActorLogin.
			name: "lockdown filters timeline entries from users without push access",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesTimelineByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockTimeline),
			}),
			requestArgs: map[string]any{
				"method":       "get_timeline",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			lockdownEnabled: true,
			expectedIDs:     []int64{123, 456},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := issueEventsDeps(t, tc.mockedClient, tc.lockdownEnabled)
			request := createMCPRequest(tc.requestArgs)

			result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}
			require.NoError(t, err)

			var returned []MinimalTimelineItem
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))

			ids := make([]int64, 0, len(returned))
			for _, item := range returned {
				ids = append(ids, item.ID)
			}
			assert.Equal(t, tc.expectedIDs, ids)
		})
	}
}

// Test_GetIssueTimeline_TrimsAndSanitizes pins the minimal shape: the comment body and the
// cross-referenced issue title survive, while the verbose upstream nesting does not.
func Test_GetIssueTimeline_TrimsAndSanitizes(t *testing.T) {
	t.Parallel()

	serverTool := IssueReadWithEvents(translations.NullTranslationHelper)
	deps := issueEventsDeps(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposIssuesTimelineByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, []*github.Timeline{
			{
				ID:    github.Ptr(int64(1)),
				Event: github.Ptr("cross-referenced"),
				Actor: &github.User{Login: github.Ptr("maintainer")},
				Source: &github.Source{
					Type: github.Ptr("issue"),
					Issue: &github.Issue{
						Number:     github.Ptr(7),
						Title:      github.Ptr("Referring issue"),
						State:      github.Ptr("open"),
						HTMLURL:    github.Ptr("https://github.com/other/repo/issues/7"),
						Repository: &github.Repository{FullName: github.Ptr("other/repo")},
					},
				},
			},
		}),
	}), false)

	request := createMCPRequest(map[string]any{
		"method":       "get_timeline",
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(42),
	})

	result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)

	var returned []MinimalTimelineItem
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
	require.Len(t, returned, 1)

	require.NotNil(t, returned[0].Source)
	require.NotNil(t, returned[0].Source.Issue)
	assert.Equal(t, "issue", returned[0].Source.Type)
	assert.Equal(t, 7, returned[0].Source.Issue.Number)
	assert.Equal(t, "Referring issue", returned[0].Source.Issue.Title)
	assert.Equal(t, "other/repo", returned[0].Source.Issue.Repository)

	// The trimmed entry must not carry the upstream commit-ancestry payload.
	var raw []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &raw))
	assert.NotContains(t, raw[0], "parents")
}

func Test_GetIssueEvent(t *testing.T) {
	serverTool := IssueReadWithEvents(translations.NullTranslationHelper)

	// The single-event endpoint is the one that returns the issue the event belongs to, since
	// the caller addresses the event by id and never supplies an issue number.
	mockEvent := &github.IssueEvent{
		ID:        github.Ptr(int64(17196710688)),
		Event:     github.Ptr("closed"),
		Actor:     &github.User{Login: github.Ptr("maintainer")},
		CommitID:  github.Ptr("abc123"),
		CreatedAt: &github.Timestamp{Time: time.Date(2025, 5, 22, 10, 0, 0, 0, time.UTC)},
		// No nested Repository: the REST payload carries repository_url instead, so the ref's
		// Repository stays empty and the issue is understood to be in the addressed repo.
		Issue: &github.Issue{
			Number:  github.Ptr(42),
			Title:   github.Ptr("Something broke"),
			State:   github.Ptr("closed"),
			HTMLURL: github.Ptr("https://github.com/owner/repo/issues/42"),
		},
	}

	tests := []struct {
		name             string
		mockedClient     *http.Client
		requestArgs      map[string]any
		lockdownEnabled  bool
		expectError      bool
		expectResultErr  bool
		expectedErrMsg   string
		expectedEventID  int64
		expectedActor    string
		expectedCommitID string
		expectedIssueNum int
	}{
		{
			name: "successful single event retrieval",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesEventByOwnerByRepoByEventID: mockResponse(t, http.StatusOK, mockEvent),
			}),
			requestArgs: map[string]any{
				"method":   "get_event",
				"owner":    "owner",
				"repo":     "repo",
				"event_id": float64(17196710688),
			},
			expectedEventID:  17196710688,
			expectedActor:    "maintainer",
			expectedCommitID: "abc123",
			expectedIssueNum: 42,
		},
		{
			// get_event takes no issue_number, which is the reason it is left out of the
			// schema-level required list.
			name: "succeeds without issue_number",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesEventByOwnerByRepoByEventID: mockResponse(t, http.StatusOK, mockEvent),
			}),
			requestArgs: map[string]any{
				"method":   "get_event",
				"owner":    "owner",
				"repo":     "repo",
				"event_id": float64(17196710688),
			},
			expectedEventID:  17196710688,
			expectedActor:    "maintainer",
			expectedCommitID: "abc123",
			expectedIssueNum: 42,
		},
		{
			name: "missing event_id",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesEventByOwnerByRepoByEventID: mockResponse(t, http.StatusOK, mockEvent),
			}),
			requestArgs: map[string]any{
				"method": "get_event",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectResultErr: true,
			expectedErrMsg:  "missing required parameter: event_id",
		},
		{
			name: "event not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesEventByOwnerByRepoByEventID: mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			}),
			requestArgs: map[string]any{
				"method":   "get_event",
				"owner":    "owner",
				"repo":     "repo",
				"event_id": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get issue event",
		},
		{
			name: "lockdown allows event from actor with push access",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesEventByOwnerByRepoByEventID: mockResponse(t, http.StatusOK, mockEvent),
			}),
			requestArgs: map[string]any{
				"method":   "get_event",
				"owner":    "owner",
				"repo":     "repo",
				"event_id": float64(17196710688),
			},
			lockdownEnabled:  true,
			expectedEventID:  17196710688,
			expectedActor:    "maintainer",
			expectedCommitID: "abc123",
			expectedIssueNum: 42,
		},
		{
			// A single event cannot be filtered down, so lockdown refuses the read outright.
			name: "lockdown rejects event from actor without push access",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesEventByOwnerByRepoByEventID: mockResponse(t, http.StatusOK, &github.IssueEvent{
					ID:    github.Ptr(int64(555)),
					Event: github.Ptr("renamed"),
					Actor: &github.User{Login: github.Ptr("outsider")},
				}),
			}),
			requestArgs: map[string]any{
				"method":   "get_event",
				"owner":    "owner",
				"repo":     "repo",
				"event_id": float64(555),
			},
			lockdownEnabled: true,
			expectResultErr: true,
			expectedErrMsg:  "access to issue event is restricted by lockdown mode",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := issueEventsDeps(t, tc.mockedClient, tc.lockdownEnabled)
			request := createMCPRequest(tc.requestArgs)

			result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)

			if tc.expectResultErr {
				assert.Contains(t, getErrorResult(t, result).Text, tc.expectedErrMsg)
				return
			}

			var returned MinimalIssueEvent
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
			assert.Equal(t, tc.expectedEventID, returned.ID)
			require.NotNil(t, returned.Actor)
			assert.Equal(t, tc.expectedActor, returned.Actor.Login)
			assert.Equal(t, tc.expectedCommitID, returned.CommitID)
			if tc.expectedIssueNum == 0 {
				assert.Nil(t, returned.Issue)
			} else {
				require.NotNil(t, returned.Issue)
				assert.Equal(t, tc.expectedIssueNum, returned.Issue.Number)
				assert.Equal(t, "Something broke", returned.Issue.Title)
				assert.Empty(t, returned.Issue.Repository)
			}
		})
	}
}

func Test_repoFullNameFromCommitURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "cross-repo commit on a referenced event",
			url:      "https://api.github.com/repos/saraycp/open-build-service/commits/5291b58b04",
			expected: "saraycp/open-build-service",
		},
		{
			name:     "fork branch on a force-push event",
			url:      "https://api.github.com/repos/artemsaveliev/github-mcp-server/commits/79b2a855ab",
			expected: "artemsaveliev/github-mcp-server",
		},
		{
			name:     "GHES host",
			url:      "https://github.example.com/api/v3/repos/owner/repo/commits/abc123",
			expected: "owner/repo",
		},
		{name: "empty url", url: "", expected: ""},
		{name: "no repos segment", url: "https://api.github.com/user", expected: ""},
		{name: "owner only", url: "https://api.github.com/repos/owner", expected: ""},
		{name: "trailing slash after owner", url: "https://api.github.com/repos/owner/", expected: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, repoFullNameFromCommitURL(tc.url))
		})
	}
}

func Test_GetIssueTimeline_CarriesCommitRepository(t *testing.T) {
	t.Parallel()

	// A bare commit_id is unresolvable when the commit lives outside this repo, which is the
	// common case for referenced and force-push entries.
	serverTool := IssueReadWithEvents(translations.NullTranslationHelper)
	deps := issueEventsDeps(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposIssuesTimelineByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, []*github.Timeline{
			{
				Event:     github.Ptr("referenced"),
				Actor:     &github.User{Login: github.Ptr("outsider")},
				CommitID:  github.Ptr("5291b58b04"),
				CommitURL: github.Ptr("https://api.github.com/repos/other/unrelated/commits/5291b58b04"),
			},
			{
				Event:    github.Ptr("labeled"),
				Actor:    &github.User{Login: github.Ptr("maintainer")},
				Label:    &github.Label{Name: "bug"},
				CommitID: github.Ptr(""),
			},
		}),
	}), false)

	request := createMCPRequest(map[string]any{
		"method":       "get_timeline",
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(42),
	})

	result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)

	var returned []MinimalTimelineItem
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
	require.Len(t, returned, 2)

	assert.Equal(t, "5291b58b04", returned[0].CommitID)
	assert.Equal(t, "other/unrelated", returned[0].CommitRepository)

	// Entries with no commit stay clean rather than carrying an empty key.
	assert.Empty(t, returned[1].CommitRepository)
	var raw []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &raw))
	assert.NotContains(t, raw[1], "commit_repository")
}

func Test_GetIssueEvent_CarriesDismissedReview(t *testing.T) {
	t.Parallel()

	serverTool := IssueReadWithEvents(translations.NullTranslationHelper)
	deps := issueEventsDeps(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposIssuesEventByOwnerByRepoByEventID: mockResponse(t, http.StatusOK, &github.IssueEvent{
			ID:    github.Ptr(int64(99)),
			Event: github.Ptr("review_dismissed"),
			Actor: &github.User{Login: github.Ptr("maintainer")},
			DismissedReview: &github.DismissedReview{
				State:             github.Ptr("changes_requested"),
				ReviewID:          github.Ptr(int64(456)),
				DismissalMessage:  github.Ptr("Stale after <script>alert(1)</script> rebase"),
				DismissalCommitID: github.Ptr("def456"),
			},
		}),
	}), false)

	request := createMCPRequest(map[string]any{
		"method":   "get_event",
		"owner":    "owner",
		"repo":     "repo",
		"event_id": float64(99),
	})

	result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)

	var returned MinimalIssueEvent
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))

	require.NotNil(t, returned.DismissedReview)
	assert.Equal(t, "changes_requested", returned.DismissedReview.State)
	assert.Equal(t, int64(456), returned.DismissedReview.ReviewID)
	assert.Equal(t, "def456", returned.DismissedReview.DismissalCommitID)
	// The dismissal message is author-supplied, so it is sanitized like any other body text.
	assert.NotContains(t, returned.DismissedReview.DismissalMessage, "<script>")
}

func Test_GetIssueTimeline_CarriesCommitAuthorship(t *testing.T) {
	t.Parallel()

	// A committed entry has neither actor nor user, so the commit object's author is the only
	// attribution a caller can get for it.
	serverTool := IssueReadWithEvents(translations.NullTranslationHelper)
	deps := issueEventsDeps(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposIssuesTimelineByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, []*github.Timeline{
			{
				Event:   github.Ptr("committed"),
				SHA:     github.Ptr("f2d79f988e"),
				Message: github.Ptr("Fix the thing"),
				Author: &github.CommitAuthor{
					Name:  github.Ptr("Artem Saveliev"),
					Email: github.Ptr("artem@example.com"),
					Date:  &github.Timestamp{Time: time.Date(2025, 4, 10, 6, 45, 46, 0, time.UTC)},
				},
				Committer: &github.CommitAuthor{
					Name:  github.Ptr("GitHub"),
					Email: github.Ptr("noreply@github.com"),
				},
			},
		}),
	}), false)

	request := createMCPRequest(map[string]any{
		"method":       "get_timeline",
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(42),
	})

	result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)

	var returned []MinimalTimelineItem
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
	require.Len(t, returned, 1)

	require.NotNil(t, returned[0].Author)
	assert.Equal(t, "Artem Saveliev", returned[0].Author.Name)
	assert.Equal(t, "artem@example.com", returned[0].Author.Email)
	assert.Equal(t, "2025-04-10T06:45:46Z", returned[0].Author.Date)

	require.NotNil(t, returned[0].Committer)
	assert.Equal(t, "GitHub", returned[0].Committer.Name)
	assert.Empty(t, returned[0].Committer.Date)

	// Commit ancestry stays dropped even though authorship is now surfaced.
	var raw []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &raw))
	assert.NotContains(t, raw[0], "parents")
}
