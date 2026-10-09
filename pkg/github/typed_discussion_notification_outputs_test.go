package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discussionNotificationTools() []inventory.ServerTool {
	t := translations.NullTranslationHelper
	return []inventory.ServerTool{
		ListNotifications(t), DismissNotification(t), MarkAllNotificationsRead(t),
		GetNotificationDetails(t), ManageNotificationSubscription(t),
		ManageRepositoryNotificationSubscription(t), ListDiscussions(t),
		GetDiscussion(t), GetDiscussionComments(t), DiscussionCommentWrite(t),
		ListDiscussionCategories(t),
	}
}

func typedDiscussionNotificationDeps(t *testing.T) BaseDeps {
	t.Helper()
	notification := &github.Notification{
		ID: new("123"), Reason: new("mention"), Unread: new(true),
		Subject: &github.NotificationSubject{Title: new("A discussion"), Type: new("Discussion")},
		Repository: &github.Repository{
			ID: new(int64(42)), FullName: new("owner/repo"),
			Description: new("legacy-only repository detail"),
		},
	}
	subscription := &github.Subscription{Subscribed: new(true), Ignored: new(false)}
	gql := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		var data any
		comment := map[string]any{"id": "DC_123", "url": "https://github.com/owner/repo/discussions/1#discussioncomment-123"}
		discussion := map[string]any{"id": "D_1", "url": "https://github.com/owner/repo/discussions/1"}
		pageInfo := map[string]any{"hasNextPage": true, "hasPreviousPage": false, "startCursor": "start", "endCursor": "end"}
		switch {
		case strings.Contains(req.Query, "addDiscussionComment("):
			data = map[string]any{"addDiscussionComment": map[string]any{"comment": comment}}
		case strings.Contains(req.Query, "updateDiscussionComment("):
			data = map[string]any{"updateDiscussionComment": map[string]any{"comment": comment}}
		case strings.Contains(req.Query, "deleteDiscussionComment("):
			data = map[string]any{"deleteDiscussionComment": map[string]any{"comment": comment}}
		case strings.Contains(req.Query, "unmarkDiscussionCommentAsAnswer("):
			data = map[string]any{"unmarkDiscussionCommentAsAnswer": map[string]any{"discussion": discussion}}
		case strings.Contains(req.Query, "markDiscussionCommentAsAnswer("):
			data = map[string]any{"markDiscussionCommentAsAnswer": map[string]any{"discussion": discussion}}
		case strings.Contains(req.Query, "node(id:"):
			id := "D_1"
			if req.Variables["replyToID"] == "wrong-discussion" {
				id = "D_2"
			}
			data = map[string]any{"node": map[string]any{
				"id": "DC_123", "discussion": map[string]any{"id": id},
			}}
		case strings.Contains(req.Query, "discussionCategories("):
			assert.Equal(t, float64(25), req.Variables["first"])
			assert.Equal(t, ".github", req.Variables["repo"])
			data = map[string]any{"repository": map[string]any{"discussionCategories": map[string]any{
				"nodes": []any{map[string]any{"id": "CAT_1", "name": "General"}}, "pageInfo": pageInfo, "totalCount": 1,
			}}}
		case strings.Contains(req.Query, "discussions("):
			assert.Equal(t, float64(30), req.Variables["first"])
			assert.Equal(t, ".github", req.Variables["repo"])
			data = map[string]any{"repository": map[string]any{"discussions": map[string]any{
				"nodes": []any{map[string]any{
					"number": 1, "title": "A title", "url": "https://github.com/owner/repo/discussions/1",
					"createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-02T00:00:00Z",
					"author": map[string]any{"login": "octocat"}, "category": map[string]any{"name": "General"},
				}}, "pageInfo": pageInfo, "totalCount": 1,
			}}}
		case strings.Contains(req.Query, "comments("):
			assert.Equal(t, float64(30), req.Variables["first"])
			assert.Equal(t, float64(1), req.Variables["discussionNumber"])
			node := map[string]any{"id": "DC_123", "body": "A comment", "isAnswer": true}
			if strings.Contains(req.Query, "replies(") {
				node["replies"] = map[string]any{
					"nodes": []any{map[string]any{"id": "DC_reply", "body": "A reply", "isAnswer": false}}, "totalCount": 1,
				}
			}
			data = map[string]any{"repository": map[string]any{"discussion": map[string]any{"comments": map[string]any{
				"nodes": []any{node}, "pageInfo": pageInfo, "totalCount": 1,
			}}}}
		case strings.Contains(req.Query, "discussion(number:") && strings.Contains(req.Query, "{id}"):
			data = map[string]any{"repository": map[string]any{"discussion": map[string]any{"id": "D_1"}}}
		case strings.Contains(req.Query, "discussion(number:"):
			assert.Equal(t, float64(1), req.Variables["discussionNumber"])
			data = map[string]any{"repository": map[string]any{"discussion": map[string]any{
				"number": 1, "title": "A title", "body": "A body",
				"url": "https://github.com/owner/repo/discussions/1", "createdAt": "2026-01-01T00:00:00Z",
				"closed": false, "isAnswered": true, "answerChosenAt": "2026-01-02T00:00:00Z",
				"category": map[string]any{"name": "General"},
			}}}
		default:
			t.Errorf("unexpected GraphQL query: %s", req.Query)
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": data}))
	})}}
	return BaseDeps{
		featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		GQLClient:      githubv4.NewClient(gql),
		Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposByOwnerByRepo:                            mockResponse(t, http.StatusOK, &github.Repository{Private: new(true)}),
			GetNotifications:                                 mockResponse(t, http.StatusOK, []*github.Notification{notification}),
			GetReposNotificationsByOwnerByRepo:               mockResponse(t, http.StatusOK, []*github.Notification{notification}),
			GetNotificationsThreadsByThreadID:                mockResponse(t, http.StatusOK, notification),
			PatchNotificationsThreadsByThreadID:              mockResponse(t, http.StatusOK, nil),
			DeleteNotificationsThreadsByThreadID:             mockResponse(t, http.StatusNoContent, nil),
			PutNotifications:                                 mockResponse(t, http.StatusOK, nil),
			PutReposNotificationsByOwnerByRepo:               mockResponse(t, http.StatusOK, nil),
			PutNotificationsThreadsSubscriptionByThreadID:    mockResponse(t, http.StatusOK, subscription),
			DeleteNotificationsThreadsSubscriptionByThreadID: mockResponse(t, http.StatusNoContent, nil),
			PutReposSubscriptionByOwnerByRepo:                mockResponse(t, http.StatusOK, subscription),
			DeleteReposSubscriptionByOwnerByRepo:             mockResponse(t, http.StatusNoContent, nil),
		})),
	}
}

