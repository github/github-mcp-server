package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListNotifications(t *testing.T) {
	// Verify tool definition and schema
	serverTool := ListNotifications(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_notifications", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "filter")
	assert.Contains(t, schema.Properties, "since")
	assert.Contains(t, schema.Properties, "before")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "page")
	assert.Contains(t, schema.Properties, "perPage")
	// All fields are optional, so Required should be empty
	assert.Empty(t, schema.Required)
	mockNotification := &github.Notification{
		ID:     github.Ptr("123"),
		Reason: github.Ptr("mention"),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult []*github.Notification
		expectedErrMsg string
	}{
		{
			name: "success default filter (no params)",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetNotifications: mockResponse(t, http.StatusOK, []*github.Notification{mockNotification}),
			}),
			requestArgs:    map[string]interface{}{},
			expectError:    false,
			expectedResult: []*github.Notification{mockNotification},
		},
		{
			name: "success with filter=include_read_notifications",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetNotifications: mockResponse(t, http.StatusOK, []*github.Notification{mockNotification}),
			}),
			requestArgs: map[string]interface{}{
				"filter": "include_read_notifications",
			},
			expectError:    false,
			expectedResult: []*github.Notification{mockNotification},
		},
		{
			name: "success with filter=only_participating",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetNotifications: mockResponse(t, http.StatusOK, []*github.Notification{mockNotification}),
			}),
			requestArgs: map[string]interface{}{
				"filter": "only_participating",
			},
			expectError:    false,
			expectedResult: []*github.Notification{mockNotification},
		},
		{
			name: "success for repo notifications",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposNotificationsByOwnerByRepo: mockResponse(t, http.StatusOK, []*github.Notification{mockNotification}),
			}),
			requestArgs: map[string]interface{}{
				"filter":  "default",
				"since":   "2024-01-01T00:00:00Z",
				"before":  "2024-01-02T00:00:00Z",
				"owner":   "octocat",
				"repo":    "hello-world",
				"page":    float64(2),
				"perPage": float64(10),
			},
			expectError:    false,
			expectedResult: []*github.Notification{mockNotification},
		},
		{
			name: "error",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetNotifications: mockResponse(t, http.StatusInternalServerError, `{"message": "error"}`),
			}),
			requestArgs:    map[string]interface{}{},
			expectError:    true,
			expectedErrMsg: "error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				if tc.expectedErrMsg != "" {
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			t.Logf("textContent: %s", textContent.Text)
			var returned []*github.Notification
			err = json.Unmarshal([]byte(textContent.Text), &returned)
			require.NoError(t, err)
			require.NotEmpty(t, returned)
			assert.Equal(t, *tc.expectedResult[0].ID, *returned[0].ID)
		})
	}
}

func Test_ManageNotificationSubscription(t *testing.T) {
	// Verify tool definition and schema
	serverTool := ManageNotificationSubscription(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "manage_notification_subscription", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "notificationID")
	assert.Contains(t, schema.Properties, "action")
	assert.Equal(t, []string{"notificationID", "action"}, schema.Required)

	mockSub := &github.Subscription{Ignored: github.Ptr(true)}
	mockSubWatch := &github.Subscription{Ignored: github.Ptr(false), Subscribed: github.Ptr(true)}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectIgnored  *bool
		expectDeleted  bool
		expectInvalid  bool
		expectedErrMsg string
	}{
		{
			name: "ignore subscription",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PutNotificationsThreadsSubscriptionByThreadID: mockResponse(t, http.StatusOK, mockSub),
			}),
			requestArgs: map[string]interface{}{
				"notificationID": "123",
				"action":         "ignore",
			},
			expectError:   false,
			expectIgnored: github.Ptr(true),
		},
		{
			name: "watch subscription",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PutNotificationsThreadsSubscriptionByThreadID: mockResponse(t, http.StatusOK, mockSubWatch),
			}),
			requestArgs: map[string]interface{}{
				"notificationID": "123",
				"action":         "watch",
			},
			expectError:   false,
			expectIgnored: github.Ptr(false),
		},
		{
			name: "delete subscription",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteNotificationsThreadsSubscriptionByThreadID: mockResponse(t, http.StatusOK, nil),
			}),
			requestArgs: map[string]interface{}{
				"notificationID": "123",
				"action":         "delete",
			},
			expectError:   false,
			expectDeleted: true,
		},
		{
			name:         "invalid action",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"notificationID": "123",
				"action":         "invalid",
			},
			expectError:   false,
			expectInvalid: true,
		},
		{
			name:         "missing required notificationID",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"action": "ignore",
			},
			expectError: true,
		},
		{
			name:         "missing required action",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"notificationID": "123",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				require.NoError(t, err)
				require.NotNil(t, result)
				text := getTextResult(t, result).Text
				switch {
				case tc.requestArgs["notificationID"] == nil:
					assert.Contains(t, text, "missing required parameter: notificationID")
				case tc.requestArgs["action"] == nil:
					assert.Contains(t, text, "missing required parameter: action")
				default:
					assert.Contains(t, text, "error")
				}
				return
			}

			require.NoError(t, err)
			textContent := getTextResult(t, result)
			if tc.expectIgnored != nil {
				var returned github.Subscription
				err = json.Unmarshal([]byte(textContent.Text), &returned)
				require.NoError(t, err)
				assert.Equal(t, *tc.expectIgnored, *returned.Ignored)
			}
			if tc.expectDeleted {
				assert.Contains(t, textContent.Text, "deleted")
			}
			if tc.expectInvalid {
				assert.Contains(t, textContent.Text, "Invalid action")
			}
		})
	}
}

