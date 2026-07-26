package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/translations"
)

// The end-to-end guarantee for the two GraphQL-backed issue_read methods.
// TestIssueReadOutputValidatesAgainstDeclaredSchema covers the REST methods
// (get, get_comments, get_sub_issues); get_parent and get_labels never touch
// the REST client, so they need the githubv4mock harness instead and are
// covered here.
//
// Same contract as the REST test: run the REAL handler, take the single text
// block, and validate it against the advertised outputSchema. The
// structured-content mirror publishes those exact bytes as structuredContent,
// so validating the text block is equivalent to validating what a client sees.
func TestIssueReadGraphQLOutputValidatesAgainstDeclaredSchema(t *testing.T) {
	t.Parallel()

	serverTool := IssueRead(translations.NullTranslationHelper)
	resolved := resolveToolSchema(t, issueReadOutputSchema)

	// Negative controls. The union is wide — five branches, two of them bare
	// arrays — so "the handler's output validated" only means something if
	// near-miss payloads for these two branches are actually rejected. If any
	// of these starts passing, every subtest below has gone vacuous.
	for _, nearMiss := range []any{
		// labels typed as array: a nil slice marshalled to null must fail.
		map[string]any{"labels": nil, "totalCount": 0},
		// a label missing id/color/description must fail its required set.
		map[string]any{"labels": []any{map[string]any{"name": "bug"}}, "totalCount": 1},
		// totalCount is required alongside labels.
		map[string]any{"labels": []any{}},
		// a parent ref missing title/state/url must fail issueRef.
		map[string]any{"parent": map[string]any{"number": 42}},
	} {
		require.Error(t, resolved.Validate(nearMiss),
			"schema must reject %v, or the conformance subtests below prove nothing", nearMiss)
	}

	// Query shapes must match the handler's anonymous structs exactly, or
	// githubv4mock refuses to serve a response and the handler errors out.
	parentQuery := struct {
		Repository struct {
			Issue struct {
				Parent *struct {
					Number githubv4.Int
					Title  githubv4.String
					State  githubv4.String
					URL    githubv4.String
					Author struct {
						Login githubv4.String
					}
					Repository struct {
						NameWithOwner githubv4.String
					}
				}
			} `graphql:"issue(number: $issueNumber)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}{}

	labelsQuery := struct {
		Repository struct {
			Issue struct {
				Labels struct {
					Nodes []struct {
						ID          githubv4.ID
						Name        githubv4.String
						Color       githubv4.String
						Description githubv4.String
					}
					TotalCount githubv4.Int
				} `graphql:"labels(first: 100)"`
			} `graphql:"issue(number: $issueNumber)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}{}

	vars := map[string]any{
		"owner":       githubv4.String("octocat"),
		"repo":        githubv4.String("repo"),
		"issueNumber": githubv4.Int(1),
	}

	tests := []struct {
		name    string
		method  string
		gqlHTTP *http.Client
		// wantContains anchors each subtest to the payload it meant to
		// produce, so a handler that silently returned some other shape
		// cannot coast through on a permissive union branch.
		wantContains string
	}{
		{
			name:   "get_parent",
			method: "get_parent",
			gqlHTTP: githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher(parentQuery, vars,
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{
							"parent": map[string]any{
								"number": githubv4.Int(42),
								"title":  githubv4.String("Parent issue"),
								"state":  githubv4.String("OPEN"),
								"url":    githubv4.String("https://github.com/octocat/repo/issues/42"),
								"author": map[string]any{
									"login": githubv4.String("octocat"),
								},
								"repository": map[string]any{
									"nameWithOwner": githubv4.String("octocat/repo"),
								},
							},
						},
					},
				}))),
			wantContains: `"number":42`,
		},
		{
			// The wrapper's only key is null. `parent` is still required, so
			// this exercises the branch's required set rather than its
			// properties.
			name:   "get_parent with no parent",
			method: "get_parent",
			gqlHTTP: githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher(parentQuery, vars,
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{
							"parent": nil,
						},
					},
				}))),
			wantContains: `"parent":null`,
		},
		{
			// A parent whose every optional GraphQL field came back empty.
			// The handler still has to emit all five issueRef keys; if it
			// ever started omitting empties, number/title/state/url would
			// go missing and this subtest would fail.
			name:   "get_parent with all optionals absent",
			method: "get_parent",
			gqlHTTP: githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher(parentQuery, vars,
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{
							"parent": map[string]any{},
						},
					},
				}))),
			wantContains: `"repository":""`,
		},
		{
			name:   "get_labels",
			method: "get_labels",
			gqlHTTP: githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher(labelsQuery, vars,
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{
							"labels": map[string]any{
								"nodes": []any{
									map[string]any{
										"id":          githubv4.ID("LA_label1"),
										"name":        githubv4.String("bug"),
										"color":       githubv4.String("d73a4a"),
										"description": githubv4.String("Something isn't working"),
									},
								},
								"totalCount": githubv4.Int(1),
							},
						},
					},
				}))),
			wantContains: `"name":"bug"`,
		},
		{
			// Empty collection: `labels` and `totalCount` are both required,
			// so [] and 0 must still be emitted.
			name:   "get_labels with no labels",
			method: "get_labels",
			gqlHTTP: githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher(labelsQuery, vars,
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{
							"labels": map[string]any{
								"nodes":      []any{},
								"totalCount": githubv4.Int(0),
							},
						},
					},
				}))),
			wantContains: `"labels":[]`,
		},
		{
			// Same trap that get_sub_issues fell into: a null node list
			// leaves the Go slice nil, which marshals to `null` unless the
			// handler normalises it. The schema types `labels` as an array,
			// so `null` would be a violation.
			name:   "get_labels with null node list",
			method: "get_labels",
			gqlHTTP: githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher(labelsQuery, vars,
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{
							"labels": map[string]any{
								"nodes":      nil,
								"totalCount": githubv4.Int(0),
							},
						},
					},
				}))),
			wantContains: `"labels":[]`,
		},
		{
			// A label with every optional field absent. The handler builds
			// the map by hand and always sets all four keys, which is what
			// the branch's required list depends on.
			name:   "get_labels with all optionals absent",
			method: "get_labels",
			gqlHTTP: githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher(labelsQuery, vars,
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{
							"labels": map[string]any{
								"nodes":      []any{map[string]any{}},
								"totalCount": githubv4.Int(1),
							},
						},
					},
				}))),
			wantContains: `"description":""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := BaseDeps{
				Client:    mustNewGHClient(t, nil),
				GQLClient: githubv4.NewClient(tt.gqlHTTP),
				// get_parent consults the lockdown cache and flags before
				// deciding whether to return the parent at all.
				RepoAccessCache: stubRepoAccessCache(nil, 15*time.Minute),
				Flags:           stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(map[string]any{
				"method":       tt.method,
				"owner":        "octocat",
				"repo":         "repo",
				"issue_number": float64(1),
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, "handler should succeed")

			text := getTextResult(t, result)
			assert.Contains(t, text.Text, tt.wantContains,
				"subtest must exercise the payload it describes")

			var payload any
			require.NoError(t, json.Unmarshal([]byte(text.Text), &payload),
				"handler output must be JSON for the mirror to publish it as structuredContent")

			require.NoError(t, resolved.Validate(payload),
				"real handler output for method=%s must conform to the advertised outputSchema", tt.method)
		})
	}
}
