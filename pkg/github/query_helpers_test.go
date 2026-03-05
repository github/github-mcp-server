package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_extractRepoFromQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected repoQueryInfo
	}{
		{
			name:     "repo:owner/repo pattern",
			query:    "repo:squareup/goosed-slackbot language:go",
			expected: repoQueryInfo{owner: "squareup", repo: "goosed-slackbot"},
		},
		{
			name:     "bare owner/repo pattern",
			query:    "squareup/goosed-slackbot",
			expected: repoQueryInfo{owner: "squareup", repo: "goosed-slackbot"},
		},
		{
			name:     "org qualifier only",
			query:    "goosed org:squareup",
			expected: repoQueryInfo{},
		},
		{
			name:     "no repo pattern",
			query:    "golang test",
			expected: repoQueryInfo{},
		},
		{
			name:     "empty query",
			query:    "",
			expected: repoQueryInfo{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractRepoFromQuery(tc.query)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func Test_extractOrgFromQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "org qualifier",
			query:    "goosed org:squareup",
			expected: "squareup",
		},
		{
			name:     "org qualifier at start",
			query:    "org:tidal-engineering language:go",
			expected: "tidal-engineering",
		},
		{
			name:     "user qualifier",
			query:    "user:octocat repos:>10",
			expected: "octocat",
		},
		{
			name:     "no org qualifier",
			query:    "golang test",
			expected: "",
		},
		{
			name:     "repo qualifier without org",
			query:    "repo:squareup/goosed-slackbot",
			expected: "",
		},
		{
			name:     "both org and repo qualifiers",
			query:    "function main repo:squareup/goosed org:squareup",
			expected: "squareup",
		},
		{
			name:     "empty query",
			query:    "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractOrgFromQuery(tc.query)
			assert.Equal(t, tc.expected, result)
		})
	}
}
