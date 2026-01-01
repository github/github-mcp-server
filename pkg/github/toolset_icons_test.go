package github

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllToolsetIconsExist validates that every toolset with an Icon field
// references an icon that actually exists in the embedded octicons.
// This prevents broken icon references from being merged.
func TestAllToolsetIconsExist(t *testing.T) {
	// Get all available toolsets from the inventory
	inv := NewInventory(stubTranslator).Build()
	toolsets := inv.AvailableToolsets()

	// Also test remote-only toolsets
	remoteToolsets := RemoteOnlyToolsets()

	// Combine both lists
	allToolsets := make([]struct {
		name string
		icon string
	}, 0)

	for _, ts := range toolsets {
		if ts.Icon != "" {
			allToolsets = append(allToolsets, struct {
				name string
				icon string
			}{name: string(ts.ID), icon: ts.Icon})
		}
	}

	for _, ts := range remoteToolsets {
		if ts.Icon != "" {
			allToolsets = append(allToolsets, struct {
				name string
				icon string
			}{name: string(ts.ID), icon: ts.Icon})
		}
	}

	require.NotEmpty(t, allToolsets, "expected at least one toolset with an icon")

	for _, ts := range allToolsets {
		t.Run(ts.name, func(t *testing.T) {
			// Check that icons return valid data URIs (not empty)
			icons := octicons.Icons(ts.icon)
			require.NotNil(t, icons, "toolset %s references icon %q which does not exist", ts.name, ts.icon)
			assert.Len(t, icons, 2, "expected light and dark icon variants for toolset %s", ts.name)

			// Verify both variants have valid data URIs
			for _, icon := range icons {
				assert.NotEmpty(t, icon.Source, "icon source should not be empty for toolset %s", ts.name)
				assert.Contains(t, icon.Source, "data:image/png;base64,",
					"icon %s for toolset %s should be a valid data URI", ts.icon, ts.name)
			}
		})
	}
}

// TestToolsetMetadataHasIcons ensures all toolsets have icons defined.
// This is a policy test - if you want to allow toolsets without icons,
// you can remove or modify this test.
func TestToolsetMetadataHasIcons(t *testing.T) {
	// These toolsets are expected to NOT have icons (internal/special purpose)
	exceptionsWithoutIcons := map[string]bool{
		"all":     true, // Meta-toolset
		"default": true, // Meta-toolset
	}

	inv := NewInventory(stubTranslator).Build()
	toolsets := inv.AvailableToolsets()

	for _, ts := range toolsets {
		if exceptionsWithoutIcons[string(ts.ID)] {
			continue
		}
		t.Run(string(ts.ID), func(t *testing.T) {
			assert.NotEmpty(t, ts.Icon, "toolset %s should have an icon defined", ts.ID)
		})
	}
}
