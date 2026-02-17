package response

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlatten(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name: "promotes primitive fields from nested map",
			input: map[string]any{
				"title": "fix bug",
				"user": map[string]any{
					"login": "user",
					"id":    float64(1),
				},
			},
			expected: map[string]any{
				"title":      "fix bug",
				"user.login": "user",
				"user.id":    float64(1),
			},
		},
		{
			name: "drops nested maps at default depth",
			input: map[string]any{
				"user": map[string]any{
					"login": "user",
					"repos": []any{"repo1"},
					"org":   map[string]any{"name": "org"},
				},
			},
			expected: map[string]any{
				"user.login": "user",
				"user.repos": []any{"repo1"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := flattenTo(tc.input, maxFlattenDepth)
			assert.Equal(t, tc.expected, result)
		})
	}

	t.Run("recurses deeper with custom depth", func(t *testing.T) {
		input := map[string]any{
			"commit": map[string]any{
				"message": "fix bug",
				"author": map[string]any{
					"name": "user",
					"date": "2026-01-01",
				},
			},
		}
		result := flattenTo(input, 3)
		assert.Equal(t, map[string]any{
			"commit.message":     "fix bug",
			"commit.author.name": "user",
			"commit.author.date": "2026-01-01",
		}, result)
	})
}

func TestTrimArrayFields(t *testing.T) {
	original := collectionFieldExtractors
	defer func() { collectionFieldExtractors = original }()

	collectionFieldExtractors = map[string][]string{
		"reviewers": {"login", "state"},
	}

	result := optimizeItem(map[string]any{
		"reviewers": []any{
			map[string]any{"login": "alice", "state": "approved", "id": float64(1)},
			map[string]any{"login": "bob", "state": "changes_requested", "id": float64(2)},
		},
		"title": "Fix bug",
	})

	expected := []any{
		map[string]any{"login": "alice", "state": "approved"},
		map[string]any{"login": "bob", "state": "changes_requested"},
	}
	assert.Equal(t, expected, result["reviewers"])
	assert.Equal(t, "Fix bug", result["title"])
}

func TestFilterByFillRate(t *testing.T) {
	items := []map[string]any{
		{"title": "a", "body": "text", "milestone": "v1"},
		{"title": "b", "body": "text"},
		{"title": "c", "body": "text"},
		{"title": "d", "body": "text"},
		{"title": "e", "body": "text"},
		{"title": "f", "body": "text"},
		{"title": "g", "body": "text"},
		{"title": "h", "body": "text"},
		{"title": "i", "body": "text"},
		{"title": "j", "body": "text"},
	}

	result := filterByFillRate(items, 0.1)

	for _, item := range result {
		assert.Contains(t, item, "title")
		assert.Contains(t, item, "body")
		assert.NotContains(t, item, "milestone")
	}
}

func TestFilterByFillRate_PreservesFields(t *testing.T) {
	original := preservedFields
	defer func() { preservedFields = original }()

	preservedFields = map[string]bool{"html_url": true}

	items := make([]map[string]any, 10)
	for i := range items {
		items[i] = map[string]any{"title": "x"}
	}
	items[0]["html_url"] = "https://github.com/repo/1"

	result := filterByFillRate(items, 0.1)
	assert.Contains(t, result[0], "html_url")
}

