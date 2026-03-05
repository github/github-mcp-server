package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_extractAllReposFromQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []repoQueryInfo
	}{
		{
			name:     "single repo: qualifier",
			query:    "repo:squareup/goosed-slackbot language:go",
			expected: []repoQueryInfo{{owner: "squareup", repo: "goosed-slackbot"}},
		},
		{
			name:  "multiple repo: qualifiers",
			query: "repo:squareup/goosed repo:tidal/music-player",
			expected: []repoQueryInfo{
				{owner: "squareup", repo: "goosed"},
				{owner: "tidal", repo: "music-player"},
			},
		},
		{
			name:     "no repo qualifier",
			query:    "golang test org:squareup",
			expected: []repoQueryInfo{},
		},
		{
			name:     "empty query",
			query:    "",
			expected: []repoQueryInfo{},
		},
		{
			name:     "mixed-case REPO: qualifier",
			query:    "REPO:squareup/goosed-slackbot",
			expected: []repoQueryInfo{{owner: "squareup", repo: "goosed-slackbot"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractAllReposFromQuery(tc.query)
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
		{
			name:     "mixed-case ORG: qualifier",
			query:    "ORG:squareup language:go",
			expected: "squareup",
		},
		{
			name:     "mixed-case User: qualifier",
			query:    "User:octocat repos:>10",
			expected: "octocat",
		},
		{
			name:     "uppercase USER: qualifier",
			query:    "USER:octocat",
			expected: "octocat",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractOrgFromQuery(tc.query)
			assert.Equal(t, tc.expected, result)
		})
	}
}
