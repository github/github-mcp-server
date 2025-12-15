package octicons

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDataURI(t *testing.T) {
	tests := []struct {
		name         string
		icon         string
		size         Size
		wantDataURI  bool
		wantFallback bool
	}{
		{
			name:         "embedded icon returns data URI",
			icon:         "repo",
			size:         SizeSM,
			wantDataURI:  true,
			wantFallback: false,
		},
		{
			name:         "embedded icon large returns data URI",
			icon:         "repo",
			size:         SizeLG,
			wantDataURI:  true,
			wantFallback: false,
		},
		{
			name:         "non-embedded icon falls back to CDN",
			icon:         "nonexistent-icon",
			size:         SizeSM,
			wantDataURI:  false,
			wantFallback: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DataURI(tc.icon, tc.size)
			if tc.wantDataURI {
				assert.True(t, strings.HasPrefix(result, "data:image/svg+xml;base64,"), "expected data URI prefix")
				// Verify it's valid base64 by checking it doesn't contain the fallback URL
				assert.NotContains(t, result, "https://")
			}
			if tc.wantFallback {
				assert.True(t, strings.HasPrefix(result, "https://"), "expected fallback URL")
			}
		})
	}
}

func TestIcons(t *testing.T) {
	tests := []struct {
		name      string
		icon      string
		wantNil   bool
		wantCount int
	}{
		{
			name:      "valid embedded icon returns two sizes",
			icon:      "repo",
			wantNil:   false,
			wantCount: 2,
		},
		{
			name:      "non-embedded icon still returns two sizes (fallback)",
			icon:      "copilot",
			wantNil:   false,
			wantCount: 2,
		},
		{
			name:      "empty name returns nil",
			icon:      "",
			wantNil:   true,
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Icons(tc.icon)
			if tc.wantNil {
				assert.Nil(t, result)
				return
			}
			assert.NotNil(t, result)
			assert.Len(t, result, tc.wantCount)

			// Verify first icon is 16x16
			assert.Equal(t, DataURI(tc.icon, SizeSM), result[0].Source)
			assert.Equal(t, "image/svg+xml", result[0].MIMEType)
			assert.Equal(t, []string{"16x16"}, result[0].Sizes)

			// Verify second icon is 24x24
			assert.Equal(t, DataURI(tc.icon, SizeLG), result[1].Source)
			assert.Equal(t, "image/svg+xml", result[1].MIMEType)
			assert.Equal(t, []string{"24x24"}, result[1].Sizes)
		})
	}
}

func TestSizeConstants(t *testing.T) {
	// Verify size constants have expected values
	assert.Equal(t, Size(16), SizeSM)
	assert.Equal(t, Size(24), SizeLG)
}

func TestEmbeddedIconsExist(t *testing.T) {
	// Test that all icons used by toolsets are properly embedded
	expectedIcons := []string{
		"apps", "beaker", "bell", "check-circle", "codescan",
		"comment-discussion", "dependabot", "git-branch", "git-pull-request",
		"issue-opened", "logo-gist", "organization", "people", "person",
		"project", "repo", "shield", "shield-lock", "star", "tag", "tools", "workflow",
	}

	for _, icon := range expectedIcons {
		t.Run(icon, func(t *testing.T) {
			uri16 := DataURI(icon, SizeSM)
			uri24 := DataURI(icon, SizeLG)
			assert.True(t, strings.HasPrefix(uri16, "data:"), "16px icon %s should be embedded", icon)
			assert.True(t, strings.HasPrefix(uri24, "data:"), "24px icon %s should be embedded", icon)
		})
	}
}
