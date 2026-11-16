package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/requeststate"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// automationGroupTestDeps serves responses keyed by method and request URI and records each request with its body.
func automationGroupTestDeps(t *testing.T, responses map[string]string) (BaseDeps, *[]string) {
	t.Helper()
	var requests []string
	httpClient := MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, agentAutomationAPIVersion, r.Header.Get("X-GitHub-Api-Version"))
		var body []byte
		if r.Body != nil {
			body, _ = io.ReadAll(r.Body)
		}
		requests = append(requests, strings.TrimSpace(r.Method+" "+r.URL.RequestURI()+" "+string(body)))
		response, ok := responses[r.Method+" "+r.URL.RequestURI()]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(response))
	})
	sealer, err := requeststate.NewRandom()
	require.NoError(t, err)
	return BaseDeps{Client: mustNewGHClient(t, httpClient), StateSealer: sealer}, &requests
}

func callAutomationGroupTool(t *testing.T, tool func(translations.TranslationHelperFunc) inventory.ServerTool, deps BaseDeps, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	request := createMCPRequest(args)
	serverTool := tool(translations.NullTranslationHelper)
	result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	return result
}

func TestListAutomationGroups(t *testing.T) {
	t.Parallel()

	var queries []string
	httpClient := MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/agents/repos/octo-org/octo-repo/automation-tags", r.URL.Path)
		assert.Equal(t, agentAutomationAPIVersion, r.Header.Get("X-GitHub-Api-Version"))
		queries = append(queries, r.URL.RawQuery)
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", `<https://api.github.com/agents/repos/octo-org/octo-repo/automation-tags?page=2&per_page=100>; rel="next"`)
			_, _ = w.Write([]byte(`{"tags":[{"id":"group-1","name":"<b>Release</b>","version":"1"}],"total_count":2}`))
			return
		}
		_, _ = w.Write([]byte(`{"tags":[{"id":"group-2","name":"Triage","version":"4"}],"total_count":2}`))
	})
	deps := BaseDeps{Client: mustNewGHClient(t, httpClient)}

	result := callAutomationGroupTool(t, ListAutomationGroups, deps, map[string]any{"owner": "octo-org", "repo": "octo-repo"})
	require.False(t, result.IsError, getTextResult(t, result).Text)
	assert.Equal(t, []string{"page=1&per_page=100", "page=2&per_page=100"}, queries)
	assert.JSONEq(t, `{
		"repository":{"owner":"octo-org","repo":"octo-repo"},
		"groups":[{"id":"group-1","name":"Release","version":"1"},{"id":"group-2","name":"Triage","version":"4"}],
		"groupingAvailable":true
	}`, getTextResult(t, result).Text)
}

func TestListAutomationGroupsReportsUnavailableGrouping(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		status    int
		wantError bool
	}{
		{name: "hidden or disabled", status: http.StatusNotFound},
		{name: "unexpected failure", status: http.StatusInternalServerError, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			httpClient := MockHTTPClientWithHandler(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			})
			deps := BaseDeps{Client: mustNewGHClient(t, httpClient)}

			result := callAutomationGroupTool(t, ListAutomationGroups, deps, map[string]any{"owner": "octo-org", "repo": "octo-repo"})
			if tc.wantError {
				require.True(t, result.IsError)
				return
			}
			require.False(t, result.IsError, getTextResult(t, result).Text)
			var output struct {
				Groups            []any  `json:"groups"`
				GroupingAvailable bool   `json:"groupingAvailable"`
				GroupingError     string `json:"groupingError"`
			}
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &output))
			assert.NotNil(t, output.Groups)
			assert.Empty(t, output.Groups)
			assert.False(t, output.GroupingAvailable)
			assert.NotEmpty(t, output.GroupingError)
		})
	}
}

