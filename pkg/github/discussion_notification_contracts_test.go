package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/sanitize"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscussionReadRequiredArguments(t *testing.T) {
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		t.Run(protocol, func(t *testing.T) {
			calls := 0
			deps := BaseDeps{GQLClient: githubv4.NewClient(&http.Client{
				Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					var request struct {
						Variables map[string]any `json:"variables"`
					}
					require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
					assert.Equal(t, "", request.Variables["owner"])
					assert.Equal(t, "", request.Variables["repo"])
					assert.Equal(t, float64(0), request.Variables["discussionNumber"])
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
						"errors": []any{map[string]any{"message": "empty coordinates rejected by GitHub"}},
					}))
				})},
			})}
			server := mcp.NewServer(&mcp.Implementation{Name: "required-arguments-test", Version: "v1"}, nil)
			server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
			for _, tool := range []inventory.ServerTool{
				GetDiscussion(translations.NullTranslationHelper),
				GetDiscussionComments(translations.NullTranslationHelper),
			} {
				tool.RegisterFunc(server, deps)
			}
			client := connectCommentVisibilityClient(t, server, protocol)
			for _, name := range []string{"get_discussion", "get_discussion_comments"} {
				// A wholly empty object must not be normalized into zero-valued
				// owner/repo/discussionNumber keys that would satisfy the SDK's
				// required-property validation and reach GraphQL.
				before := calls
				result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: map[string]any{}})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Nil(t, result.StructuredContent)
				text := getTextResult(t, result).Text
				assert.Contains(t, text, "owner")
				assert.Contains(t, text, "repo")
				assert.Contains(t, text, "discussionNumber")
				assert.Equal(t, before, calls, "empty object must not reach GitHub")

				for _, missing := range []string{"owner", "repo", "discussionNumber"} {
					args := map[string]any{"owner": "owner", "repo": "repo", "discussionNumber": "1"}
					delete(args, missing)
					before := calls
					result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
					require.NoError(t, err)
					require.True(t, result.IsError)
					assert.Nil(t, result.StructuredContent)
					assert.Contains(t, getTextResult(t, result).Text, missing)
					assert.Equal(t, before, calls, "missing required property must not reach GitHub")
				}
				result, err = client.CallTool(context.Background(), &mcp.CallToolParams{
					Name: name, Arguments: map[string]any{"OWNER": "", "REPO": "", "DISCUSSIONNUMBER": 0},
				})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Contains(t, getTextResult(t, result).Text, "empty coordinates rejected by GitHub")
			}
			assert.Equal(t, 2, calls, "present empty values retain main's WeakDecode/API behavior")
		})
	}
}

func TestDiscussionNotificationEmptyAndNullOutputs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		api    string
		tool   inventory.ServerTool
		args   map[string]any
		legacy string
		modern string
	}{
		{"notifications null", "null", ListNotifications(translations.NullTranslationHelper), map[string]any{}, "null", "[]"},
		{"notifications empty", "[]", ListNotifications(translations.NullTranslationHelper), map[string]any{}, "[]", "[]"},
		{"notification null item", "[null]", ListNotifications(translations.NullTranslationHelper), map[string]any{}, "[null]", "[null]"},
		{"notification null detail", "null", GetNotificationDetails(translations.NullTranslationHelper), map[string]any{"notificationID": "123"}, "null", "{}"},
	} {
		for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
			t.Run(tc.name+"/"+protocol, func(t *testing.T) {
				deps := BaseDeps{Client: mustNewGHClient(t, &http.Client{
					Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						_, err := w.Write([]byte(tc.api))
						require.NoError(t, err)
					})},
				})}
				server := mcp.NewServer(&mcp.Implementation{Name: "empty-output-test", Version: "v1"}, nil)
				server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
				tc.tool.RegisterFunc(server, deps)
				client := connectCommentVisibilityClient(t, server, protocol)
				result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool.Tool.Name, Arguments: tc.args})
				require.NoError(t, err)
				require.False(t, result.IsError)
				if protocol == "2025-11-25" {
					assert.Equal(t, tc.legacy, getTextResult(t, result).Text)
					assert.Nil(t, result.StructuredContent)
				} else {
					assert.JSONEq(t, tc.modern, mustMarshalJSON(t, result.StructuredContent))
					assert.JSONEq(t, tc.modern, getTextResult(t, result).Text)
					resolved, err := tc.tool.Tool.OutputSchema.(*jsonschema.Schema).Resolve(nil)
					require.NoError(t, err)
					var output any
					require.NoError(t, json.Unmarshal([]byte(tc.modern), &output))
					assert.NoError(t, resolved.Validate(output))
				}
			})
		}
	}
}

func TestDiscussionStructuredSanitizationMatchesLegacy(t *testing.T) {
	const title = "<script>alert('title')</script>Discussion"
	const body = "<script>alert('body')</script>Discussion body"
	deps := BaseDeps{GQLClient: githubv4.NewClient(&http.Client{
		Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"repository": map[string]any{"discussion": map[string]any{
					"number": 1, "title": title, "body": body,
					"createdAt": "2026-01-01T00:00:00Z", "category": map[string]any{"name": "General"},
				}},
			}}))
		})},
	})}
	server := mcp.NewServer(&mcp.Implementation{Name: "sanitized-output-test", Version: "v1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	tool := GetDiscussion(translations.NullTranslationHelper)
	tool.RegisterFunc(server, deps)
	client := connectCommentVisibilityClient(t, server, inventory.ProtocolVersionMultiRoundTrip)
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name: tool.Tool.Name, Arguments: map[string]any{"owner": "owner", "repo": "repo", "discussionNumber": 1},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	var legacy, modern map[string]any
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &legacy))
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &modern))
	assert.Equal(t, legacy, modern, "modern JSON text and structured DTO must match")
	assert.Equal(t, sanitize.PlainText(title), modern["title"])
	assert.Equal(t, sanitize.Content(body), modern["body"])
	assert.Equal(t, legacy["title"], modern["title"])
	assert.Equal(t, legacy["body"], modern["body"])
}

