package github

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-github/v87/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parsePushFilesEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     any
		want    []pushFileEntry
		wantErr string
	}{
		{
			name: "array of maps",
			raw: []any{
				map[string]any{"path": "README.md", "content": "# Hello"},
			},
			want: []pushFileEntry{{Path: "README.md", Content: "# Hello"}},
		},
		{
			name: "typed slice via json round trip",
			raw: []map[string]any{
				{"path": "docs/a.md", "content": "alpha"},
				{"path": "docs/b.md", "content": "beta"},
			},
			want: []pushFileEntry{
				{Path: "docs/a.md", Content: "alpha"},
				{Path: "docs/b.md", Content: "beta"},
			},
		},
		{
			name: "json string payload",
			raw: `[{"path":"README.md","content":"from string"}]`,
			want: []pushFileEntry{{Path: "README.md", Content: "from string"}},
		},
		{
			name:    "invalid string payload",
			raw:     "not-json",
			wantErr: "files parameter must be an array",
		},
		{
			name:    "missing path",
			raw:     []any{map[string]any{"content": "x"}},
			wantErr: "each file must have a path",
		},
		{
			name:    "missing content",
			raw:     []any{map[string]any{"path": "a.txt"}},
			wantErr: "each file must have content",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePushFilesEntries(tc.raw)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_isGitRefUpdateConflict(t *testing.T) {
	t.Parallel()

	resp422 := &github.Response{Response: &http.Response{StatusCode: http.StatusUnprocessableEntity}}
	assert.True(t, isGitRefUpdateConflict(&github.ErrorResponse{
		Response: resp422.Response,
		Message:  "Update is not a fast forward",
	}, resp422))
	assert.True(t, isGitRefUpdateConflict(&github.ErrorResponse{
		Response: resp422.Response,
		Message:  "Cannot fast-forward",
	}, resp422))

	assert.False(t, isGitRefUpdateConflict(&github.ErrorResponse{
		Response: resp422.Response,
		Message:  "Reference does not exist",
	}, resp422))

	resp200 := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
	assert.False(t, isGitRefUpdateConflict(nil, resp200))
}

func Test_commitEntriesToRef_retriesOnFastForwardConflict(t *testing.T) {
	mockRefInitial := &github.Reference{
		Ref: github.Ptr("refs/heads/main"),
		Object: &github.GitObject{
			SHA: github.Ptr("abc123"),
		},
	}
	mockRefUpdated := &github.Reference{
		Ref: github.Ptr("refs/heads/main"),
		Object: &github.GitObject{
			SHA: github.Ptr("concurrent999"),
		},
	}
	mockCommitInitial := &github.Commit{
		SHA:  github.Ptr("abc123"),
		Tree: &github.Tree{SHA: github.Ptr("def456")},
	}
	mockCommitUpdated := &github.Commit{
		SHA:  github.Ptr("concurrent999"),
		Tree: &github.Tree{SHA: github.Ptr("tree999")},
	}
	mockTree := &github.Tree{SHA: github.Ptr("ghi789")}
	mockNewCommit := &github.Commit{SHA: github.Ptr("jkl012")}
	mockUpdatedRef := &github.Reference{
		Ref: github.Ptr("refs/heads/main"),
		Object: &github.GitObject{
			SHA: github.Ptr("jkl012"),
		},
	}

	getRefCalls := 0
	updateRefCalls := 0

	client := mustNewGHClient(t, NewMockedHTTPClient(
		WithRequestMatchHandler(
			GetReposGitRefByOwnerByRepoByRef,
			func(w http.ResponseWriter, _ *http.Request) {
				getRefCalls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				ref := mockRefInitial
				if getRefCalls >= 3 {
					ref = mockRefUpdated
				}
				require.NoError(t, json.NewEncoder(w).Encode(ref))
			},
		),
		WithRequestMatchHandler(
			GetReposGitCommitsByOwnerByRepoByCommitSHA,
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				commit := mockCommitInitial
				if r.URL.Path == "/repos/owner/repo/git/commits/concurrent999" {
					commit = mockCommitUpdated
				}
				require.NoError(t, json.NewEncoder(w).Encode(commit))
			},
		),
		WithRequestMatchHandler(
			PostReposGitTreesByOwnerByRepo,
			mockResponse(t, http.StatusCreated, mockTree),
		),
		WithRequestMatchHandler(
			PostReposGitCommitsByOwnerByRepo,
			mockResponse(t, http.StatusCreated, mockNewCommit),
		),
		WithRequestMatchHandler(
			PatchReposGitRefsByOwnerByRepoByRef,
			func(w http.ResponseWriter, _ *http.Request) {
				updateRefCalls++
				w.Header().Set("Content-Type", "application/json")
				if updateRefCalls == 1 {
					w.WriteHeader(http.StatusUnprocessableEntity)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
						"message": "Update is not a fast forward",
					}))
					return
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(mockUpdatedRef))
			},
		),
	))

	result, resp, err := commitEntriesToRef(
		t.Context(),
		client,
		"owner",
		"repo",
		"refs/heads/main",
		"Update files",
		pushFileEntriesToTreeEntries([]pushFileEntry{{Path: "README.md", Content: "# Hi"}}),
	)
	require.NoError(t, err)
	require.Nil(t, resp)
	require.NotNil(t, result)
	assert.Equal(t, "jkl012", result.Ref.GetObject().GetSHA())
	assert.Equal(t, 2, updateRefCalls)
	assert.Equal(t, 2, getRefCalls)
}
