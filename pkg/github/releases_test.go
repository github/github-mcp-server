package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_CreateRelease(t *testing.T) {
	toolDef := CreateRelease(translations.NullTranslationHelper)
	assert.Equal(t, "create_release", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "tag_name")
}

func Test_CreateRelease_Execute(t *testing.T) {
	serverTool := CreateRelease(translations.NullTranslationHelper)

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful release creation",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"POST /repos/owner/repo/releases": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					release := &github.RepositoryRelease{
						ID:         github.Ptr(int64(1)),
						TagName:    github.Ptr("v1.0.0"),
						Name:       github.Ptr("Release 1.0.0"),
						Body:       github.Ptr("First release"),
						Draft:      github.Ptr(false),
						Prerelease: github.Ptr(false),
						HTMLURL:    github.Ptr("https://github.com/owner/repo/releases/tag/v1.0.0"),
					}
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(release)
				}),
			}),
			requestArgs: map[string]any{
				"owner":    "owner",
				"repo":     "repo",
				"tag_name": "v1.0.0",
				"name":     "Release 1.0.0",
				"body":     "First release",
			},
			expectError: false,
		},
		{
			name: "create draft prerelease",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"POST /repos/owner/repo/releases": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					release := &github.RepositoryRelease{
						ID:         github.Ptr(int64(2)),
						TagName:    github.Ptr("v2.0.0-beta"),
						Name:       github.Ptr("Beta Release"),
						Draft:      github.Ptr(true),
						Prerelease: github.Ptr(true),
						HTMLURL:    github.Ptr("https://github.com/owner/repo/releases/tag/v2.0.0-beta"),
					}
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(release)
				}),
			}),
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"tag_name":   "v2.0.0-beta",
				"name":       "Beta Release",
				"draft":      true,
				"prerelease": true,
			},
			expectError: false,
		},
		{
			name:         "missing required tag_name",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: tag_name",
		},
		{
			name:         "invalid tag name with spaces",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"owner":    "owner",
				"repo":     "repo",
				"tag_name": "invalid tag",
			},
			expectError:    true,
			expectedErrMsg: "invalid tag name",
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
			if tc.expectError {
				require.NoError(t, err)
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				textContent := getTextResult(t, result)
				assert.NotEmpty(t, textContent.Text)
			}
		})
	}
}

func Test_UpdateRelease(t *testing.T) {
	toolDef := UpdateRelease(translations.NullTranslationHelper)
	assert.Equal(t, "update_release", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "release_id")
}

func Test_UpdateRelease_Execute(t *testing.T) {
	serverTool := UpdateRelease(translations.NullTranslationHelper)

	tests := []struct {
		name         string
		mockedClient *http.Client
		requestArgs  map[string]any
		expectError  bool
	}{
		{
			name: "successful update",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"PATCH /repos/owner/repo/releases/1": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					release := &github.RepositoryRelease{
						ID:      github.Ptr(int64(1)),
						TagName: github.Ptr("v1.0.1"),
						Name:    github.Ptr("Updated Release"),
						HTMLURL: github.Ptr("https://github.com/owner/repo/releases/tag/v1.0.1"),
					}
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(release)
				}),
			}),
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"release_id": float64(1),
				"name":       "Updated Release",
			},
			expectError: false,
		},
		{
			name:         "missing release_id",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
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
			if tc.expectError {
				require.NoError(t, err)
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, "missing required parameter")
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

func Test_DeleteRelease(t *testing.T) {
	toolDef := DeleteRelease(translations.NullTranslationHelper)
	assert.Equal(t, "delete_release", toolDef.Tool.Name)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "release_id")
}

func Test_DeleteRelease_Execute(t *testing.T) {
	serverTool := DeleteRelease(translations.NullTranslationHelper)

	t.Run("successful deletion", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"DELETE /repos/owner/repo/releases/42": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		})

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":      "owner",
			"repo":       "repo",
			"release_id": float64(42),
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "deleted successfully")
	})
}

func Test_GetReleaseByID(t *testing.T) {
	serverTool := GetReleaseByID(translations.NullTranslationHelper)
	assert.Equal(t, "get_release", serverTool.Tool.Name)

	t.Run("successful get", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/owner/repo/releases/1": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				release := &github.RepositoryRelease{
					ID:      github.Ptr(int64(1)),
					TagName: github.Ptr("v1.0.0"),
					Name:    github.Ptr("Release 1.0.0"),
					HTMLURL: github.Ptr("https://github.com/owner/repo/releases/tag/v1.0.0"),
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(release)
			}),
		})

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":      "owner",
			"repo":       "repo",
			"release_id": float64(1),
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "v1.0.0")
	})
}

func Test_ListReleaseAssets(t *testing.T) {
	serverTool := ListReleaseAssets(translations.NullTranslationHelper)
	assert.Equal(t, "list_release_assets", serverTool.Tool.Name)

	t.Run("successful list", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/owner/repo/releases/1/assets": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				assets := []*github.ReleaseAsset{
					{
						ID:                 github.Ptr(int64(100)),
						Name:               github.Ptr("app.zip"),
						ContentType:        github.Ptr("application/zip"),
						Size:               github.Ptr(1024),
						DownloadCount:      github.Ptr(50),
						BrowserDownloadURL: github.Ptr("https://github.com/owner/repo/releases/download/v1.0.0/app.zip"),
					},
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(assets)
			}),
		})

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":      "owner",
			"repo":       "repo",
			"release_id": float64(1),
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "app.zip")
	})
}

func Test_DeleteReleaseAsset(t *testing.T) {
	serverTool := DeleteReleaseAsset(translations.NullTranslationHelper)
	assert.Equal(t, "delete_release_asset", serverTool.Tool.Name)

	t.Run("successful deletion", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"DELETE /repos/owner/repo/releases/assets/100": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		})

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":    "owner",
			"repo":     "repo",
			"asset_id": float64(100),
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "deleted successfully")
	})
}

func Test_isValidTagName(t *testing.T) {
	tests := []struct {
		tag   string
		valid bool
	}{
		{"v1.0.0", true},
		{"release-2024", true},
		{"v2.0.0-beta.1", true},
		{"", false},
		{"tag with spaces", false},
		{"tag\ttab", false},
	}
	for _, tc := range tests {
		t.Run(tc.tag, func(t *testing.T) {
			assert.Equal(t, tc.valid, isValidTagName(tc.tag))
		})
	}
}
