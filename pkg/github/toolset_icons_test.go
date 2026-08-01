package github

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllToolsetIconsExist 验证s that every 工具集 with 一个Icon field
// references 一个icon that actually exists 在embedded octicons.
// 此prevents broken icon references from being merged.
func TestAllToolsetIconsExist(t *testing.T) {
	// Get 所有available 工具集s 来自inventory
	inv, err := NewInventory(stubTranslator).Build()
	require.NoError(t, err)
	toolsets := inv.AvailableToolsets()

	// Also test remote-仅工具集s
	remoteToolsets := RemoteOnlyToolsets()

	// Combine both 列出s
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
			// Check that icons 返回 valid 数据 URIs (不空)
			icons := octicons.Icons(ts.icon)
			require.NotNil(t, icons, "toolset %s references icon %q which does not exist", ts.name, ts.icon)
			assert.Len(t, icons, 2, "expected light and dark icon variants for toolset %s", ts.name)

			// Verify both variants have valid 数据 URIs
			for _, icon := range icons {
				assert.NotEmpty(t, icon.Source, "icon source should not be empty for toolset %s", ts.name)
				assert.Contains(t, icon.Source, "data:image/png;base64,",
					"icon %s for toolset %s should be a valid data URI", ts.icon, ts.name)
			}
		})
	}
}

// TestToolsetMeta数据HasIcons 确保所有 工具集s have icons defined.
// 此is 一个policy test - if you want to allow 工具集s without icons,
// you can remove 或modify this test.
func TestToolsetMetadataHasIcons(t *testing.T) {
	// 这些工具集s are expected to NOT have icons (internal/special purpose)
	exceptionsWithoutIcons := map[string]bool{
		"all":     true, // Meta-工具集
		"default": true, // Meta-工具集
	}

	inv, err := NewInventory(stubTranslator).Build()
	require.NoError(t, err)
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
