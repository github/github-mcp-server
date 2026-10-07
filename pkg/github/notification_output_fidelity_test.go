package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationOutputFidelity(t *testing.T) {
	for _, subjectType := range []string{"Issue", "PullRequest", "Discussion", "FutureSubject"} {
		t.Run(subjectType, func(t *testing.T) {
			notification := &github.Notification{
				ID: new("123"), Reason: new("agent_session_finished"),
				Subject: &github.NotificationSubject{
					Type:             new(subjectType),
					URL:              new("https://github.example/api/v3/repos/owner/repo/issues/42"),
					LatestCommentURL: new("legacy-only-comment-url"),
				},
				URL: new("legacy-only-thread-url"),
			}
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetNotifications:                   mockResponse(t, http.StatusOK, []*github.Notification{notification}),
				GetReposNotificationsByOwnerByRepo: mockResponse(t, http.StatusOK, []*github.Notification{notification}),
				GetNotificationsThreadsByThreadID:  mockResponse(t, http.StatusOK, notification),
			}))}
			for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
				server := mcp.NewServer(&mcp.Implementation{Name: "notification-fidelity", Version: "v1"}, nil)
				server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
				for _, tool := range []inventory.ServerTool{
					ListNotifications(translations.NullTranslationHelper),
					GetNotificationDetails(translations.NullTranslationHelper),
				} {
					tool.RegisterFunc(server, deps)
				}
				client := connectCommentVisibilityClient(t, server, protocol)
				for _, call := range []mcp.CallToolParams{
					{Name: "list_notifications", Arguments: map[string]any{}},
					{Name: "list_notifications", Arguments: map[string]any{"owner": "owner", "repo": "repo"}},
					{Name: "get_notification_details", Arguments: map[string]any{"notificationID": "123"}},
				} {
					result, err := client.CallTool(context.Background(), &call)
					require.NoError(t, err)
					require.False(t, result.IsError, "%s", getTextResult(t, result).Text)
					if protocol == "2025-11-25" {
						var expected any = notification
						if call.Name == "list_notifications" {
							expected = []*github.Notification{notification}
						}
						assert.Equal(t, mustMarshalJSON(t, expected), getTextResult(t, result).Text)
						assert.Nil(t, result.StructuredContent)
						continue
					}
					require.NotNil(t, result.StructuredContent)
					text := mustMarshalJSON(t, result.StructuredContent)
					assert.JSONEq(t, text, getTextResult(t, result).Text)
					assert.Contains(t, text, `"reason":"agent_session_finished"`)
					assert.Contains(t, text, `"type":"`+subjectType+`"`)
					assert.Contains(t, text, `"url":"`+notification.Subject.GetURL()+`"`)
					assert.NotContains(t, text, "legacy-only")
					resolved, err := notificationOutputSchema(call.Name == "list_notifications").Resolve(nil)
					require.NoError(t, err)
					var output any
					require.NoError(t, json.Unmarshal([]byte(text), &output))
					require.NoError(t, resolved.Validate(output))
				}
			}
		})
	}
}