func TestNotificationProjectionTrimsHypermedia(t *testing.T) {
	notification := &github.Notification{
		ID: new("123"), URL: new("https://api.github.com/notifications/threads/123"),
		Subject: &github.NotificationSubject{
			Title: new("Title"), Type: new("Issue"),
			URL:              new("https://api.github.com/repos/owner/repo/issues/1"),
			LatestCommentURL: new("https://api.github.com/repos/owner/repo/issues/comments/2"),
		},
	}
	before := `{"id":"123","subject":{"title":"Title","url":"https://api.github.com/repos/owner/repo/issues/1","latest_comment_url":"https://api.github.com/repos/owner/repo/issues/comments/2","type":"Issue"},"url":"https://api.github.com/notifications/threads/123"}`
	after := mustMarshalJSON(t, notificationOutput(notification))
	assert.JSONEq(t, `{"id":"123","subject":{"title":"Title","type":"Issue","url":"https://api.github.com/repos/owner/repo/issues/1"}}`, after)
	assert.Less(t, len(after), len(before))
	assert.Contains(t, mustMarshalJSON(t, notification), "latest_comment_url", "legacy formatter must retain its existing links")
	t.Logf("populated notification projection: %d -> %d JSON bytes", len(before), len(after))
}

func TestDiscussionEmptyCollections(t *testing.T) {
	deps := BaseDeps{GQLClient: githubv4.NewClient(&http.Client{
		Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request struct {
				Query string `json:"query"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			repository := map[string]any{"discussions": map[string]any{"nodes": nil}}
			if strings.Contains(request.Query, "discussionCategories(") {
				repository = map[string]any{"discussionCategories": map[string]any{"nodes": nil}}
			} else if strings.Contains(request.Query, "comments(") {
				repository = map[string]any{"discussion": map[string]any{"comments": map[string]any{"nodes": nil}}}
			}
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"repository": repository,
			}}))
		})},
	})}
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		t.Run(protocol, func(t *testing.T) {
			server := mcp.NewServer(&mcp.Implementation{Name: "empty-collections-test", Version: "v1"}, nil)
			server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
			for _, tool := range []inventory.ServerTool{
				ListDiscussions(translations.NullTranslationHelper),
				GetDiscussionComments(translations.NullTranslationHelper),
				ListDiscussionCategories(translations.NullTranslationHelper),
			} {
				tool.RegisterFunc(server, deps)
			}
			client := connectCommentVisibilityClient(t, server, protocol)
			for _, tc := range []struct {
				name, collection string
			}{
				{"list_discussions", "discussions"},
				{"get_discussion_comments", "comments"},
				{"list_discussion_categories", "categories"},
			} {
				result, err := client.CallTool(context.Background(), &mcp.CallToolParams{
					Name: tc.name, Arguments: map[string]any{"owner": "owner", "repo": "repo", "discussionNumber": 1},
				})
				require.NoError(t, err)
				require.False(t, result.IsError, "%s", result)
				var text map[string]any
				require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &text))
				if protocol == "2025-11-25" {
					assert.Nil(t, text[tc.collection], "legacy nil collections retain JSON null")
					assert.Nil(t, result.StructuredContent)
				} else {
					var modern map[string]any
					require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &modern))
					assert.Equal(t, []any{}, modern[tc.collection])
					assert.Equal(t, text, modern, "modern empty collections use the same DTO in both representations")
				}
			}
		})
	}
}

func TestNotificationOutputProviderValuesAndReferenceSchema(t *testing.T) {
	schema := notificationOutputSchema(false)
	assert.NotContains(t, schema.Properties, "url")
	assert.Contains(t, schema.Properties["subject"].Properties, "url")
	assert.NotContains(t, schema.Properties["subject"].Properties, "latest_comment_url")
	assert.Contains(t, schema.Properties["repository"].Properties, "html_url")
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)
	assert.Empty(t, schema.Properties["reason"].Enum)
	assert.Empty(t, schema.Properties["subject"].Properties["type"].Enum)
	for _, reason := range []string{
		"agent_session_finished", "approval_requested", "assign", "author", "ci_activity",
		"comment", "invitation", "manual", "member_feature_requested", "mention",
		"review_requested", "security_advisory_credit", "security_alert",
		"state_change", "subscribed", "team_mention", "future_reason",
	} {
		assert.NoError(t, resolved.Validate(map[string]any{"reason": reason}), reason)
	}
	for _, subjectType := range []string{
		"CheckSuite", "Commit", "Discussion", "Issue", "PullRequest", "Release",
		"RepositoryInvitation", "SecurityAdvisory", "FutureSubject",
	} {
		assert.NoError(t, resolved.Validate(map[string]any{"subject": map[string]any{"type": subjectType}}), subjectType)
	}
	assert.Error(t, resolved.Validate(map[string]any{"reason": 42}))
	assert.Error(t, resolved.Validate(map[string]any{"subject": map[string]any{"type": 42}}))
	subscription := discussionNotificationOutputSchema[NotificationSubscriptionOutput]()
	for _, field := range []string{"url", "thread_url", "repository_url"} {
		assert.NotContains(t, subscription.Properties, field)
	}
}