func Test_ManageRepositoryNotificationSubscription(t *testing.T) {
	// Verify tool definition and schema
	serverTool := ManageRepositoryNotificationSubscription(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "manage_repository_notification_subscription", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "action")
	assert.Equal(t, []string{"owner", "repo", "action"}, schema.Required)

	mockSub := &github.Subscription{Ignored: github.Ptr(true)}
	mockWatchSub := &github.Subscription{Ignored: github.Ptr(false), Subscribed: github.Ptr(true)}

	tests := []struct {
		name             string
		mockedClient     *http.Client
		requestArgs      map[string]interface{}
		expectError      bool
		expectIgnored    *bool
		expectSubscribed *bool
		expectDeleted    bool
		expectInvalid    bool
		expectedErrMsg   string
	}{
		{
			name: "ignore subscription",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PutReposSubscriptionByOwnerByRepo: mockResponse(t, http.StatusOK, mockSub),
			}),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"action": "ignore",
			},
			expectError:   false,
			expectIgnored: github.Ptr(true),
		},
		{
			name: "watch subscription",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PutReposSubscriptionByOwnerByRepo: mockResponse(t, http.StatusOK, mockWatchSub),
			}),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"action": "watch",
			},
			expectError:      false,
			expectIgnored:    github.Ptr(false),
			expectSubscribed: github.Ptr(true),
		},
		{
			name: "delete subscription",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteReposSubscriptionByOwnerByRepo: mockResponse(t, http.StatusOK, nil),
			}),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"action": "delete",
			},
			expectError:   false,
			expectDeleted: true,
		},
		{
			name:         "invalid action",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"action": "invalid",
			},
			expectError:   false,
			expectInvalid: true,
		},
		{
			name:         "missing required owner",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"repo":   "repo",
				"action": "ignore",
			},
			expectError: true,
		},
		{
			name:         "missing required repo",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"action": "ignore",
			},
			expectError: true,
		},
		{
			name:         "missing required action",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				require.NotNil(t, result)
				text := getTextResult(t, result).Text
				switch {
				case tc.requestArgs["owner"] == nil:
					assert.Contains(t, text, "missing required parameter: owner")
				case tc.requestArgs["repo"] == nil:
					assert.Contains(t, text, "missing required parameter: repo")
				case tc.requestArgs["action"] == nil:
					assert.Contains(t, text, "missing required parameter: action")
				default:
					assert.Contains(t, text, "error")
				}
				return
			}

			require.NoError(t, err)
			textContent := getTextResult(t, result)
			if tc.expectIgnored != nil || tc.expectSubscribed != nil {
				var returned github.Subscription
				err = json.Unmarshal([]byte(textContent.Text), &returned)
				require.NoError(t, err)
				if tc.expectIgnored != nil {
					assert.Equal(t, *tc.expectIgnored, *returned.Ignored)
				}
				if tc.expectSubscribed != nil {
					assert.Equal(t, *tc.expectSubscribed, *returned.Subscribed)
				}
			}
			if tc.expectDeleted {
				assert.Contains(t, textContent.Text, "deleted")
			}
			if tc.expectInvalid {
				assert.Contains(t, textContent.Text, "Invalid action")
			}
		})
	}
}

