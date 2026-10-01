package github

import (
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationOutputsMatchDeclaredSchemas(t *testing.T) {
	notification := notificationOutputFixture()
	expected := notificationOutputExpected()

	tests := []struct {
		name     string
		tool     inventory.ServerTool
		endpoint string
		args     map[string]any
		response any
		expected any
	}{
		{
			name:     "list_notifications",
			tool:     ListNotifications(translations.NullTranslationHelper),
			endpoint: GetNotifications,
			args:     map[string]any{},
			response: []any{notification},
			expected: []any{expected},
		},
		{
			name:     "get_notification_details",
			tool:     GetNotificationDetails(translations.NullTranslationHelper),
			endpoint: GetNotificationsThreadsByThreadID,
			args:     map[string]any{"notificationID": "0"},
			response: notification,
			expected: expected,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := callRegisteredTool(t, tc.tool, notificationOutputDeps(t, tc.endpoint,
				mockResponse(t, http.StatusOK, tc.response)), tc.args)

			requireStructuredJSONAgreement(t, tc.tool, result)
			assert.Equal(t, tc.expected, result.StructuredContent)
		})
	}
}

func TestListNotificationsStructuredEmptyResult(t *testing.T) {
	tool := ListNotifications(translations.NullTranslationHelper)
	result := callRegisteredTool(t, tool, notificationOutputDeps(t, GetNotifications,
		mockResponse(t, http.StatusOK, []any{})), map[string]any{})

	requireStructuredJSONAgreement(t, tool, result)
	assert.Equal(t, []any{}, result.StructuredContent)
}

func TestNotificationOutputOmitsAbsentOptionalFields(t *testing.T) {
	tool := GetNotificationDetails(translations.NullTranslationHelper)
	response := map[string]any{
		"id":     "notification",
		"unread": false,
		"subject": map[string]any{
			"title": "title",
			"type":  "Issue",
		},
		"repository": map[string]any{
			"name":    "repo",
			"private": false,
		},
	}
	result := callRegisteredTool(t, tool, notificationOutputDeps(t, GetNotificationsThreadsByThreadID,
		mockResponse(t, http.StatusOK, response)), map[string]any{"notificationID": "notification"})

	requireStructuredJSONAgreement(t, tool, result)
	assert.Equal(t, response, result.StructuredContent)
}

func TestListNotificationsStructuredFiltersAndPagination(t *testing.T) {
	tool := ListNotifications(translations.NullTranslationHelper)
	handler := expectQueryParams(t, map[string]string{
		"all":      "true",
		"since":    "2026-08-01T00:00:00Z",
		"before":   "2026-08-24T00:00:00Z",
		"page":     "2",
		"per_page": "10",
	}).andThen(mockResponse(t, http.StatusOK, []any{}))
	result := callRegisteredTool(t, tool, notificationOutputDeps(t,
		GetReposNotificationsByOwnerByRepo, handler), map[string]any{
		"filter":  FilterIncludeRead,
		"since":   "2026-08-01T00:00:00Z",
		"before":  "2026-08-24T00:00:00Z",
		"owner":   "owner",
		"repo":    "repo",
		"page":    2,
		"perPage": 10,
	})

	requireStructuredJSONAgreement(t, tool, result)
	assert.Equal(t, []any{}, result.StructuredContent)
}

func TestNotificationOutputErrorsHaveNoStructuredContent(t *testing.T) {
	tests := []struct {
		name     string
		tool     inventory.ServerTool
		endpoint string
		args     map[string]any
	}{
		{
			name:     "list_notifications",
			tool:     ListNotifications(translations.NullTranslationHelper),
			endpoint: GetNotifications,
			args:     map[string]any{},
		},
		{
			name:     "get_notification_details",
			tool:     GetNotificationDetails(translations.NullTranslationHelper),
			endpoint: GetNotificationsThreadsByThreadID,
			args:     map[string]any{"notificationID": "0"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := callRegisteredTool(t, tc.tool, notificationOutputDeps(t, tc.endpoint,
				mockResponse(t, http.StatusInternalServerError, map[string]any{"message": "boom"})), tc.args)

			require.True(t, result.IsError)
			assert.Nil(t, result.StructuredContent)
		})
	}
}

func notificationOutputDeps(t *testing.T, endpoint string, handler http.HandlerFunc) ToolDependencies {
	t.Helper()
	return BaseDeps{
		Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			endpoint: handler,
		})),
		Obsv: stubExporters(),
	}
}

func notificationOutputFixture() map[string]any {
	return map[string]any{
		"id":     "",
		"reason": "",
		"unread": false,
		"subject": map[string]any{
			"title":              "",
			"url":                "",
			"latest_comment_url": "",
			"type":               "Issue",
		},
		"repository": map[string]any{
			"id":        0,
			"node_id":   "",
			"name":      "",
			"full_name": "",
			"private":   false,
			"owner": map[string]any{
				"login":      "",
				"id":         0,
				"node_id":    "",
				"avatar_url": "",
				"html_url":   "",
				"email":      "not projected",
			},
			"html_url":    "",
			"url":         "",
			"description": "not projected",
		},
		"updated_at":   "2026-08-24T12:34:56Z",
		"last_read_at": "2026-08-24T12:35:00Z",
		"url":          "",
	}
}

func notificationOutputExpected() map[string]any {
	return map[string]any{
		"id":     "",
		"reason": "",
		"unread": false,
		"subject": map[string]any{
			"title":              "",
			"url":                "",
			"latest_comment_url": "",
			"type":               "Issue",
		},
		"repository": map[string]any{
			"id":        float64(0),
			"node_id":   "",
			"name":      "",
			"full_name": "",
			"private":   false,
			"owner": map[string]any{
				"login":      "",
				"id":         float64(0),
				"node_id":    "",
				"avatar_url": "",
				"html_url":   "",
			},
			"html_url": "",
			"url":      "",
		},
		"updated_at":   "2026-08-24T12:34:56Z",
		"last_read_at": "2026-08-24T12:35:00Z",
		"url":          "",
	}
}
