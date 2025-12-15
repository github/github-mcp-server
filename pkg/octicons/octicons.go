// Package octicons provides helpers for working with GitHub Octicon icons.
// See https://primer.style/foundations/icons for available icons.
package octicons

import (
	"embed"
	"encoding/base64"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed icons/*.svg
var iconsFS embed.FS

// Size represents the size of an Octicon icon.
type Size int

const (
	// SizeSM is the small (16x16) icon size.
	SizeSM Size = 16
	// SizeLG is the large (24x24) icon size.
	SizeLG Size = 24
)

// DataURI returns a data URI for the embedded Octicon SVG.
// If the icon is not found in the embedded filesystem, it falls back to a CDN URL.
func DataURI(name string, size Size) string {
	filename := fmt.Sprintf("icons/%s-%d.svg", name, size)
	data, err := iconsFS.ReadFile(filename)
	if err != nil {
		// Fall back to CDN URL if icon is not embedded
		return fmt.Sprintf("https://raw.githubusercontent.com/primer/octicons/main/icons/%s-%d.svg", name, size)
	}
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(data)
}

// Icons returns MCP Icon objects for the given octicon name in both 16x16 and 24x24 sizes.
// Icons are embedded as data URIs for offline use and faster loading.
// The name should be the base octicon name without size suffix (e.g., "repo" not "repo-16").
// See https://primer.style/foundations/icons for available icons.
func Icons(name string) []mcp.Icon {
	if name == "" {
		return nil
	}
	return []mcp.Icon{
		{
			Source:   DataURI(name, SizeSM),
			MIMEType: "image/svg+xml",
			Sizes:    []string{"16x16"},
		},
		{
			Source:   DataURI(name, SizeLG),
			MIMEType: "image/svg+xml",
			Sizes:    []string{"24x24"},
		},
	}
}
