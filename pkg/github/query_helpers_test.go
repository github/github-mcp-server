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

func Test_extractAllOrgsFromQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "single org qualifier",
			query:    "goosed org:squareup",
			expected: []string{"squareup"},
		},
		{
			name:     "single user qualifier",
			query:    "user:octocat repos:>10",
			expected: []string{"octocat"},
		},
		{
			name:     "multiple qualifiers — catches bypass attempts",
			query:    "user:allowed-user org:denied-org",
			expected: []string{"allowed-user", "denied-org"},
		},
		{
			name:     "no org qualifier",
			query:    "golang test",
			expected: []string{},
		},
		{
			name:     "empty query",
			query:    "",
			expected: []string{},
		},
		{
			name:     "mixed-case ORG: qualifier",
			query:    "ORG:squareup language:go",
			expected: []string{"squareup"},
		},
		{
			name:     "mixed-case User: qualifier",
			query:    "User:octocat repos:>10",
			expected: []string{"octocat"},
		},
		{
			name:     "both org and repo qualifiers",
			query:    "function main repo:squareup/goosed org:squareup",
			expected: []string{"squareup"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractAllOrgsFromQuery(tc.query)
			assert.Equal(t, tc.expected, result)
		})
	}
}
