package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_CheckOrgMembership(t *testing.T) {
	serverTool := CheckOrgMembership(translations.NullTranslationHelper)
	tool := serverTool.Tool

	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "check_org_membership", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "org")
	assert.Contains(t, schema.Properties, "username")
	assert.ElementsMatch(t, schema.Required, []string{"org", "username"})
	assert.True(t, serverTool.IsReadOnly())
	assert.ElementsMatch(t, []string{"read:org"}, serverTool.RequiredScopes)
}

func Test_CheckOrgMembership_PublicMember(t *testing.T) {
	output := runCheckOrgMembership(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetOrgsMembersByOrgByUsername:       expectPath(t, "/orgs/canonical/members/octocat").andThen(mockStatus(http.StatusNoContent)),
		GetOrgsPublicMembersByOrgByUsername: expectPath(t, "/orgs/canonical/public_members/octocat").andThen(mockStatus(http.StatusNoContent)),
	}), false)

	assert.Equal(t, CheckOrgMembershipOutput{
		Org:        "canonical",
		Username:   "octocat",
		IsMember:   true,
		Visibility: "public",
	}, output)
}

func Test_CheckOrgMembership_PrivateMember(t *testing.T) {
	output := runCheckOrgMembership(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetOrgsMembersByOrgByUsername:       expectPath(t, "/orgs/canonical/members/octocat").andThen(mockStatus(http.StatusNoContent)),
		GetOrgsPublicMembersByOrgByUsername: expectPath(t, "/orgs/canonical/public_members/octocat").andThen(mockStatus(http.StatusNotFound)),
	}), false)

	assert.Equal(t, CheckOrgMembershipOutput{
		Org:        "canonical",
		Username:   "octocat",
		IsMember:   true,
		Visibility: "private",
	}, output)
}

func Test_CheckOrgMembership_NotAMember(t *testing.T) {
	output := runCheckOrgMembership(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetOrgsMembersByOrgByUsername:       expectPath(t, "/orgs/canonical/members/octocat").andThen(mockStatus(http.StatusNotFound)),
		GetOrgsPublicMembersByOrgByUsername: expectPath(t, "/orgs/canonical/public_members/octocat").andThen(mockStatus(http.StatusNotFound)),
		GetOrgsByOrg:                        expectPath(t, "/orgs/canonical").andThen(mockResponse(t, http.StatusOK, &gogithub.Organization{Login: gogithub.Ptr("canonical")})),
	}), false)

	assert.Equal(t, "canonical", output.Org)
	assert.Equal(t, "octocat", output.Username)
	assert.False(t, output.IsMember)
	assert.Equal(t, "none", output.Visibility)
	assert.Equal(t, "result reflects caller visibility; private members of orgs you can't see appear as non-members.", output.Note)
}

func Test_CheckOrgMembership_OrgNotFound(t *testing.T) {
	text := runCheckOrgMembershipError(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetOrgsMembersByOrgByUsername:       expectPath(t, "/orgs/missing-org/members/octocat").andThen(mockStatus(http.StatusNotFound)),
		GetOrgsPublicMembersByOrgByUsername: expectPath(t, "/orgs/missing-org/public_members/octocat").andThen(mockStatus(http.StatusNotFound)),
		GetOrgsByOrg:                        expectPath(t, "/orgs/missing-org").andThen(mockResponse(t, http.StatusNotFound, map[string]string{"message": "Not Found"})),
	}), map[string]any{"org": "missing-org", "username": "octocat"})

	assert.Contains(t, text, "failed to get organization")
	assert.Contains(t, text, "404")
}

func Test_CheckOrgMembership_ScopeError(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			text := runCheckOrgMembershipError(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetOrgsMembersByOrgByUsername: expectPath(t, "/orgs/canonical/members/octocat").andThen(
					mockResponse(t, status, map[string]string{"message": http.StatusText(status)}),
				),
			}), map[string]any{"org": "canonical", "username": "octocat"})

			assert.Contains(t, text, "failed to check organization membership")
			assert.Contains(t, text, http.StatusText(status))
		})
	}
}

func runCheckOrgMembership(t *testing.T, httpClient *http.Client, expectError bool) CheckOrgMembershipOutput {
	t.Helper()

	result := callCheckOrgMembership(t, httpClient, map[string]any{"org": "canonical", "username": "octocat"})
	require.Equal(t, expectError, result.IsError)

	textContent := getTextResult(t, result)
	var output CheckOrgMembershipOutput
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &output))
	return output
}

func runCheckOrgMembershipError(t *testing.T, httpClient *http.Client, args map[string]any) string {
	t.Helper()

	result := callCheckOrgMembership(t, httpClient, args)
	textContent := getErrorResult(t, result)
	return textContent.Text
}

func callCheckOrgMembership(t *testing.T, httpClient *http.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	client := gogithub.NewClient(httpClient)
	deps := BaseDeps{Client: client}
	serverTool := CheckOrgMembership(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)
	request := createMCPRequest(args)

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	return result
}

func mockStatus(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}
}