func TestTypedDiscussionNotificationOutputs(t *testing.T) {
	const notificationText = `{"id":"123","repository":{"id":42,"full_name":"owner/repo","description":"legacy-only repository detail"},"subject":{"title":"A discussion","type":"Discussion"},"reason":"mention","unread":true}`
	const commentText = `{"id":"DC_123","url":"https://github.com/owner/repo/discussions/1#discussioncomment-123"}`
	const answerText = `{"discussionID":"D_1","discussionURL":"https://github.com/owner/repo/discussions/1"}`
	const pageText = `{"endCursor":"end","hasNextPage":true,"hasPreviousPage":false,"startCursor":"start"}`
	tests := []struct {
		name string
		args map[string]any
		text string
	}{
		{"list_notifications", map[string]any{"page": "0", "perPage": "0"}, "[" + notificationText + "]"},
		{"list_notifications", map[string]any{"owner": "owner", "repo": "repo", "since": "2026-01-01T00:00:00Z"}, "[" + notificationText + "]"},
		{"get_notification_details", map[string]any{"notificationID": "123"}, notificationText},
		{"dismiss_notification", map[string]any{"threadID": "123", "state": "read"}, "Notification marked as read"},
		{"dismiss_notification", map[string]any{"threadID": "123", "state": "done"}, "Notification marked as done"},
		{"mark_all_notifications_read", map[string]any{"lastReadAt": "2026-01-01T00:00:00Z"}, "All notifications marked as read"},
		{"mark_all_notifications_read", map[string]any{"owner": "owner", "repo": "repo", "lastReadAt": "2026-01-01T00:00:00Z"}, "All notifications marked as read"},
		{"list_discussions", map[string]any{"owner": "owner", "perPage": "30", "after": "cursor", "orderBy": "UPDATED_AT", "direction": "DESC", "category": "CAT_1"},
			`{"discussions":[{"category":{"name":"General"},"html_url":"https://github.com/owner/repo/discussions/1","number":1,"title":"A title","user":{"login":"octocat"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z"}],"pageInfo":` + pageText + `,"totalCount":1}`},
		{"get_discussion", map[string]any{"OWNER": "owner", "REPO": "repo", "DISCUSSIONNUMBER": "1"},
			`{"answerChosenAt":"2026-01-02T00:00:00Z","body":"A body","category":{"name":"General"},"closed":false,"createdAt":"2026-01-01T00:00:00Z","isAnswered":true,"number":1,"title":"A title","url":"https://github.com/owner/repo/discussions/1"}`},
		{"get_discussion_comments", map[string]any{"owner": "owner", "repo": "repo", "discussionNumber": "1"},
			`{"comments":[{"id":"DC_123","body":"A comment","isAnswer":true}],"pageInfo":` + pageText + `,"totalCount":1}`},
		{"get_discussion_comments", map[string]any{"owner": "owner", "repo": "repo", "discussionNumber": 1.9, "includeReplies": true, "perPage": "0"},
			`{"comments":[{"id":"DC_123","body":"A comment","isAnswer":true,"replies":[{"id":"DC_reply","body":"A reply"}],"replyTotalCount":1}],"pageInfo":` + pageText + `,"totalCount":1}`},
		{"list_discussion_categories", map[string]any{"owner": "owner"},
			`{"categories":[{"id":"CAT_1","name":"General"}],"pageInfo":` + pageText + `,"totalCount":1}`},
	}
	for _, name := range []string{"manage_notification_subscription", "manage_repository_notification_subscription"} {
		for _, action := range []string{"ignore", "watch", "delete"} {
			args := map[string]any{"action": action, "notificationID": "123", "owner": "owner", "repo": "repo"}
			text := `{"subscribed":true,"ignored":false}`
			if action == "delete" {
				text = "Notification subscription deleted"
				if name == "manage_repository_notification_subscription" {
					text = "Repository subscription deleted"
				}
			}
			tests = append(tests, struct {
				name string
				args map[string]any
				text string
			}{name, args, text})
		}
	}
	for _, method := range []string{"add", "reply", "update", "delete", "mark_answer", "unmark_answer"} {
		args := map[string]any{"method": method, "owner": "owner", "repo": "repo", "discussionNumber": "1", "body": "New body", "commentNodeID": "DC_123"}
		text := commentText
		if method == "mark_answer" || method == "unmark_answer" {
			text = answerText
		}
		tests = append(tests, struct {
			name string
			args map[string]any
			text string
		}{"discussion_comment_write", args, text})
	}

	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		t.Run(protocol, func(t *testing.T) {
			deps := typedDiscussionNotificationDeps(t)
			server := mcp.NewServer(&mcp.Implementation{Name: "typed-discussion-notification-test", Version: "v1"}, nil)
			server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
			for _, tool := range discussionNotificationTools() {
				tool.RegisterFunc(server, deps)
			}
			client := connectCommentVisibilityClient(t, server, protocol)
			list, err := client.ListTools(context.Background(), nil)
			require.NoError(t, err)
			require.Len(t, list.Tools, 11)
			byName := map[string]*mcp.Tool{}
			for _, tool := range list.Tools {
				byName[tool.Name] = tool
				if protocol == "2025-11-25" {
					assert.Nil(t, tool.OutputSchema, tool.Name)
				} else {
					require.NotNil(t, tool.OutputSchema, tool.Name)
					canonical := tool.OutputSchema
					for _, definition := range discussionNotificationTools() {
						if definition.Tool.Name == tool.Name {
							assert.JSONEq(t, mustMarshalJSON(t, definition.Tool.OutputSchema), mustMarshalJSON(t, canonical))
							require.NoError(t, toolsnaps.Test(definition.Tool.Name, definition.Tool))
						}
					}
				}
			}
			for i, tc := range tests {
				t.Run(tc.name+"/"+string(rune('A'+i)), func(t *testing.T) {
					result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
					require.NoError(t, err)
					require.False(t, result.IsError, "%s", result)
					require.Len(t, result.Content, 1, "text must not duplicate structured content")
					switch tc.name {
					case "get_notification_details":
						assert.JSONEq(t, mustMarshalJSON(t, ifc.LabelNotificationDetails()), mustMarshalJSON(t, result.Meta["ifc"]))
					case "list_discussions", "get_discussion", "get_discussion_comments":
						assert.JSONEq(t, mustMarshalJSON(t, ifc.LabelRepoUserContent(true)), mustMarshalJSON(t, result.Meta["ifc"]))
					case "list_discussion_categories":
						assert.JSONEq(t, mustMarshalJSON(t, ifc.LabelRepoMetadata(true)), mustMarshalJSON(t, result.Meta["ifc"]))
					}
					if protocol == "2025-11-25" {
						assert.Equal(t, tc.text, getTextResult(t, result).Text, "legacy text must remain byte-equivalent")
						assert.Nil(t, result.StructuredContent)
						return
					}
					require.NotNil(t, result.StructuredContent)
					if json.Valid([]byte(tc.text)) {
						assert.JSONEq(t, mustMarshalJSON(t, result.StructuredContent), getTextResult(t, result).Text,
							"modern JSON text must serialize the same compact DTO as structuredContent")
					} else {
						assert.Equal(t, tc.text, getTextResult(t, result).Text, "plain status and deletion messages remain unchanged")
					}
					var schema jsonschema.Schema
					require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, byName[tc.name].OutputSchema)), &schema))
					resolved, err := schema.Resolve(nil)
					require.NoError(t, err)
					var output any
					require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
					require.NoError(t, resolved.Validate(output), tc.name)
					assert.NotContains(t, mustMarshalJSON(t, output), "legacy-only repository detail")
					if tc.name == "get_discussion" {
						assert.Contains(t, mustMarshalJSON(t, output), `"html_url"`)
						assert.NotContains(t, mustMarshalJSON(t, output), `"url"`)
					}
				})
			}
			for _, tc := range []struct {
				name string
				args map[string]any
				text string
			}{
				{"list_notifications", map[string]any{"since": "invalid"}, "invalid since time format, should be RFC3339/ISO8601:"},
				{"list_notifications", map[string]any{"before": "invalid"}, "invalid before time format, should be RFC3339/ISO8601:"},
				{"mark_all_notifications_read", map[string]any{"lastReadAt": "invalid"}, "invalid lastReadAt time format, should be RFC3339/ISO8601:"},
				{"discussion_comment_write", map[string]any{"method": "reply", "owner": "owner", "repo": "repo", "discussionNumber": 1, "body": "reply", "commentNodeID": "wrong-discussion"}, `does not belong to discussion #1 in owner/repo`},
				{"discussion_comment_write", map[string]any{"method": "delete", "commentNodeID": " "}, "commentNodeID cannot be blank"},
				{"discussion_comment_write", map[string]any{"method": "add", "owner": "owner", "repo": "repo", "discussionNumber": 1}, "missing required parameter: body"},
			} {
				result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Nil(t, result.StructuredContent, "errors must not contain a success DTO")
				assert.Contains(t, getTextResult(t, result).Text, tc.text)
			}
		})
	}
}

