package github

import (
	"context"
	"testing"

	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/internal/githubv4mock"
)

func Test_ParseOwnerRepo(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    OwnerRepo
		wantErr bool
	}{
		{
			name:  "valid owner/repo",
			input: "octocat/hello-world",
			want:  OwnerRepo{Owner: "octocat", Repo: "hello-world"},
		},
		{
			name:    "missing repo",
			input:   "octocat/",
			wantErr: true,
		},
		{
			name:    "missing owner",
			input:   "/hello-world",
			wantErr: true,
		},
		{
			name:    "missing both",
			input:   "/",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "too many parts",
			input:   "octocat/hello/world",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOwnerRepo(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func Test_IsRepoPrivate(t *testing.T) {
	tests := []struct {
		name      string
		ownerRepo OwnerRepo
		mockData  githubv4mock.Matcher
		want      bool
		wantErr   bool
	}{
		{
			name:      "public repository",
			ownerRepo: OwnerRepo{Owner: "octocat", Repo: "hello-world"},
			mockData: githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						IsPrivate githubv4.Boolean
					} `graphql:"repository(owner: $owner, name: $name)"`
				}{},
				map[string]interface{}{
					"owner": githubv4.String("octocat"),
					"name":  githubv4.String("hello-world"),
				},
				githubv4mock.DataResponse(map[string]interface{}{
					"repository": map[string]interface{}{
						"isPrivate": false,
					},
				}),
			),
			want:    false,
			wantErr: false,
		},
		{
			name:      "private repository",
			ownerRepo: OwnerRepo{Owner: "octocat", Repo: "hello-world"},
			mockData: githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						IsPrivate githubv4.Boolean
					} `graphql:"repository(owner: $owner, name: $name)"`
				}{},
				map[string]interface{}{
					"owner": githubv4.String("octocat"),
					"name":  githubv4.String("hello-world"),
				},
				githubv4mock.DataResponse(map[string]interface{}{
					"repository": map[string]interface{}{
						"isPrivate": true,
					},
				}),
			),
			want:    true,
			wantErr: false,
		},
		{
			name:      "repository not found",
			ownerRepo: OwnerRepo{Owner: "octocat", Repo: "not-found"},
			mockData: githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						IsPrivate githubv4.Boolean
					} `graphql:"repository(owner: $owner, name: $name)"`
				}{},
				map[string]interface{}{
					"owner": githubv4.String("octocat"),
					"name":  githubv4.String("not-found"),
				},
				githubv4mock.ErrorResponse("repository not found"),
			),
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(tt.mockData))
			getGQLClient := func(context.Context) (*githubv4.Client, error) {
				return client, nil
			}

			got, err := IsRepoPrivate(context.Background(), tt.ownerRepo, getGQLClient)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func Test_HasPushAccess(t *testing.T) {
	// Setup a test context with content filter settings
	ctx := context.Background()
	ownerRepo := OwnerRepo{Owner: "octocat", Repo: "hello-world"}
	settings := &ContentFilterSettings{
		Enabled:      true,
		TrustedRepo:  "octocat/hello-world",
		OwnerRepo:    ownerRepo,
		IsPrivate:    false,
		TrustedUsers: map[string]bool{},
	}
	ctx = context.WithValue(ctx, contentFilterKey, settings)

	tests := []struct {
		name     string
		username string
		mockData githubv4mock.Matcher
		want     bool
		wantErr  bool
	}{
		{
			name:     "user with push access",
			username: "contributor",
			mockData: githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						Collaborators struct {
							Edges []struct {
								Permission githubv4.String
								Node       struct {
									Login githubv4.String
								}
							}
						} `graphql:"collaborators(query: $username, first: 1)"`
					} `graphql:"repository(owner: $owner, name: $name)"`
				}{},
				map[string]interface{}{
					"owner":    githubv4.String("octocat"),
					"name":     githubv4.String("hello-world"),
					"username": githubv4.String("contributor"),
				},
				githubv4mock.DataResponse(map[string]interface{}{
					"repository": map[string]interface{}{
						"collaborators": map[string]interface{}{
							"edges": []interface{}{
								map[string]interface{}{
									"permission": "WRITE",
									"node": map[string]interface{}{
										"login": "contributor",
									},
								},
							},
						},
					},
				}),
			),
			want:    true,
			wantErr: false,
		},
		{
			name:     "user without push access",
			username: "reader",
			mockData: githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						Collaborators struct {
							Edges []struct {
								Permission githubv4.String
								Node       struct {
									Login githubv4.String
								}
							}
						} `graphql:"collaborators(query: $username, first: 1)"`
					} `graphql:"repository(owner: $owner, name: $name)"`
				}{},
				map[string]interface{}{
					"owner":    githubv4.String("octocat"),
					"name":     githubv4.String("hello-world"),
					"username": githubv4.String("reader"),
				},
				githubv4mock.DataResponse(map[string]interface{}{
					"repository": map[string]interface{}{
						"collaborators": map[string]interface{}{
							"edges": []interface{}{
								map[string]interface{}{
									"permission": "READ",
									"node": map[string]interface{}{
										"login": "reader",
									},
								},
							},
						},
					},
				}),
			),
			want:    false,
			wantErr: false,
		},
		{
			name:     "user not found",
			username: "not-found",
			mockData: githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						Collaborators struct {
							Edges []struct {
								Permission githubv4.String
								Node       struct {
									Login githubv4.String
								}
							}
						} `graphql:"collaborators(query: $username, first: 1)"`
					} `graphql:"repository(owner: $owner, name: $name)"`
				}{},
				map[string]interface{}{
					"owner":    githubv4.String("octocat"),
					"name":     githubv4.String("hello-world"),
					"username": githubv4.String("not-found"),
				},
				githubv4mock.DataResponse(map[string]interface{}{
					"repository": map[string]interface{}{
						"collaborators": map[string]interface{}{
							"edges": []interface{}{},
						},
					},
				}),
			),
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(tt.mockData))
			getGQLClient := func(context.Context) (*githubv4.Client, error) {
				return client, nil
			}

			// Reset trusted users for each test
			settings.TrustedUsers = map[string]bool{}

			got, err := HasPushAccess(ctx, tt.username, getGQLClient)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			// Check if the result was cached
			settings.mu.RLock()
			cachedValue, found := settings.TrustedUsers[tt.username]
			settings.mu.RUnlock()
			assert.True(t, found)
			assert.Equal(t, tt.want, cachedValue)
		})
	}
}

