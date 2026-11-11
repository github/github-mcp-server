// Package octicons provides helpers for working with GitHub Octicon icons.
// See https://primer.style/foundations/icons for available icons.
package octicons

import (
	"bufio"
	_ "embed"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed required_icons.txt
var requiredIconsTxt string

//go:embed icons_data_uris.txt
var embeddedDataURIs string

type dataURIKey struct {
	name  string
	theme Theme
}

var dataURIs = loadDataURIs()

func loadDataURIs() map[dataURIKey]string {
	dataURIs := make(map[dataURIKey]string)
	for line := range strings.SplitSeq(strings.TrimSpace(embeddedDataURIs), "\n") {
		filename, dataURI, ok := strings.Cut(line, "\t")
		separator := strings.LastIndexByte(filename, '-')
		if !ok || separator <= 0 || !strings.HasPrefix(dataURI, "data:image/png;base64,") {
			panic(fmt.Sprintf("invalid embedded icon data URI entry %q", line))
		}
		theme := Theme(filename[separator+1:])
		if theme != ThemeLight && theme != ThemeDark {
			panic(fmt.Sprintf("invalid embedded icon theme %q", theme))
		}
		dataURIs[dataURIKey{
			name:  filename[:separator],
			theme: theme,
		}] = dataURI
	}
	return dataURIs
}

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
// If the icon is not found in the embedded icon set, it returns an empty string.
func DataURI(name string, theme Theme) string {
	return dataURIs[dataURIKey{name: name, theme: theme}]
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
