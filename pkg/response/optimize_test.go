package response

import (
	"encoding/json"
	"fmt"
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

	t.Run("silently drops nested maps at maxDepth boundary", func(t *testing.T) {
		// With defaultMaxDepth=2, depth starts at 1.
		// "user" is recursed into at depth=2, but "org" inside it
		// is a map at depth=2 where depth < maxDepth is false,
		// and the else-if !ok branch also doesn't fire because ok=true.
		// Result: "user.org" and its contents are silently lost.
		input := map[string]any{
			"title": "issue",
			"user": map[string]any{
				"login": "alice",
				"org": map[string]any{
					"name": "acme",
				},
			},
		}
		result := flattenTo(input, defaultMaxDepth)

		assert.Equal(t, "issue", result["title"])
		assert.Equal(t, "alice", result["user.login"])
		// BUG: org data is silently dropped — neither "user.org" nor
		// "user.org.name" appear in the result.
		assert.Nil(t, result["user.org"], "nested map at maxDepth is silently dropped")
		assert.Nil(t, result["user.org.name"], "nested map contents at maxDepth are lost")
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

func TestOptimizeList_NilInput_AsRawMessage(t *testing.T) {
	// This mirrors how list_issues embeds OptimizeList output as json.RawMessage.
	// When the input slice is nil, OptimizeList returns the bytes "null",
	// which produces {"issues":null,...} instead of {"issues":[],...}.
	// Consumers expecting an array will break.
	optimized, err := OptimizeList[map[string]any](nil)
	require.NoError(t, err)

	wrapper := map[string]any{
		"issues":     json.RawMessage(optimized),
		"totalCount": 0,
	}

	out, err := json.Marshal(wrapper)
	require.NoError(t, err)

	// Parse back and check what "issues" became
	var parsed map[string]json.RawMessage
	err = json.Unmarshal(out, &parsed)
	require.NoError(t, err)

	// BUG: "issues" is JSON null, not an empty array
	assert.Equal(t, "null", string(parsed["issues"]),
		"nil input produces JSON null instead of empty array when embedded as RawMessage")

	// This is what a consumer trying to decode an array would see:
	var issues []map[string]any
	err = json.Unmarshal(parsed["issues"], &issues)
	require.NoError(t, err) // unmarshal succeeds but...
	assert.Nil(t, issues, "decoded slice is nil, not empty — may cause nil-pointer issues in consumers")
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

func TestWhitespaceNormalization_DestroysCodeBlocks(t *testing.T) {
	// PR/issue bodies often contain markdown with code blocks, bullet lists,
	// and intentional line breaks. The whitespace normalization strategy
	// collapses all of this into a single line.
	body := "## Steps to reproduce\n\n" +
		"1. Run the following:\n\n" +
		"```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n\n" +
		"2. Observe the error:\n\n" +
		"```\npanic: runtime error\n  goroutine 1\n```"

	items := []map[string]any{
		{
			"title": "Bug report",
			"body":  body,
		},
	}

	raw, err := OptimizeList(items)
	require.NoError(t, err)

	var result []map[string]any
	err = json.Unmarshal(raw, &result)
	require.NoError(t, err)
	require.Len(t, result, 1)

	optimized := result[0]["body"].(string)

	assert.Equal(t,
		"## Steps to reproduce 1. Run the following: ```go func main() { fmt.Println(\"hello\") } ``` 2. Observe the error: ``` panic: runtime error goroutine 1 ```",
		optimized,
		"code blocks and markdown structure are flattened into unreadable text",
	)
}

func TestFillRateAfterZeroRemoval_DropsLegitimateValues(t *testing.T) {
	// This simulates list_branches with 10 branches where 1 is protected.
	// Pipeline order: optimizeItem (strips protected:false) → filterByFillRate.
	// After optimizeItem, "protected" only appears on 1/10 items.
	// Fill rate = 1/10 = 0.1, minCount = int(0.1*10) = 1, and 1 > 1 is false.
	// So "protected: true" is removed from the one branch that had it.

	items := make([]map[string]any, 10)
	for i := range items {
		items[i] = map[string]any{
			"name":      fmt.Sprintf("branch-%d", i),
			"protected": false,
		}
	}
	// One branch is actually protected
	items[0]["protected"] = true

	raw, err := OptimizeList(items)
	require.NoError(t, err)

	var result []map[string]any
	err = json.Unmarshal(raw, &result)
	require.NoError(t, err)
	require.Len(t, result, 10)

	// BUG: The protected branch lost its "protected: true" field.
	// optimizeItem stripped "protected: false" from 9 items, then
	// filterByFillRate saw "protected" on only 1/10 and removed it.
	assert.Nil(t, result[0]["protected"],
		"protected:true is lost because zero-value removal deflated the fill rate")
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
