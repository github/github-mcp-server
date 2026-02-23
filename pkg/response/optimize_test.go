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
			result := flattenTo(tc.input, defaultMaxDepth)
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

func TestFilterByFillRate(t *testing.T) {
	cfg := OptimizeListConfig{}

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

	result := filterByFillRate(items, 0.1, cfg)

	for _, item := range result {
		assert.Contains(t, item, "title")
		assert.Contains(t, item, "body")
		assert.NotContains(t, item, "milestone")
	}
}

func TestFilterByFillRate_PreservesFields(t *testing.T) {
	cfg := OptimizeListConfig{
		preservedFields: map[string]bool{"html_url": true},
	}

	items := make([]map[string]any, 10)
	for i := range items {
		items[i] = map[string]any{"title": "x"}
	}
	items[0]["html_url"] = "https://github.com/repo/1"

	result := filterByFillRate(items, 0.1, cfg)
	assert.Contains(t, result[0], "html_url")
}

func TestOptimizeList_AllStrategies(t *testing.T) {
	items := []map[string]any{
		{
			"title":      "Fix bug",
			"body":       "line1\n\nline2",
			"url":        "https://api.github.com/repos/1",
			"html_url":   "https://github.com/repo/1",
			"avatar_url": "https://avatars.githubusercontent.com/1",
			"draft":      false,
			"merged_at":  nil,
			"labels":     []any{"bug", "fix"},
			"user": map[string]any{
				"login":      "user",
				"avatar_url": "https://avatars.githubusercontent.com/1",
			},
		},
	}

	raw, err := OptimizeList(items,
		WithPreservedFields("html_url", "draft"),
	)
	require.NoError(t, err)

	var result []map[string]any
	err = json.Unmarshal(raw, &result)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "Fix bug", result[0]["title"])
	assert.Equal(t, "line1 line2", result[0]["body"])
	assert.Equal(t, "https://github.com/repo/1", result[0]["html_url"])
	assert.Equal(t, false, result[0]["draft"])
	assert.Equal(t, []any{"bug", "fix"}, result[0]["labels"])
	assert.Equal(t, "user", result[0]["user.login"])
	assert.Nil(t, result[0]["url"])
	assert.Nil(t, result[0]["avatar_url"])
	assert.Nil(t, result[0]["merged_at"])
}

func TestOptimizeList_NilInput(t *testing.T) {
	raw, err := OptimizeList[map[string]any](nil)
	require.NoError(t, err)
	assert.Equal(t, "null", string(raw))
}

func TestOptimizeList_SkipsFillRateBelowMinRows(t *testing.T) {
	items := []map[string]any{
		{"title": "a", "rare": "x"},
		{"title": "b"},
	}

	raw, err := OptimizeList(items)
	require.NoError(t, err)

	var result []map[string]any
	err = json.Unmarshal(raw, &result)
	require.NoError(t, err)

	assert.Equal(t, "x", result[0]["rare"])
}

func TestPreservedFields(t *testing.T) {
	t.Run("keeps preserved URL keys, strips non-preserved", func(t *testing.T) {
		cfg := OptimizeListConfig{
			preservedFields: map[string]bool{
				"html_url":  true,
				"clone_url": true,
			},
		}

		result := optimizeItem(map[string]any{
			"html_url":       "https://github.com/repo/1",
			"clone_url":      "https://github.com/repo/1.git",
			"avatar_url":     "https://avatars.githubusercontent.com/1",
			"user.html_url":  "https://github.com/user",
			"user.clone_url": "https://github.com/user.git",
		}, cfg)

		assert.Contains(t, result, "html_url")
		assert.Contains(t, result, "clone_url")
		assert.NotContains(t, result, "avatar_url")
		assert.NotContains(t, result, "user.html_url")
		assert.NotContains(t, result, "user.clone_url")
	})

	t.Run("protects zero values", func(t *testing.T) {
		cfg := OptimizeListConfig{
			preservedFields: map[string]bool{"draft": true},
		}

		result := optimizeItem(map[string]any{
			"draft": false,
			"body":  "",
		}, cfg)

		assert.Contains(t, result, "draft")
		assert.NotContains(t, result, "body")
	})
}
