package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
)

// The end-to-end guarantee for discussion_comment_write: what the real handler
// emits must validate against the outputSchema the tool advertises. The
// structured-content mirror publishes the exact bytes of the text block as
// structuredContent, so validating the text block here is equivalent to
// validating what a client receives.
//
// Every value of the tool's `method` enum is exercised, and the enum itself is
// read back off the tool at the end so a newly added method cannot slip through
// without a conformance case.
func TestDiscussionCommentWriteOutputValidatesAgainstDeclaredSchema(t *testing.T) {
	t.Parallel()

	serverTool := DiscussionCommentWrite(translations.NullTranslationHelper)
	resolved := resolveToolSchema(t, discussionCommentWriteOutputSchema)

	// Resolving the discussion node ID from its number: shared by add and reply.
	discussionQueryMatcher := discussionCommentWriteDiscussionQueryMatcher(
		1,
		githubv4mock.DataResponse(map[string]any{
			"repository": map[string]any{
				"discussion": map[string]any{"id": "D_kwDOTest123"},
			},
		}),
	)

	// The reply path validates the target comment node before mutating.
	replyValidationMatcher := discussionCommentWriteReplyValidationQueryMatcher(
		"DC_kwDOComment456",
		githubv4mock.DataResponse(map[string]any{
			"node": map[string]any{
				"id":         "DC_kwDOComment456",
				"discussion": map[string]any{"id": "D_kwDOTest123"},
			},
		}),
	)

	tests := []struct {
		name         string
		method       string
		requestArgs  map[string]any
		mockedClient *http.Client
	}{
		{
			name:   "add",
			method: "add",
			requestArgs: map[string]any{
				"method":           "add",
				"owner":            "owner",
				"repo":             "repo",
				"discussionNumber": int32(1),
				"body":             "This is a test comment",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				discussionQueryMatcher,
				discussioncommentwriteAddMutationMatcher(
					githubv4.AddDiscussionCommentInput{
						DiscussionID: githubv4.ID("D_kwDOTest123"),
						Body:         githubv4.String("This is a test comment"),
					},
					githubv4mock.DataResponse(map[string]any{
						"addDiscussionComment": map[string]any{
							"comment": map[string]any{
								"id":  "DC_kwDOComment456",
								"url": "https://github.com/owner/repo/discussions/1#discussioncomment-456",
							},
						},
					}),
				),
			),
		},
		{
			// The sparse case: the mutation payload comes back with the comment
			// object entirely absent, so every field the handler reads is unset.
			// Both keys are required by the schema branch, so this is what
			// proves the handler always emits them rather than omitting on zero.
			//
			// It also pins a wart: githubv4.ID is `any`, and the handler
			// stringifies it with fmt.Sprintf("%v", ...), so an unset ID emits
			// the literal string "<nil>" rather than "". That conforms (the
			// schema says string, and it is one), so this test cannot fail on
			// it — but it is the shape a client would actually receive.
			name:   "add with mutation payload fields absent",
			method: "add",
			requestArgs: map[string]any{
				"method":           "add",
				"owner":            "owner",
				"repo":             "repo",
				"discussionNumber": int32(1),
				"body":             "This is a test comment",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				discussionQueryMatcher,
				discussioncommentwriteAddMutationMatcher(
					githubv4.AddDiscussionCommentInput{
						DiscussionID: githubv4.ID("D_kwDOTest123"),
						Body:         githubv4.String("This is a test comment"),
					},
					githubv4mock.DataResponse(map[string]any{
						"addDiscussionComment": map[string]any{"comment": nil},
					}),
				),
			),
		},
		{
			name:   "reply",
			method: "reply",
			requestArgs: map[string]any{
				"method":           "reply",
				"owner":            "owner",
				"repo":             "repo",
				"discussionNumber": int32(1),
				"body":             "This is a reply",
				"commentNodeID":    "DC_kwDOComment456",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				replyValidationMatcher,
				discussionQueryMatcher,
				discussioncommentwriteAddMutationMatcher(
					githubv4.AddDiscussionCommentInput{
						DiscussionID: githubv4.ID("D_kwDOTest123"),
						Body:         githubv4.String("This is a reply"),
						ReplyToID:    githubv4ptr("DC_kwDOComment456"),
					},
					githubv4mock.DataResponse(map[string]any{
						"addDiscussionComment": map[string]any{
							"comment": map[string]any{
								"id":  "DC_kwDOReply789",
								"url": "https://github.com/owner/repo/discussions/1#discussioncomment-789",
							},
						},
					}),
				),
			),
		},
		{
			name:   "reply with mutation payload fields absent",
			method: "reply",
			requestArgs: map[string]any{
				"method":           "reply",
				"owner":            "owner",
				"repo":             "repo",
				"discussionNumber": int32(1),
				"body":             "This is a reply",
				"commentNodeID":    "DC_kwDOComment456",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				replyValidationMatcher,
				discussionQueryMatcher,
				discussioncommentwriteAddMutationMatcher(
					githubv4.AddDiscussionCommentInput{
						DiscussionID: githubv4.ID("D_kwDOTest123"),
						Body:         githubv4.String("This is a reply"),
						ReplyToID:    githubv4ptr("DC_kwDOComment456"),
					},
					githubv4mock.DataResponse(map[string]any{
						"addDiscussionComment": map[string]any{"comment": nil},
					}),
				),
			),
		},
		{
			name:   "update",
			method: "update",
			requestArgs: map[string]any{
				"method":        "update",
				"commentNodeID": "DC_kwDOComment456",
				"body":          "Updated comment body",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				discussioncommentwriteUpdateMutationMatcher(
					githubv4.UpdateDiscussionCommentInput{
						CommentID: githubv4.ID("DC_kwDOComment456"),
						Body:      githubv4.String("Updated comment body"),
					},
					githubv4mock.DataResponse(map[string]any{
						"updateDiscussionComment": map[string]any{
							"comment": map[string]any{
								"id":  "DC_kwDOComment456",
								"url": "https://github.com/owner/repo/discussions/1#discussioncomment-456",
							},
						},
					}),
				),
			),
		},
		{
			// Empty strings rather than an absent object: the other way a
			// payload can be maximally uninformative while still well-formed.
			name:   "update with empty id and url",
			method: "update",
			requestArgs: map[string]any{
				"method":        "update",
				"commentNodeID": "DC_kwDOComment456",
				"body":          "Updated comment body",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				discussioncommentwriteUpdateMutationMatcher(
					githubv4.UpdateDiscussionCommentInput{
						CommentID: githubv4.ID("DC_kwDOComment456"),
						Body:      githubv4.String("Updated comment body"),
					},
					githubv4mock.DataResponse(map[string]any{
						"updateDiscussionComment": map[string]any{
							"comment": map[string]any{"id": "", "url": ""},
						},
					}),
				),
			),
		},
		{
			name:   "delete",
			method: "delete",
			requestArgs: map[string]any{
				"method":        "delete",
				"commentNodeID": "DC_kwDOComment456",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				discussioncommentwriteDeleteMutationMatcher(
					githubv4.DeleteDiscussionCommentInput{ID: githubv4.ID("DC_kwDOComment456")},
					githubv4mock.DataResponse(map[string]any{
						"deleteDiscussionComment": map[string]any{
							"comment": map[string]any{
								"id":  "DC_kwDOComment456",
								"url": "https://github.com/owner/repo/discussions/1#discussioncomment-456",
							},
						},
					}),
				),
			),
		},
		{
			name:   "delete with mutation payload fields absent",
			method: "delete",
			requestArgs: map[string]any{
				"method":        "delete",
				"commentNodeID": "DC_kwDOComment456",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				discussioncommentwriteDeleteMutationMatcher(
					githubv4.DeleteDiscussionCommentInput{ID: githubv4.ID("DC_kwDOComment456")},
					githubv4mock.DataResponse(map[string]any{
						"deleteDiscussionComment": map[string]any{"comment": nil},
					}),
				),
			),
		},
		{
			name:   "mark_answer",
			method: "mark_answer",
			requestArgs: map[string]any{
				"method":        "mark_answer",
				"commentNodeID": "DC_kwDOComment456",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				discussioncommentwriteMarkAnswerMutationMatcher(
					githubv4.MarkDiscussionCommentAsAnswerInput{ID: githubv4.ID("DC_kwDOComment456")},
					githubv4mock.DataResponse(map[string]any{
						"markDiscussionCommentAsAnswer": map[string]any{
							"discussion": map[string]any{
								"id":  "D_kwDOTest123",
								"url": "https://github.com/owner/repo/discussions/1",
							},
						},
					}),
				),
			),
		},
		{
			// Exercises the second anyOf branch's required set
			// (discussionID + discussionURL) with nothing to fill it from.
			name:   "mark_answer with mutation payload fields absent",
			method: "mark_answer",
			requestArgs: map[string]any{
				"method":        "mark_answer",
				"commentNodeID": "DC_kwDOComment456",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				discussioncommentwriteMarkAnswerMutationMatcher(
					githubv4.MarkDiscussionCommentAsAnswerInput{ID: githubv4.ID("DC_kwDOComment456")},
					githubv4mock.DataResponse(map[string]any{
						"markDiscussionCommentAsAnswer": map[string]any{"discussion": nil},
					}),
				),
			),
		},
		{
			name:   "unmark_answer",
			method: "unmark_answer",
			requestArgs: map[string]any{
				"method":        "unmark_answer",
				"commentNodeID": "DC_kwDOComment456",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				discussioncommentwriteUnmarkAnswerMutationMatcher(
					githubv4.UnmarkDiscussionCommentAsAnswerInput{ID: githubv4.ID("DC_kwDOComment456")},
					githubv4mock.DataResponse(map[string]any{
						"unmarkDiscussionCommentAsAnswer": map[string]any{
							"discussion": map[string]any{
								"id":  "D_kwDOTest123",
								"url": "https://github.com/owner/repo/discussions/1",
							},
						},
					}),
				),
			),
		},
		{
			name:   "unmark_answer with empty discussion id and url",
			method: "unmark_answer",
			requestArgs: map[string]any{
				"method":        "unmark_answer",
				"commentNodeID": "DC_kwDOComment456",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				discussioncommentwriteUnmarkAnswerMutationMatcher(
					githubv4.UnmarkDiscussionCommentAsAnswerInput{ID: githubv4.ID("DC_kwDOComment456")},
					githubv4mock.DataResponse(map[string]any{
						"unmarkDiscussionCommentAsAnswer": map[string]any{
							"discussion": map[string]any{"id": "", "url": ""},
						},
					}),
				),
			),
		},
	}

	covered := map[string]bool{}
	for _, tt := range tests {
		covered[tt.method] = true

		t.Run(tt.name, func(t *testing.T) {
			deps := BaseDeps{GQLClient: githubv4.NewClient(tt.mockedClient)}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tt.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, "handler should succeed; a tool error would validate vacuously")

			text := getTextResult(t, result)
			var payload any
			require.NoError(t, json.Unmarshal([]byte(text.Text), &payload),
				"handler output must be JSON for the mirror to publish it as structuredContent")

			require.NoError(t, resolved.Validate(payload),
				"real handler output for method=%s must conform to the advertised outputSchema: %s", tt.method, text.Text)
		})
	}

	// No method of this tool returns a collection, so there is no empty-list
	// case to exercise; the awkward cases here are the absent/empty payload
	// fields above. Guard the enum instead: a method added to the tool without
	// a conformance case is exactly the drift this file exists to catch.
	t.Run("every method in the enum is covered", func(t *testing.T) {
		schema, ok := serverTool.Tool.InputSchema.(*jsonschema.Schema)
		require.True(t, ok, "InputSchema should be *jsonschema.Schema")
		methodSchema, ok := schema.Properties["method"]
		require.True(t, ok, "tool should declare a method property")
		require.NotEmpty(t, methodSchema.Enum, "method should be an enum")

		for _, v := range methodSchema.Enum {
			method := fmt.Sprintf("%v", v)
			assert.True(t, covered[method],
				"method %q is advertised by the tool but has no output-conformance case", method)
		}
	})
}