func Test_ShouldIncludeContent(t *testing.T) {
	tests := []struct {
		name             string
		setupCtx         func() context.Context
		username         string
		setupMockClient  func() GetGQLClientFn
		expectedIncluded bool
	}{
		{
			name: "content filtering disabled",
			setupCtx: func() context.Context {
				return context.Background()
			},
			username: "any-user",
			setupMockClient: func() GetGQLClientFn {
				return func(context.Context) (*githubv4.Client, error) {
					return nil, nil
				}
			},
			expectedIncluded: true,
		},
		{
			name: "private repository",
			setupCtx: func() context.Context {
				ctx := context.Background()
				settings := &ContentFilterSettings{
					Enabled:      true,
					TrustedRepo:  "octocat/hello-world",
					OwnerRepo:    OwnerRepo{Owner: "octocat", Repo: "hello-world"},
					IsPrivate:    true,
					TrustedUsers: map[string]bool{},
				}
				return context.WithValue(ctx, contentFilterKey, settings)
			},
			username: "any-user",
			setupMockClient: func() GetGQLClientFn {
				return func(context.Context) (*githubv4.Client, error) {
					return nil, nil
				}
			},
			expectedIncluded: true,
		},
		{
			name: "user with push access",
			setupCtx: func() context.Context {
				ctx := context.Background()
				settings := &ContentFilterSettings{
					Enabled:      true,
					TrustedRepo:  "octocat/hello-world",
					OwnerRepo:    OwnerRepo{Owner: "octocat", Repo: "hello-world"},
					IsPrivate:    false,
					TrustedUsers: map[string]bool{"trusted-user": true},
				}
				return context.WithValue(ctx, contentFilterKey, settings)
			},
			username: "trusted-user",
			setupMockClient: func() GetGQLClientFn {
				return func(context.Context) (*githubv4.Client, error) {
					return nil, nil
				}
			},
			expectedIncluded: true,
		},
		{
			name: "user without push access",
			setupCtx: func() context.Context {
				ctx := context.Background()
				settings := &ContentFilterSettings{
					Enabled:      true,
					TrustedRepo:  "octocat/hello-world",
					OwnerRepo:    OwnerRepo{Owner: "octocat", Repo: "hello-world"},
					IsPrivate:    false,
					TrustedUsers: map[string]bool{"untrusted-user": false},
				}
				return context.WithValue(ctx, contentFilterKey, settings)
			},
			username: "untrusted-user",
			setupMockClient: func() GetGQLClientFn {
				return func(context.Context) (*githubv4.Client, error) {
					return nil, nil
				}
			},
			expectedIncluded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			getGQLClient := tt.setupMockClient()

			included := ShouldIncludeContent(ctx, tt.username, getGQLClient)
			assert.Equal(t, tt.expectedIncluded, included)
		})
	}
}

