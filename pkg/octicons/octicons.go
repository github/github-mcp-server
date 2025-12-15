// Package octicons provides helpers for working with GitHub Octicon icons.
// See https://primer.style/foundations/icons for available icons.
package octicons

import (
	"embed"
	"encoding/base64"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed icons/*.png
var iconsFS embed.FS

// Theme represents the color theme of an icon.
type Theme string

const (
	// ThemeLight is for light backgrounds (dark/black icons).
	ThemeLight Theme = "light"
	// ThemeDark is for dark backgrounds (light/white icons).
	ThemeDark Theme = "dark"
)

// DataURI returns a data URI for the embedded Octicon PNG.
// The theme parameter specifies which variant to use:
// - ThemeLight: dark icons for light backgrounds
// - ThemeDark: light icons for dark backgrounds
// If the icon is not found in the embedded filesystem, it returns an empty string.
func DataURI(name string, theme Theme) string {
	filename := fmt.Sprintf("icons/%s-%s.png", name, theme)
	data, err := iconsFS.ReadFile(filename)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

// Icons returns MCP Icon objects for the given octicon name in light and dark themes.
// Icons are embedded as 16x16 PNG data URIs for offline use and faster loading.
// The name should be the base octicon name without size suffix (e.g., "repo" not "repo-16").
// See https://primer.style/foundations/icons for available icons.
func Icons(name string) []mcp.Icon {
	if name == "" {
		return nil
	}
	return []mcp.Icon{
		{
			Source:   DataURI(name, ThemeLight),
			MIMEType: "image/png",
			Sizes:    []string{"16x16"},
			Theme:    string(ThemeLight),
		},
		{
			Source:   DataURI(name, ThemeDark),
			MIMEType: "image/png",
			Sizes:    []string{"16x16"},
			Theme:    string(ThemeDark),
		},
	}
}
