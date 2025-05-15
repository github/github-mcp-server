package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v69/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListNotifications(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := ListNotifications(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_notifications", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.Contains(t, tool.InputSchema.Properties, "per_page")
	assert.Contains(t, tool.InputSchema.Properties, "all")

	// Setup mock notifications
	mockNotifications := []*github.Notification{
		{
			ID:     github.Ptr("1"),
			Reason: github.Ptr("mention"),
			Subject: &github.NotificationSubject{
				Title: github.Ptr("Test Notification 1"),
			},
			UpdatedAt: &github.Timestamp{Time: time.Now()},
			URL:       github.Ptr("https://example.com/notifications/threads/1"),
		},
		{
			ID:     github.Ptr("2"),
			Reason: github.Ptr("team_mention"),
			Subject: &github.NotificationSubject{
				Title: github.Ptr("Test Notification 2"),
			},
			UpdatedAt: &github.Timestamp{Time: time.Now()},
			URL:       github.Ptr("https://example.com/notifications/threads/1"),
		},
	}

	tests := []struct {
		name             string
		mockedClient     *http.Client
		requestArgs      map[string]interface{}
		expectError      bool
		expectedResponse []*github.Notification
		expectedErrMsg   string
	}{
		{
			name: "list all notifications",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetNotifications,
					mockNotifications,
				),
			),
			requestArgs: map[string]interface{}{
				"all": true,
			},
			expectError:      false,
			expectedResponse: mockNotifications,
		},
		{
			name: "list unread notifications",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetNotifications,
					mockNotifications[:1], // Only the first notification
				),
			),
			requestArgs: map[string]interface{}{
				"all": false,
			},
			expectError:      false,
			expectedResponse: mockNotifications[:1],
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListNotifications(stubGetClientFn(client), translations.NullTranslationHelper)

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
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedNotifications []*github.Notification
			err = json.Unmarshal([]byte(textContent.Text), &returnedNotifications)
			require.NoError(t, err)
			assert.Equal(t, len(tc.expectedResponse), len(returnedNotifications))
			for i, notification := range returnedNotifications {
				assert.Equal(t, *tc.expectedResponse[i].ID, *notification.ID)
				assert.Equal(t, *tc.expectedResponse[i].Reason, *notification.Reason)
				assert.Equal(t, *tc.expectedResponse[i].Subject.Title, *notification.Subject.Title)
			}
		})
	}
}
