package octicons

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURL(t *testing.T) {
	tests := []struct {
		name     string
		icon     string
		size     Size
		expected string
	}{
		{
			name:     "small size",
			icon:     "repo",
			size:     SizeSM,
			expected: "https://raw.githubusercontent.com/primer/octicons/main/icons/repo-16.svg",
		},
		{
			name:     "large size",
			icon:     "repo",
			size:     SizeLG,
			expected: "https://raw.githubusercontent.com/primer/octicons/main/icons/repo-24.svg",
		},
		{
			name:     "copilot icon small",
			icon:     "copilot",
			size:     SizeSM,
			expected: "https://raw.githubusercontent.com/primer/octicons/main/icons/copilot-16.svg",
		},
		{
			name:     "copilot icon large",
			icon:     "copilot",
			size:     SizeLG,
			expected: "https://raw.githubusercontent.com/primer/octicons/main/icons/copilot-24.svg",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := URL(tc.icon, tc.size)
			assert.Equal(t, tc.expected, result)
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
			name:      "valid icon returns two sizes",
			icon:      "repo",
			wantNil:   false,
			wantCount: 2,
		},
		{
			name:      "copilot icon returns two sizes",
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
			assert.Equal(t, URL(tc.icon, SizeSM), result[0].Source)
			assert.Equal(t, "image/svg+xml", result[0].MIMEType)
			assert.Equal(t, []string{"16x16"}, result[0].Sizes)

			// Verify second icon is 24x24
			assert.Equal(t, URL(tc.icon, SizeLG), result[1].Source)
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