func TestDiscussionNotificationUnionsRejectImpossibleResults(t *testing.T) {
	for _, tc := range []struct {
		name    string
		schema  *jsonschema.Schema
		valid   []string
		invalid []string
	}{
		{
			"comment write", discussionCommentWriteOutputSchema(),
			[]string{`{"id":"","url":""}`, `{"discussionID":"","discussionURL":""}`},
			[]string{`{}`, `{"id":"123"}`, `{"discussionID":"D_1"}`, `{"id":"123","url":"","discussionID":"D_1","discussionURL":""}`, `{"message":"deleted"}`},
		},
		{
			"subscription", notificationSubscriptionResultSchema(),
			[]string{`{"subscription":{}}`, `{"message":"deleted"}`},
			[]string{`{}`, `{"subscription":{},"message":"deleted"}`, `{"subscription":null}`, `{"subscribed":true}`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := tc.schema.Resolve(nil)
			require.NoError(t, err)
			for _, raw := range tc.valid {
				var value any
				require.NoError(t, json.Unmarshal([]byte(raw), &value))
				assert.NoError(t, resolved.Validate(value), raw)
			}
			for _, raw := range tc.invalid {
				var value any
				require.NoError(t, json.Unmarshal([]byte(raw), &value))
				assert.Error(t, resolved.Validate(value), raw)
			}
		})
	}
}