func Test_InitContentFilter(t *testing.T) {
	tests := []struct {
		name        string
		trustedRepo string
		mockData    githubv4mock.Matcher
		wantErr     bool
		checkFilter func(t *testing.T, ctx context.Context)
	}{
		{
			name:        "empty trusted repo",
			trustedRepo: "",
			mockData:    githubv4mock.NewQueryMatcher(struct{}{}, map[string]interface{}{}, githubv4mock.DataResponse(map[string]interface{}{})),
			wantErr:     false,
			checkFilter: func(t *testing.T, ctx context.Context) {
				settings, ok := GetContentFilterFromContext(ctx)
				assert.False(t, ok)
				assert.Nil(t, settings)
			},
		},
		{
			name:        "invalid trusted repo format",
			trustedRepo: "invalid-format",
			mockData:    githubv4mock.NewQueryMatcher(struct{}{}, map[string]interface{}{}, githubv4mock.DataResponse(map[string]interface{}{})),
			wantErr:     true,
			checkFilter: func(t *testing.T, ctx context.Context) {
				settings, ok := GetContentFilterFromContext(ctx)
				assert.False(t, ok)
				assert.Nil(t, settings)
			},
		},
		{
			name:        "public repository",
			trustedRepo: "octocat/hello-world",
			mockData: githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						IsPrivate githubv4.Boolean
					} `graphql:"repository(owner: $owner, name: $name)"`
				}{},
				map[string]interface{}{
					"owner": githubv4.String("octocat"),
					"name":  githubv4.String("hello-world"),
				},
				githubv4mock.DataResponse(map[string]interface{}{
					"repository": map[string]interface{}{
						"isPrivate": false,
					},
				}),
			),
			wantErr: false,
			checkFilter: func(t *testing.T, ctx context.Context) {
				settings, ok := GetContentFilterFromContext(ctx)
				assert.True(t, ok)
				require.NotNil(t, settings)
				assert.True(t, settings.Enabled)
				assert.Equal(t, "octocat/hello-world", settings.TrustedRepo)
				assert.Equal(t, "octocat", settings.OwnerRepo.Owner)
				assert.Equal(t, "hello-world", settings.OwnerRepo.Repo)
				assert.False(t, settings.IsPrivate)
			},
		},
		{
			name:        "private repository",
			trustedRepo: "octocat/private-repo",
			mockData: githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						IsPrivate githubv4.Boolean
					} `graphql:"repository(owner: $owner, name: $name)"`
				}{},
				map[string]interface{}{
					"owner": githubv4.String("octocat"),
					"name":  githubv4.String("private-repo"),
				},
				githubv4mock.DataResponse(map[string]interface{}{
					"repository": map[string]interface{}{
						"isPrivate": true,
					},
				}),
			),
			wantErr: false,
			checkFilter: func(t *testing.T, ctx context.Context) {
				settings, ok := GetContentFilterFromContext(ctx)
				assert.True(t, ok)
				require.NotNil(t, settings)
				assert.True(t, settings.Enabled)
				assert.Equal(t, "octocat/private-repo", settings.TrustedRepo)
				assert.Equal(t, "octocat", settings.OwnerRepo.Owner)
				assert.Equal(t, "private-repo", settings.OwnerRepo.Repo)
				assert.True(t, settings.IsPrivate)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var getGQLClient GetGQLClientFn
			client := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(tt.mockData))
			getGQLClient = func(context.Context) (*githubv4.Client, error) {
				return client, nil
			}

			ctx, err := InitContentFilter(context.Background(), tt.trustedRepo, getGQLClient)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.checkFilter(t, ctx)
			}
		})
	}
}