func TestOptimizeItems(t *testing.T) {
	tests := []struct {
		name     string
		items    []map[string]any
		expected []map[string]any
	}{
		{
			name: "applies all strategies in sequence",
			items: []map[string]any{
				{
					"title":      "Fix bug",
					"body":       "line1\n\nline2",
					"url":        "https://api.github.com/repos/1",
					"html_url":   "https://github.com/repo/1",
					"avatar_url": "https://avatars.githubusercontent.com/1",
					"draft":      false,
					"merged_at":  nil,
					"labels":     []any{map[string]any{"name": "bug"}},
					"user": map[string]any{
						"login":      "user",
						"avatar_url": "https://avatars.githubusercontent.com/1",
					},
				},
			},
			expected: []map[string]any{
				{
					"title":      "Fix bug",
					"body":       "line1 line2",
					"html_url":   "https://github.com/repo/1",
					"draft":      false,
					"labels":     "bug",
					"user.login": "user",
				},
			},
		},
		{
			name:     "nil input",
			items:    nil,
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := OptimizeItems(tc.items, maxFlattenDepth)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestOptimizeItems_SkipsFillRateBelowMinRows(t *testing.T) {
	items := []map[string]any{
		{"title": "a", "rare": "x"},
		{"title": "b"},
	}

	result := OptimizeItems(items, maxFlattenDepth)
	assert.Equal(t, "x", result[0]["rare"])
}

func TestPreservedFields(t *testing.T) {
	original := preservedFields
	defer func() { preservedFields = original }()

	preservedFields = map[string]bool{
		"html_url":  true,
		"clone_url": true,
	}

	result := optimizeItem(map[string]any{
		"html_url":       "https://github.com/repo/1",
		"clone_url":      "https://github.com/repo/1.git",
		"avatar_url":     "https://avatars.githubusercontent.com/1",
		"user.html_url":  "https://github.com/user",
		"user.clone_url": "https://github.com/user.git",
	})

	assert.Contains(t, result, "html_url")
	assert.Contains(t, result, "clone_url")
	assert.NotContains(t, result, "avatar_url")
	assert.NotContains(t, result, "user.html_url")
	assert.NotContains(t, result, "user.clone_url")
}

func TestPreservedFields_ProtectsZeroValues(t *testing.T) {
	original := preservedFields
	defer func() { preservedFields = original }()

	preservedFields = map[string]bool{"draft": true}

	result := optimizeItem(map[string]any{
		"draft": false,
		"body":  "",
	})

	assert.Contains(t, result, "draft")
	assert.NotContains(t, result, "body")
}

func TestPreservedFields_ProtectsFromCollectionSummarization(t *testing.T) {
	original := preservedFields
	defer func() { preservedFields = original }()

	preservedFields = map[string]bool{"assignees": true}

	assignees := []any{
		map[string]any{"login": "alice", "id": float64(1)},
		map[string]any{"login": "bob", "id": float64(2)},
	}

	result := optimizeItem(map[string]any{
		"assignees": assignees,
		"comments":  []any{map[string]any{"id": "1"}, map[string]any{"id": "2"}},
	})

	assert.Equal(t, assignees, result["assignees"])
	assert.Equal(t, "[2 items]", result["comments"])
}

func TestCollectionFieldExtractors_SurviveFillRate(t *testing.T) {
	original := collectionFieldExtractors
	defer func() { collectionFieldExtractors = original }()

	collectionFieldExtractors = map[string][]string{"labels": {"name"}}

	items := []map[string]any{
		{"title": "PR 1", "labels": "bug"},
		{"title": "PR 2"},
		{"title": "PR 3"},
		{"title": "PR 4"},
	}

	result := filterByFillRate(items, defaultFillRateThreshold)

	assert.Contains(t, result[0], "labels")
}

func TestMarshalItems_PlainSlice(t *testing.T) {
	type commit struct {
		SHA     string `json:"sha"`
		Message string `json:"message"`
		URL     string `json:"url"`
	}

	data := []commit{
		{SHA: "abc123", Message: "fix bug", URL: "https://api.github.com/commits/abc123"},
		{SHA: "def456", Message: "add feature", URL: "https://api.github.com/commits/def456"},
		{SHA: "ghi789", Message: "update docs", URL: "https://api.github.com/commits/ghi789"},
	}

	raw, err := MarshalItems(data)
	require.NoError(t, err)

	var result []map[string]any
	err = json.Unmarshal(raw, &result)
	require.NoError(t, err)

	for _, item := range result {
		assert.NotEmpty(t, item["sha"])
		assert.NotEmpty(t, item["message"])
		assert.Nil(t, item["url"])
	}
}

func TestMarshalItems_WrappedObject(t *testing.T) {
	data := map[string]any{
		"issues": []any{
			map[string]any{
				"title":    "bug report",
				"url":      "https://api.github.com/issues/1",
				"html_url": "https://github.com/issues/1",
				"draft":    false,
			},
			map[string]any{
				"title":    "feature request",
				"url":      "https://api.github.com/issues/2",
				"html_url": "https://github.com/issues/2",
				"draft":    false,
			},
			map[string]any{
				"title":    "docs update",
				"url":      "https://api.github.com/issues/3",
				"html_url": "https://github.com/issues/3",
				"draft":    false,
			},
		},
		"totalCount": 100,
		"pageInfo": map[string]any{
			"hasNextPage": true,
			"endCursor":   "abc123",
		},
	}

	raw, err := MarshalItems(data)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(raw, &result)
	require.NoError(t, err)

	assert.NotNil(t, result["totalCount"])
	assert.NotNil(t, result["pageInfo"])

	issues, ok := result["issues"].([]any)
	require.True(t, ok)
	require.Len(t, issues, 3)

	for _, issue := range issues {
		m := issue.(map[string]any)
		assert.NotEmpty(t, m["title"])
		assert.NotEmpty(t, m["html_url"])
		assert.Nil(t, m["url"])
		assert.Equal(t, false, m["draft"])
	}
}
