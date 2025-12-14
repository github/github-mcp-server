// Package octicons provides helpers for working with GitHub Octicon icons.
// See https://primer.style/foundations/icons for available icons.
package octicons

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Size represents the size of an Octicon icon.
type Size int

const (
	// SizeSM is the small (16x16) icon size.
	SizeSM Size = 16
	// SizeLG is the large (24x24) icon size.
	SizeLG Size = 24
)

// URL returns the CDN URL for a GitHub Octicon SVG.
func URL(name string, size Size) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/primer/octicons/main/icons/%s-%d.svg", name, size)
}

// Icons returns MCP Icon objects for the given octicon name in both 16x16 and 24x24 sizes.
// Use this to set custom icons on individual tools that should override their toolset's default icon.
// The name should be the base octicon name without size suffix (e.g., "copilot" not "copilot-16").
// See https://primer.style/foundations/icons for available icons.
func Icons(name string) []mcp.Icon {
	if name == "" {
		return nil
	}
	return []mcp.Icon{
		{
			Source:   URL(name, SizeSM),
			MIMEType: "image/svg+xml",
			Sizes:    []string{"16x16"},
		},
		{
			Source:   URL(name, SizeLG),
			MIMEType: "image/svg+xml",
			Sizes:    []string{"24x24"},
		},
	}
}
