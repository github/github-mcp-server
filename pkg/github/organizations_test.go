package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v69/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListOrganizations(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := ListOrganizations(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_organizations", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")

	// Setup mock orgs for success case
	mockOrgs := []*github.Organization{
		{
			Login:     github.Ptr("org1"),
			NodeID:    github.Ptr("node1"),
			AvatarURL: github.Ptr("https://github.com/images/org1.png"),
			HTMLURL:   github.Ptr("https://github.com/org1"),
		},
		{
			Login:     github.Ptr("org2"),
			NodeID:    github.Ptr("node2"),
			AvatarURL: github.Ptr("https://github.com/images/org2.png"),
			HTMLURL:   github.Ptr("https://github.com/org2"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedOrgs   []*github.Organization
		expectedErrMsg string
	}{
		{
			name: "successful orgs fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetUserOrgs,
					mockOrgs,
				),
			),
			requestArgs: map[string]interface{}{
				"page":    float64(1),
				"perPage": float64(10),
			},
			expectError:  false,
			expectedOrgs: mockOrgs,
		},
		{
			name: "orgs fetch fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserOrgs,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnauthorized)
						_, _ = w.Write([]byte(`{"message": "Unauthorized"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"page":    float64(1),
				"perPage": float64(10),
			},
			expectError:    true,
			expectedErrMsg: "failed to list organizations",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListOrganizations(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedOrgs []*github.Organization
			err = json.Unmarshal([]byte(textContent.Text), &returnedOrgs)
			require.NoError(t, err)
			require.Equal(t, len(tc.expectedOrgs), len(returnedOrgs))

			for i, expectedOrg := range tc.expectedOrgs {
				assert.Equal(t, *expectedOrg.Login, *returnedOrgs[i].Login)
				assert.Equal(t, *expectedOrg.NodeID, *returnedOrgs[i].NodeID)
				assert.Equal(t, *expectedOrg.HTMLURL, *returnedOrgs[i].HTMLURL)
			}
		})
	}
}

func Test_GetOrganization(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetOrganization(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_organization", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "org")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"org"})

	// Setup mock org for success case
	mockOrg := &github.Organization{
		Login:       github.Ptr("testorg"),
		NodeID:      github.Ptr("node123"),
		Name:        github.Ptr("Test Organization"),
		Description: github.Ptr("This is a test organization"),
		AvatarURL:   github.Ptr("https://github.com/images/testorg.png"),
		HTMLURL:     github.Ptr("https://github.com/testorg"),
		Location:    github.Ptr("San Francisco"),
		Blog:        github.Ptr("https://testorg.com"),
		Email:       github.Ptr("info@testorg.com"),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedOrg    *github.Organization
		expectedErrMsg string
	}{
		{
			name: "successful org fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetOrgsByOrg,
					mockOrg,
				),
			),
			requestArgs: map[string]interface{}{
				"org": "testorg",
			},
			expectError: false,
			expectedOrg: mockOrg,
		},
		{
			name: "org fetch fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetOrgsByOrg,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org": "nonexistentorg",
			},
			expectError:    true,
			expectedErrMsg: "failed to get organization",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetOrganization(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedOrg github.Organization
			err = json.Unmarshal([]byte(textContent.Text), &returnedOrg)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedOrg.Login, *returnedOrg.Login)
			assert.Equal(t, *tc.expectedOrg.Name, *returnedOrg.Name)
			assert.Equal(t, *tc.expectedOrg.Description, *returnedOrg.Description)
			assert.Equal(t, *tc.expectedOrg.HTMLURL, *returnedOrg.HTMLURL)
		})
	}
}

