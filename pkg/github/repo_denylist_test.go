package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRepoDenylist_IsDenied(t *testing.T) {
	tests := []struct {
		name     string
		entries  []string
		owner    string
		repo     string
		expected bool
	}{
		{
			name:     "exact match is denied",
			entries:  []string{"squareup/infosec-minesweeper"},
			owner:    "squareup",
			repo:     "infosec-minesweeper",
			expected: true,
		},
		{
			name:     "non-denied repo is allowed",
			entries:  []string{"squareup/infosec-minesweeper"},
			owner:    "squareup",
			repo:     "goosed-slackbot",
			expected: false,
		},
		{
			name:     "org wildcard denies any repo under org",
			entries:  []string{"squareup/*"},
			owner:    "squareup",
			repo:     "any-repo",
			expected: true,
		},
		{
			name:     "org wildcard does not affect other orgs",
			entries:  []string{"squareup/*"},
			owner:    "otherorg",
			repo:     "any-repo",
			expected: false,
		},
		{
			name:     "case-insensitive owner match",
			entries:  []string{"SquareUp/infosec-minesweeper"},
			owner:    "squareup",
			repo:     "infosec-minesweeper",
			expected: true,
		},
		{
			name:     "case-insensitive repo match",
			entries:  []string{"squareup/Infosec-Minesweeper"},
			owner:    "squareup",
			repo:     "infosec-minesweeper",
			expected: true,
		},
		{
			name:     "case-insensitive input",
			entries:  []string{"squareup/infosec-minesweeper"},
			owner:    "SquareUp",
			repo:     "Infosec-Minesweeper",
			expected: true,
		},
		{
			name:     "empty denylist allows everything",
			entries:  []string{},
			owner:    "squareup",
			repo:     "any-repo",
			expected: false,
		},
		{
			name:     "whitespace around entries is trimmed",
			entries:  []string{"  squareup/infosec-minesweeper  "},
			owner:    "squareup",
			repo:     "infosec-minesweeper",
			expected: true,
		},
		{
			name:     "comma-separated entries parsed correctly (multiple)",
			entries:  []string{"squareup/infosec-minesweeper", "squareup/cax-regulatory"},
			owner:    "squareup",
			repo:     "cax-regulatory",
			expected: true,
		},
		{
			name:     "invalid entry without slash is skipped",
			entries:  []string{"noslash", "squareup/valid-repo"},
			owner:    "squareup",
			repo:     "valid-repo",
			expected: true,
		},
		{
			name:     "empty string entries are skipped",
			entries:  []string{"", "squareup/infosec-minesweeper"},
			owner:    "squareup",
			repo:     "infosec-minesweeper",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewRepoDenylist(tc.entries)
			assert.Equal(t, tc.expected, d.IsDenied(tc.owner, tc.repo))
		})
	}
}

func TestRepoDenylist_NilSafe(t *testing.T) {
	var d *RepoDenylist
	assert.False(t, d.IsDenied("squareup", "any-repo"))
	assert.False(t, d.IsOrgDenied("squareup"))
	assert.True(t, d.IsEmpty())
}

func TestRepoDenylist_IsEmpty(t *testing.T) {
	assert.True(t, NewRepoDenylist(nil).IsEmpty())
	assert.True(t, NewRepoDenylist([]string{}).IsEmpty())
	assert.True(t, NewRepoDenylist([]string{"", "  "}).IsEmpty())
	assert.False(t, NewRepoDenylist([]string{"squareup/repo"}).IsEmpty())
	assert.False(t, NewRepoDenylist([]string{"squareup/*"}).IsEmpty())
}

func TestRepoDenylist_IsOrgDenied(t *testing.T) {
	d := NewRepoDenylist([]string{"squareup/*", "afterpaytouch/specific-repo"})

	// Org wildcard entry
	assert.True(t, d.IsOrgDenied("squareup"))
	assert.True(t, d.IsOrgDenied("SquareUp")) // case-insensitive

	// Exact entry does not make the whole org denied
	assert.False(t, d.IsOrgDenied("afterpaytouch"))

	// Unrelated org
	assert.False(t, d.IsOrgDenied("someotherorg"))
}
