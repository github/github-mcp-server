package octicons

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestDataURI(t *testing.T) {
	tests := []struct {
		name        string
		icon        string
		theme       Theme
		wantDataURI bool
		wantEmpty   bool
	}{
		{
			name:        "light theme icon returns data URI",
			icon:        "repo",
			theme:       ThemeLight,
			wantDataURI: true,
			wantEmpty:   false,
		},
		{
			name:        "dark theme icon returns data URI",
			icon:        "repo",
			theme:       ThemeDark,
			wantDataURI: true,
			wantEmpty:   false,
		},
		{
			name:        "non-embedded icon returns empty string",
			icon:        "nonexistent-icon",
			theme:       ThemeLight,
			wantDataURI: false,
			wantEmpty:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DataURI(tc.icon, tc.theme)
			if tc.wantDataURI {
				assert.True(t, strings.HasPrefix(result, "data:image/png;base64,"), "expected data URI prefix")
				assert.NotContains(t, result, "https://")
			}
			if tc.wantEmpty {
				assert.Empty(t, result, "expected empty string for non-embedded icon")
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
			name:      "valid embedded icon returns light and dark variants",
			icon:      "repo",
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

			// Verify first icon is light theme
			assert.Equal(t, DataURI(tc.icon, ThemeLight), result[0].Source)
			assert.Equal(t, "image/png", result[0].MIMEType)
			assert.Empty(t, result[0].Sizes) // Sizes field omitted for backward compatibility
			assert.Equal(t, mcp.IconThemeLight, result[0].Theme)

			// Verify second icon is dark theme
			assert.Equal(t, DataURI(tc.icon, ThemeDark), result[1].Source)
			assert.Equal(t, "image/png", result[1].MIMEType)
			assert.Empty(t, result[1].Sizes) // Sizes field omitted for backward compatibility
			assert.Equal(t, mcp.IconThemeDark, result[1].Theme)
		})
	}
}

func TestThemeConstants(t *testing.T) {
	assert.Equal(t, Theme("light"), ThemeLight)
	assert.Equal(t, Theme("dark"), ThemeDark)
}

func TestEmbeddedIconsExist(t *testing.T) {
	// Test that all required icons from required_icons.txt are properly embedded
	// This is the single source of truth for which icons should be available
	expectedIcons := RequiredIcons()
	for _, icon := range expectedIcons {
		t.Run(icon, func(t *testing.T) {
			lightURI := DataURI(icon, ThemeLight)
			darkURI := DataURI(icon, ThemeDark)
			assert.True(t, strings.HasPrefix(lightURI, "data:image/png;base64,"), "light theme icon %s should be embedded", icon)
			assert.True(t, strings.HasPrefix(darkURI, "data:image/png;base64,"), "dark theme icon %s should be embedded", icon)
		})
	}
}