func discussioncommentwriteAddMutationMatcher(input githubv4.AddDiscussionCommentInput, response githubv4mock.GQLResponse) githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			AddDiscussionComment struct {
				Comment struct {
					ID  githubv4.ID
					URL githubv4.String `graphql:"url"`
				}
			} `graphql:"addDiscussionComment(input: $input)"`
		}{},
		input,
		nil,
		response,
	)
}

func discussioncommentwriteUpdateMutationMatcher(input githubv4.UpdateDiscussionCommentInput, response githubv4mock.GQLResponse) githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			UpdateDiscussionComment struct {
				Comment struct {
					ID  githubv4.ID
					URL githubv4.String `graphql:"url"`
				}
			} `graphql:"updateDiscussionComment(input: $input)"`
		}{},
		input,
		nil,
		response,
	)
}

func discussioncommentwriteDeleteMutationMatcher(input githubv4.DeleteDiscussionCommentInput, response githubv4mock.GQLResponse) githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			DeleteDiscussionComment struct {
				Comment struct {
					ID  githubv4.ID
					URL githubv4.String `graphql:"url"`
				}
			} `graphql:"deleteDiscussionComment(input: $input)"`
		}{},
		input,
		nil,
		response,
	)
}

func discussioncommentwriteMarkAnswerMutationMatcher(input githubv4.MarkDiscussionCommentAsAnswerInput, response githubv4mock.GQLResponse) githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			MarkDiscussionCommentAsAnswer struct {
				Discussion struct {
					ID  githubv4.ID
					URL githubv4.String `graphql:"url"`
				}
			} `graphql:"markDiscussionCommentAsAnswer(input: $input)"`
		}{},
		input,
		nil,
		response,
	)
}

func discussioncommentwriteUnmarkAnswerMutationMatcher(input githubv4.UnmarkDiscussionCommentAsAnswerInput, response githubv4mock.GQLResponse) githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			UnmarkDiscussionCommentAsAnswer struct {
				Discussion struct {
					ID  githubv4.ID
					URL githubv4.String `graphql:"url"`
				}
			} `graphql:"unmarkDiscussionCommentAsAnswer(input: $input)"`
		}{},
		input,
		nil,
		response,
	)
}
