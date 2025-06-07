package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v72/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListUsersPublicSSHKeys(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := ListUsersPublicSSHKeys(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	assert.Equal(t, "list_users_public_ssh_keys", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")

	// Setup mock results
	mockListSSHKeyResult := []*github.Key{
		{
			ID:       github.Ptr(int64(1)),
			Key:      github.Ptr("ssh test key"),
			URL:      github.Ptr("test url"),
			Title:    github.Ptr("test key 1"),
			ReadOnly: github.Ptr(true),
			Verified: github.Ptr(true),
		},
		{
			ID:       github.Ptr(int64(2)),
			Key:      github.Ptr("ssh test key"),
			URL:      github.Ptr("test url"),
			Title:    github.Ptr("test key 2"),
			ReadOnly: github.Ptr(true),
			Verified: github.Ptr(true),
		},
	}
	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedResult []*github.Key
		expectedErrMsg string
	}{
		{
			name: "list public ssh keys",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserKeys,
					expectQueryParams(t, map[string]string{
						"page":     "2",
						"per_page": "10",
					}).andThen(
						mockResponse(t, http.StatusOK, mockListSSHKeyResult),
					),
				),
			),
			requestArgs: map[string]any{
				"page":    float64(2),
				"perPage": float64(10),
			},
			expectError:    false,
			expectedResult: mockListSSHKeyResult,
		},
		{
			name: "list public ssh keys with default pagination",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserKeys,
					expectQueryParams(t, map[string]string{
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockListSSHKeyResult),
					),
				),
			),
			expectError:    false,
			expectedResult: mockListSSHKeyResult,
		},
		{
			name: "list ssh key fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserKeys,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnauthorized)
						_, _ = w.Write([]byte(`{"message": "bad permission"}`))
					}),
				),
			),
			expectError:    true,
			expectedErrMsg: "failed to list users ssh keys",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListUsersPublicSSHKeys(stubGetClientFn(client), translations.NullTranslationHelper)

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
			var returnedResult []*github.Key
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			assert.Equal(t, len(tc.expectedResult), len(returnedResult))
			for i, keyData := range returnedResult {
				assert.Equal(t, tc.expectedResult[i].ID, keyData.ID)
				assert.Equal(t, tc.expectedResult[i].Title, keyData.Title)
				assert.Equal(t, tc.expectedResult[i].URL, keyData.URL)
				assert.Equal(t, tc.expectedResult[i].Key, keyData.Key)
				assert.Equal(t, tc.expectedResult[i].Verified, keyData.Verified)
				assert.Equal(t, tc.expectedResult[i].ReadOnly, keyData.ReadOnly)
			}
		})
	}
}

func Test_GetUsersPublicSSHKey(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := GetUsersPublicSSHKey(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	assert.Equal(t, "get_users_public_ssh_key", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "key_id")
	assert.NotContains(t, tool.InputSchema.Properties, "page")
	assert.NotContains(t, tool.InputSchema.Properties, "perPage")

	// Setup mock results
	mockGetSSHKeyResult := &github.Key{
		ID:       github.Ptr(int64(1)),
		Key:      github.Ptr("ssh test key"),
		URL:      github.Ptr("test url"),
		Title:    github.Ptr("test key 1"),
		ReadOnly: github.Ptr(true),
		Verified: github.Ptr(true),
	}
	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedResult *github.Key
		expectedErrMsg string
	}{
		{
			name: "get public ssh key",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserKeysByKeyId,
					expectPath(t, "/user/keys/1").
						andThen(
							mockResponse(t, http.StatusOK, mockGetSSHKeyResult),
						),
				),
			),
			requestArgs: map[string]any{
				"key_id": float64(1),
			},
			expectError:    false,
			expectedResult: mockGetSSHKeyResult,
		},
		{
			name: "get public ssh key with bad wrong key",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserKeysByKeyId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "key not found"}`))
					}),
				),
			),
			requestArgs: map[string]any{
				"key_id": float64(2),
			},
			expectError:    true,
			expectedErrMsg: "failed to get ssh key details",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetUsersPublicSSHKey(stubGetClientFn(client), translations.NullTranslationHelper)

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
			var returnedResult *github.Key
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult.ID, returnedResult.ID)
			assert.Equal(t, tc.expectedResult.Key, returnedResult.Key)
			assert.Equal(t, tc.expectedResult.Title, returnedResult.Title)
			assert.Equal(t, tc.expectedResult.URL, returnedResult.URL)
			assert.Equal(t, tc.expectedResult.Verified, returnedResult.Verified)
			assert.Equal(t, tc.expectedResult.ReadOnly, returnedResult.ReadOnly)
		})
	}
}

func Test_AddUsersPublicSSHKey(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := AddUsersPublicSSHKey(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	assert.Equal(t, "add_users_public_ssh_key", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "title")
	assert.Contains(t, tool.InputSchema.Properties, "key")
	assert.NotContains(t, tool.InputSchema.Properties, "page")
	assert.NotContains(t, tool.InputSchema.Properties, "perPage")

	// Setup mock results
	mockAddKeyResult := &github.Key{
		ID:       github.Ptr(int64(1)),
		Key:      github.Ptr("ssh test key"),
		URL:      github.Ptr("test url"),
		Title:    github.Ptr("test key 1"),
		ReadOnly: github.Ptr(true),
		Verified: github.Ptr(true),
	}
	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedResult *github.Key
		expectedErrMsg string
	}{
		{
			name: "add public ssh key",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostUserKeys,
					expectRequestBody(t, map[string]any{
						"title": "test key 1",
						"key":   "ssh test key",
					}).
						andThen(
							mockResponse(t, http.StatusCreated, mockAddKeyResult),
						),
				),
			),
			requestArgs: map[string]any{
				"title": "test key 1",
				"key":   "ssh test key",
			},
			expectError:    false,
			expectedResult: mockAddKeyResult,
		},
		{
			name: "add public ssh key fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostUserKeys,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte(`{"message": "something bad happened"}`))
					}),
				),
			),
			requestArgs: map[string]any{
				"title": "test key 1",
				"key":   "ssh test key",
			},
			expectError:    true,
			expectedErrMsg: "failed to add ssh key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := AddUsersPublicSSHKey(stubGetClientFn(client), translations.NullTranslationHelper)

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
			var returnedResult *github.Key
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult.ID, returnedResult.ID)
			assert.Equal(t, tc.expectedResult.Key, returnedResult.Key)
			assert.Equal(t, tc.expectedResult.Title, returnedResult.Title)
			assert.Equal(t, tc.expectedResult.URL, returnedResult.URL)
			assert.Equal(t, tc.expectedResult.Verified, returnedResult.Verified)
			assert.Equal(t, tc.expectedResult.ReadOnly, returnedResult.ReadOnly)
		})
	}
}