func TestGetAutomationGroup(t *testing.T) {
	t.Parallel()

	deps, requests := automationGroupTestDeps(t, map[string]string{
		"GET /agents/repos/octo-org/octo-repo/automation-tags/group-1":                                                        `{"id":"group-1","name":"<b>Release</b>","description":"Release automations","version":"3"}`,
		"GET /agents/repos/octo-org/octo-repo/automations?is_tag_membership_included=true&page=1&per_page=100&tag_id=group-1": `{"automations":[{"id":"automation-1","name":"Ship\u202E it","tag_ids":["group-1"],"unavailable_tag_ids":[],"tag_membership_version":"7"}],"total_count":1}`,
	})

	result := callAutomationGroupTool(t, GetAutomationGroup, deps, map[string]any{"owner": "octo-org", "repo": "octo-repo", "group_id": "group-1"})
	require.False(t, result.IsError, getTextResult(t, result).Text)
	assert.Len(t, *requests, 2)
	membershipNotice, err := json.Marshal(automationGroupMembershipNotice)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"repository":{"owner":"octo-org","repo":"octo-repo"},
		"group":{"id":"group-1","name":"Release","description":"Release automations","version":"3"},
		"automations":[{"id":"automation-1","name":"Ship it","tag_ids":["group-1"],"unavailable_tag_ids":[],"tag_membership_version":"7"}],
		"membershipNotice":`+string(membershipNotice)+`
	}`, getTextResult(t, result).Text)
}

func TestCreateAutomationGroup(t *testing.T) {
	t.Parallel()

	created := `{"id":"group-2","name":"Release","description":"Release automations","version":"1"}`
	var createBody map[string]string
	httpClient := MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/agents/repos/octo-org/octo-repo/automation-tags", r.URL.Path)
		assert.Equal(t, agentAutomationAPIVersion, r.Header.Get("X-GitHub-Api-Version"))
		if r.Method == http.MethodPost {
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&createBody))
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(created))
			return
		}
		assert.Equal(t, "page=1&per_page=100", r.URL.RawQuery)
		_, _ = w.Write([]byte(`{"tags":[{"id":"group-1","name":"Triage","version":"4"},` + created + `],"total_count":2}`))
	})
	sealer, err := requeststate.NewRandom()
	require.NoError(t, err)
	deps := BaseDeps{Client: mustNewGHClient(t, httpClient), StateSealer: sealer}

	result := invokeConfirmedAutomation(t, CreateAutomationGroup(translations.NullTranslationHelper), deps,
		map[string]any{"owner": "octo-org", "repo": "octo-repo", "name": "Release", "description": "Release automations"},
		agentAutomationConfirmationIDPrefix+"create_group", "CREATE GROUP octo-org/octo-repo", nil, []string{"Release"})
	require.False(t, result.IsError, getTextResult(t, result).Text)
	assert.NotEmpty(t, createBody["idempotency_key"])
	delete(createBody, "idempotency_key")
	assert.Equal(t, map[string]string{"name": "Release", "description": "Release automations"}, createBody)
	assert.JSONEq(t, `{"status":"created","group":`+created+`}`, getTextResult(t, result).Text)
}

func TestAutomationGroupMembershipTools(t *testing.T) {
	t.Parallel()

	const (
		groupRead      = "GET /agents/repos/octo-org/octo-repo/automation-tags/group-1"
		membershipRead = "GET /agents/repos/octo-org/octo-repo/automations/automation-1?is_tag_membership_included=true"
		attach         = "PUT /agents/repos/octo-org/octo-repo/automations/automation-1/tags/group-1"
		detach         = "DELETE /agents/repos/octo-org/octo-repo/automations/automation-1/tags/group-1?expected_tag_membership_version=7"
	)
	confirmations := map[string][2]string{
		"add_automation_to_group":      {agentAutomationConfirmationIDPrefix + "add_to_group", "ADD octo-org/octo-repo/automation-1 TO GROUP group-1"},
		"remove_automation_from_group": {agentAutomationConfirmationIDPrefix + "remove_from_group", "REMOVE octo-org/octo-repo/automation-1 FROM GROUP group-1"},
	}
	for _, tc := range []struct {
		name       string
		tool       func(translations.TranslationHelperFunc) inventory.ServerTool
		membership string
		mutation   string
		body       string
		response   string
		wantStatus string
		wantError  string
	}{
		{
			name:       "add",
			tool:       AddAutomationToGroup,
			membership: `{"id":"automation-1","tag_ids":[],"unavailable_tag_ids":[],"tag_membership_version":"7"}`,
			mutation:   attach,
			body:       `{"expected_tag_membership_version":"7"}`,
			response:   `{"automation_id":"automation-1","tag_ids":["group-1"],"unavailable_tag_ids":[],"version":"8"}`,
			wantStatus: "added",
		},
		{
			name:       "remove",
			tool:       RemoveAutomationFromGroup,
			membership: `{"id":"automation-1","tag_ids":["group-1"],"unavailable_tag_ids":[],"tag_membership_version":"7"}`,
			mutation:   detach,
			response:   `{"automation_id":"automation-1","tag_ids":[],"unavailable_tag_ids":[],"version":"8"}`,
			wantStatus: "removed",
		},
		{
			name:       "add refuses an automation that references another group",
			tool:       AddAutomationToGroup,
			membership: `{"id":"automation-1","tag_ids":[],"unavailable_tag_ids":["group-0"],"tag_membership_version":"7"}`,
			wantError:  "already has a group",
		},
		{
			name:       "add refuses incomplete membership",
			tool:       AddAutomationToGroup,
			membership: `{"id":"automation-1"}`,
			wantError:  "invalid group membership",
		},
		{
			name:       "remove refuses a non-member",
			tool:       RemoveAutomationFromGroup,
			membership: `{"id":"automation-1","tag_ids":["group-0"],"unavailable_tag_ids":[],"tag_membership_version":"7"}`,
			wantError:  "already in place",
		},
		{
			name:       "add rejects an unconfirmed response",
			tool:       AddAutomationToGroup,
			membership: `{"id":"automation-1","tag_ids":[],"unavailable_tag_ids":[],"tag_membership_version":"7"}`,
			mutation:   attach,
			body:       `{"expected_tag_membership_version":"7"}`,
			response:   `{"automation_id":"automation-1","tag_ids":[],"unavailable_tag_ids":[],"version":"8"}`,
			wantError:  "did not confirm",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			responses := map[string]string{
				groupRead:      `{"id":"group-1","name":"Release","version":"3"}`,
				membershipRead: tc.membership,
			}
			wantRequests := []string{groupRead, membershipRead}
			if tc.mutation != "" {
				responses[tc.mutation] = tc.response
				wantRequests = append(wantRequests, strings.TrimSpace(tc.mutation+" "+tc.body))
			}
			deps, requests := automationGroupTestDeps(t, responses)
			tool := tc.tool(translations.NullTranslationHelper)
			confirmation := confirmations[tool.Tool.Name]

			result := invokeConfirmedAutomation(t, tool, deps, map[string]any{
				"owner": "octo-org", "repo": "octo-repo", "automation_id": "automation-1", "group_id": "group-1",
			}, confirmation[0], confirmation[1], nil, nil)
			assert.Equal(t, wantRequests, *requests)
			if tc.wantError != "" {
				assert.Contains(t, getErrorResult(t, result).Text, tc.wantError)
				return
			}
			require.False(t, result.IsError, getTextResult(t, result).Text)
			assert.JSONEq(t, `{"status":"`+tc.wantStatus+`","membership":`+tc.response+`,"automationDeleted":false,"scheduleChanged":false}`, getTextResult(t, result).Text)
		})
	}
}

func TestAutomationGroupMutationsRequireConfirmation(t *testing.T) {
	t.Parallel()

	membershipArgs := map[string]any{"owner": "octo-org", "repo": "octo-repo", "automation_id": "automation-1", "group_id": "group-1"}
	for _, mutation := range []struct {
		tool         func(translations.TranslationHelperFunc) inventory.ServerTool
		args         map[string]any
		confirmID    string
		confirmValue string
	}{
		{
			tool:         CreateAutomationGroup,
			args:         map[string]any{"owner": "octo-org", "repo": "octo-repo", "name": "Release"},
			confirmID:    agentAutomationConfirmationIDPrefix + "create_group",
			confirmValue: "CREATE GROUP octo-org/octo-repo",
		},
		{
			tool:         AddAutomationToGroup,
			args:         membershipArgs,
			confirmID:    agentAutomationConfirmationIDPrefix + "add_to_group",
			confirmValue: "ADD octo-org/octo-repo/automation-1 TO GROUP group-1",
		},
		{
			tool:         RemoveAutomationFromGroup,
			args:         membershipArgs,
			confirmID:    agentAutomationConfirmationIDPrefix + "remove_from_group",
			confirmValue: "REMOVE octo-org/octo-repo/automation-1 FROM GROUP group-1",
		},
	} {
		for _, tc := range []struct {
			name        string
			unprotected bool
			action      string
			value       string
			wantError   string
		}{
			{name: "without request-state protection", unprotected: true, wantError: "request-state protection is not configured"},
			{name: "declined", action: "decline", value: mutation.confirmValue, wantError: "was not confirmed"},
			{name: "incorrect confirmation", action: "accept", value: strings.ToLower(mutation.confirmValue), wantError: "did not match"},
		} {
			tool := mutation.tool(translations.NullTranslationHelper)
			t.Run(tool.Tool.Name+" "+tc.name, func(t *testing.T) {
				t.Parallel()
				deps, requests := automationGroupTestDeps(t, nil)
				if tc.unprotected {
					deps.StateSealer = nil
				}
				handler := tool.Handler(deps)
				request := createMCPRequest(mutation.args)
				result, err := handler(ContextWithDeps(context.Background(), deps), &request)
				require.NoError(t, err)
				if !tc.unprotected {
					retry := createMCPRequest(mutation.args)
					retry.Params.RequestState = result.RequestState
					retry.Params.InputResponses = mcp.InputResponseMap{
						mutation.confirmID: &mcp.ElicitResult{
							Action:  tc.action,
							Content: map[string]any{agentAutomationConfirmationField: tc.value},
						},
					}
					result, err = handler(ContextWithDeps(context.Background(), deps), &retry)
					require.NoError(t, err)
				}
				assert.Contains(t, getErrorResult(t, result).Text, tc.wantError)
				assert.Empty(t, *requests)
			})
		}
	}
}

func TestAutomationGroupToolsRejectUnsafeInput(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		tool func(translations.TranslationHelperFunc) inventory.ServerTool
		args map[string]any
	}{
		{name: "list with dot owner", tool: ListAutomationGroups, args: map[string]any{"owner": ".", "repo": "octo-repo"}},
		{name: "get without group ID", tool: GetAutomationGroup, args: map[string]any{"owner": "octo-org", "repo": "octo-repo"}},
		{name: "get with dot-dot group ID", tool: GetAutomationGroup, args: map[string]any{"owner": "octo-org", "repo": "octo-repo", "group_id": ".."}},
		{name: "create with dot-dot repo", tool: CreateAutomationGroup, args: map[string]any{"owner": "octo-org", "repo": "..", "name": "Release"}},
		{name: "create with blank name", tool: CreateAutomationGroup, args: map[string]any{"owner": "octo-org", "repo": "octo-repo", "name": " "}},
		{
			name: "add with reserved automation ID",
			tool: AddAutomationToGroup,
			args: map[string]any{"owner": "octo-org", "repo": "octo-repo", "automation_id": "configuration", "group_id": "group-1"},
		},
		{
			name: "remove with dot group ID",
			tool: RemoveAutomationFromGroup,
			args: map[string]any{"owner": "octo-org", "repo": "octo-repo", "automation_id": "automation-1", "group_id": "."},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deps, requests := automationGroupTestDeps(t, nil)
			result := callAutomationGroupTool(t, tc.tool, deps, tc.args)
			require.True(t, result.IsError)
			assert.Empty(t, *requests)
		})
	}
}
