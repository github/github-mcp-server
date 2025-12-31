// Package octicons provides helpers for working with GitHub Octicon icons.
// See https://primer.style/foundations/icons for available icons.
package octicons

import (
	"bufio"
	"embed"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed icons/*.png
var iconsFS embed.FS

//go:embed required_icons.txt
var requiredIconsTxt string

// RequiredIcons returns the list of icon names from required_icons.txt.
// This is the single source of truth for which icons should be embedded.
func RequiredIcons() []string {
	var icons []string
	scanner := bufio.NewScanner(strings.NewReader(requiredIconsTxt))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		icons = append(icons, line)
	}
	return icons
}

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
// Icons are embedded as 24x24 PNG data URIs for offline use and faster loading.
// The name should be the base octicon name without size suffix (e.g., "repo" not "repo-16").
// See https://primer.style/foundations/icons for available icons.
//
// Note: The Sizes field is omitted for backward compatibility with older MCP clients
// that expect it to be a string rather than an array per the 2025-11-25 MCP spec.
func Icons(name string) []mcp.Icon {
	if name == "" {
		return nil
	}
	return []mcp.Icon{
		{
			Source:   DataURI(name, ThemeLight),
			MIMEType: "image/png",
			Theme:    mcp.IconThemeLight,
		},
		{
			Source:   DataURI(name, ThemeDark),
			MIMEType: "image/png",
			Theme:    mcp.IconThemeDark,
		},
	}
}
