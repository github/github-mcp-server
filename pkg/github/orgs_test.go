package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_CreateOrgInvitation(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := CreateOrgInvitation(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "create_org_invitation", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "org")
	assert.Contains(t, tool.InputSchema.Properties, "invitee_id")
	assert.Contains(t, tool.InputSchema.Properties, "email")
	assert.Contains(t, tool.InputSchema.Properties, "role")
	assert.Contains(t, tool.InputSchema.Properties, "team_ids")

	// Verify required parameters
	assert.Contains(t, tool.InputSchema.Required, "org")

	// Setup mock data for test cases
	createdAt := time.Now()
	createdInvitation := &github.Invitation{
		ID:                github.Ptr(int64(1)),
		Login:             github.Ptr("octocat"),
		Email:             github.Ptr("octocat@github.com"),
		Role:              github.Ptr("direct_member"),
		CreatedAt:         &github.Timestamp{Time: createdAt},
		InvitationTeamURL: github.Ptr("https://api.github.com/organizations/1/invitations/1/teams"),
		Inviter: &github.User{
			Login: github.Ptr("admin"),
		},
	}

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]interface{}
		expectError        bool
		expectedErrMsg     string
		expectedInvitation *github.Invitation
	}{
		{
			name: "create invitation with email successfully",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostOrgsInvitationsByOrg,
					mockResponse(t, http.StatusCreated, createdInvitation),
				),
			),
			requestArgs: map[string]interface{}{
				"org":   "test-org",
				"email": "octocat@github.com",
				"role":  "direct_member",
			},
			expectError:        false,
			expectedInvitation: createdInvitation,
		},
		{
			name: "create invitation with invitee_id successfully",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostOrgsInvitationsByOrg,
					mockResponse(t, http.StatusCreated, createdInvitation),
				),
			),
			requestArgs: map[string]interface{}{
				"org":        "test-org",
				"invitee_id": float64(123456),
				"role":       "admin",
			},
			expectError:        false,
			expectedInvitation: createdInvitation,
		},
		{
			name: "create invitation with team_ids",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostOrgsInvitationsByOrg,
					mockResponse(t, http.StatusCreated, createdInvitation),
				),
			),
			requestArgs: map[string]interface{}{
				"org":      "test-org",
				"email":    "octocat@github.com",
				"role":     "direct_member",
				"team_ids": []interface{}{float64(12), float64(26)},
			},
			expectError:        false,
			expectedInvitation: createdInvitation,
		},
		{
			name:         "missing required org parameter",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"email": "octocat@github.com",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: org",
		},
		{
			name:         "missing both invitee_id and email",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"org": "test-org",
			},
			expectError:    true,
			expectedErrMsg: "either invitee_id or email must be provided",
		},
		{
			name:         "invalid team_ids format",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"org":      "test-org",
				"email":    "octocat@github.com",
				"team_ids": []interface{}{"12", "abc"},
			},
			expectError:    true,
			expectedErrMsg: "invalid team_id",
		},
		{
			name: "api returns unauthorized error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostOrgsInvitationsByOrg,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnauthorized)
						_, _ = w.Write([]byte(`{"message": "Must have admin rights to organization"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org":   "test-org",
				"email": "octocat@github.com",
			},
			expectError:    true,
			expectedErrMsg: "failed to create organization invitation",
		},
		{
			name: "api returns validation error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostOrgsInvitationsByOrg,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org":   "test-org",
				"email": "invalid-email",
			},
			expectError:    true,
			expectedErrMsg: "failed to create organization invitation",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := CreateOrgInvitation(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
				} else {
					// For errors returned as part of the result, not as an error
					assert.NotNil(t, result)
					textContent := getTextResult(t, result)
					assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Unmarshal and verify the invitation result
			var invitation struct {
				ID                 int64  `json:"id"`
				Login              string `json:"login,omitempty"`
				Email              string `json:"email,omitempty"`
				Role               string `json:"role"`
				InvitationTeamsURL string `json:"invitation_teams_url"`
				CreatedAt          string `json:"created_at"`
				InviterLogin       string `json:"inviter_login,omitempty"`
			}
			err = json.Unmarshal([]byte(textContent.Text), &invitation)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedInvitation.GetID(), invitation.ID)
			assert.Equal(t, tc.expectedInvitation.GetRole(), invitation.Role)
			if tc.expectedInvitation.GetEmail() != "" {
				assert.Equal(t, tc.expectedInvitation.GetEmail(), invitation.Email)
			}
			if tc.expectedInvitation.GetLogin() != "" {
				assert.Equal(t, tc.expectedInvitation.GetLogin(), invitation.Login)
			}
		})
	}
}