func Test_DismissNotification(t *testing.T) {
	// Verify tool definition and schema
	serverTool := DismissNotification(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "dismiss_notification", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "threadID")
	assert.Contains(t, schema.Properties, "state")
	assert.Equal(t, []string{"threadID", "state"}, schema.Required)

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectRead     bool
		expectDone     bool
		expectInvalid  bool
		expectedErrMsg string
	}{
		{
			name: "mark as read",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchNotificationsThreadsByThreadID: mockResponse(t, http.StatusOK, nil),
			}),
			requestArgs: map[string]interface{}{
				"threadID": "123",
				"state":    "read",
			},
			expectError: false,
			expectRead:  true,
		},
		{
			name: "mark as done",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteNotificationsThreadsByThreadID: mockResponse(t, http.StatusOK, nil),
			}),
			requestArgs: map[string]interface{}{
				"threadID": "123",
				"state":    "done",
			},
			expectError: false,
			expectDone:  true,
		},
		{
			name:         "invalid threadID format",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"threadID": "notanumber",
				"state":    "done",
			},
			expectError:   false,
			expectInvalid: true,
		},
		{
			name:         "missing required threadID",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"state": "read",
			},
			expectError: true,
		},
		{
			name:         "missing required state",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"threadID": "123",
			},
			expectError: true,
		},
		{
			name:         "invalid state value",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"threadID": "123",
				"state":    "invalid",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				// The tool returns a ToolResultError with a specific message
				require.NotNil(t, result)
				text := getTextResult(t, result).Text
				switch {
				case tc.requestArgs["threadID"] == nil:
					assert.Contains(t, text, "missing required parameter: threadID")
				case tc.requestArgs["state"] == nil:
					assert.Contains(t, text, "missing required parameter: state")
				case tc.name == "invalid threadID format":
					assert.Contains(t, text, "invalid threadID format")
				case tc.name == "invalid state value":
					assert.Contains(t, text, "Invalid state. Must be one of: read, done.")
				default:
					// fallback for other errors
					assert.Contains(t, text, "error")
				}
				return
			}

			require.NoError(t, err)
			textContent := getTextResult(t, result)
			if tc.expectRead {
				assert.Contains(t, textContent.Text, "Notification marked as read")
			}
			if tc.expectDone {
				assert.Contains(t, textContent.Text, "Notification marked as done")
			}
			if tc.expectInvalid {
				assert.Contains(t, textContent.Text, "invalid threadID format")
			}
		})
	}
}

func Test_MarkAllNotificationsRead(t *testing.T) {
	// Verify tool definition and schema
	serverTool := MarkAllNotificationsRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "mark_all_notifications_read", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "lastReadAt")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Empty(t, schema.Required)

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectMarked   bool
		expectedErrMsg string
	}{
		{
			name: "success (no params)",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PutNotifications: mockResponse(t, http.StatusOK, nil),
			}),
			requestArgs:  map[string]interface{}{},
			expectError:  false,
			expectMarked: true,
		},
		{
			name: "success with lastReadAt param",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PutNotifications: mockResponse(t, http.StatusOK, nil),
			}),
			requestArgs: map[string]interface{}{
				"lastReadAt": "2024-01-01T00:00:00Z",
			},
			expectError:  false,
			expectMarked: true,
		},
		{
			name: "success with owner and repo",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PutReposNotificationsByOwnerByRepo: mockResponse(t, http.StatusOK, nil),
			}),
			requestArgs: map[string]interface{}{
				"owner": "octocat",
				"repo":  "hello-world",
			},
			expectError:  false,
			expectMarked: true,
		},
		{
			name: "API error",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PutNotifications: mockResponse(t, http.StatusInternalServerError, `{"message": "error"}`),
			}),
			requestArgs:    map[string]interface{}{},
			expectError:    true,
			expectedErrMsg: "error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				if tc.expectedErrMsg != "" {
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			if tc.expectMarked {
				assert.Contains(t, textContent.Text, "All notifications marked as read")
			}
		})
	}
}

func Test_GetNotificationDetails(t *testing.T) {
	// Verify tool definition and schema
	serverTool := GetNotificationDetails(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_notification_details", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "notificationID")
	assert.Equal(t, []string{"notificationID"}, schema.Required)

	mockThread := &github.Notification{ID: github.Ptr("123"), Reason: github.Ptr("mention")}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectResult   *github.Notification
		expectedErrMsg string
	}{
		{
			name: "success",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetNotificationsThreadsByThreadID: mockResponse(t, http.StatusOK, mockThread),
			}),
			requestArgs: map[string]interface{}{
				"notificationID": "123",
			},
			expectError:  false,
			expectResult: mockThread,
		},
		{
			name: "not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetNotificationsThreadsByThreadID: mockResponse(t, http.StatusNotFound, `{"message": "not found"}`),
			}),
			requestArgs: map[string]interface{}{
				"notificationID": "123",
			},
			expectError:    true,
			expectedErrMsg: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				if tc.expectedErrMsg != "" {
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			var returned github.Notification
			err = json.Unmarshal([]byte(textContent.Text), &returned)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectResult.ID, *returned.ID)
		})
	}
}
