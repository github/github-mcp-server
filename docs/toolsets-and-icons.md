# Toolsets and Icons

This document explains how to work with toolsets and icons in the GitHub MCP Server.

## Toolset Overview

Toolsets are logical groupings of related tools. Each toolset has metadata defined in `pkg/github/tools.go`:

```go
ToolsetMetadataRepos = inventory.ToolsetMetadata{
    ID:          "repos",
    Description: "GitHub Repository related tools",
    Default:     true,
    Icon:        "repo",
}
```

### Toolset Fields

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `ToolsetID` | Unique identifier used in URLs and CLI flags (e.g., `repos`, `issues`) |
| `Description` | `string` | Human-readable description shown in documentation |
| `Default` | `bool` | Whether this toolset is enabled by default |
| `Icon` | `string` | Octicon name for visual representation in MCP clients |

## Adding Icons to Toolsets

Icons help users quickly identify toolsets in MCP-compatible clients. We use [Primer Octicons](https://primer.style/foundations/icons) for all icons.

### Step 1: Choose an Octicon

Browse the [Octicon gallery](https://primer.style/foundations/icons) and select an appropriate icon. Use the base name without size suffix (e.g., `repo` not `repo-16`).

### Step 2: Add Icon to Required Icons List

Icons are defined in `pkg/octicons/required_icons.txt`, which is the single source of truth for which icons should be embedded:

```
# Required icons for the GitHub MCP Server
# Add new icons below (one per line)
repo
issue-opened
git-pull-request
your-new-icon  # Add your icon here
```

### Step 3: Fetch the Icon Files

Run the fetch-icons script to download and convert the icon:

```bash
# Fetch a specific icon
script/fetch-icons your-new-icon

# Or fetch all required icons
script/fetch-icons
```

This script:
- Downloads the 24px SVG from [Primer Octicons](https://github.com/primer/octicons)
- Converts to PNG with light theme (dark icons for light backgrounds)
- Converts to PNG with dark theme (white icons for dark backgrounds)
- Saves both variants to `pkg/octicons/icons/`

**Requirements:** The script requires `rsvg-convert`:
- Ubuntu/Debian: `sudo apt-get install librsvg2-bin`
- macOS: `brew install librsvg`

### Step 4: Update the Toolset Metadata

Add or update the `Icon` field in the toolset definition:

```go
// In pkg/github/tools.go
ToolsetMetadataRepos = inventory.ToolsetMetadata{
    ID:          "repos",
    Description: "GitHub Repository related tools",
    Default:     true,
    Icon:        "repo",  // Add this line
}
```

### Step 5: Regenerate Documentation

Run the documentation generator to update all markdown files:

```bash
go run ./cmd/github-mcp-server generate-docs
```

This updates icons in:
- `README.md` - Toolsets table and tool section headers
- `docs/remote-server.md` - Remote toolsets table

## Remote-Only Toolsets

Some toolsets are only available in the remote GitHub MCP Server (hosted at `api.githubcopilot.com`). These are defined in `pkg/github/tools.go` with their icons, but are not registered with the local server:

```go
// Remote-only toolsets
ToolsetMetadataCopilot = inventory.ToolsetMetadata{
    ID:          "copilot",
    Description: "Copilot related tools",
    Icon:        "copilot",
}
```

The `RemoteOnlyToolsets()` function returns the list of these toolsets for documentation generation.

To add a new remote-only toolset:

1. Add the metadata definition in `pkg/github/tools.go`
2. Add it to the slice returned by `RemoteOnlyToolsets()`
3. Regenerate documentation

## Tool Icon Inheritance

Individual tools inherit icons from their parent toolset. When a tool is registered with a toolset, its icons are automatically set:

```go
// In pkg/inventory/server_tool.go
toolCopy.Icons = tool.Toolset.Icons()
```

This means you only need to set the icon once on the toolset, and all tools in that toolset will display the same icon.

## How Icons Work in MCP

The MCP protocol supports tool icons via the `icons` field. We provide icons in two formats:

1. **Data URIs** - Base64-encoded PNG images embedded in the tool definition
2. **Light/Dark variants** - Both theme variants are provided for proper display

The `octicons.Icons()` function generates the MCP-compatible icon objects:

```go
// Returns []mcp.Icon with both light and dark variants
icons := octicons.Icons("repo")
```

## Existing Toolset Icons

| Toolset | Octicon Name |
|---------|--------------|
| Context | `person` |
| Repositories | `repo` |
| Issues | `issue-opened` |
| Pull Requests | `git-pull-request` |
| Git | `git-branch` |
| Users | `people` |
| Organizations | `organization` |
| Actions | `workflow` |
| Code Security | `codescan` |
| Secret Protection | `shield-lock` |
| Dependabot | `dependabot` |
| Discussions | `comment-discussion` |
| Gists | `logo-gist` |
| Security Advisories | `shield` |
| Projects | `project` |
| Labels | `tag` |
| Stargazers | `star` |
| Notifications | `bell` |
| Dynamic | `tools` |
| Copilot | `copilot` |
| Support Search | `book` |

## Troubleshooting

### Icons not appearing in documentation

1. Ensure PNG files exist in `pkg/octicons/icons/` with `-light.png` and `-dark.png` suffixes
2. Run `go run ./cmd/github-mcp-server generate-docs` to regenerate
3. Check that the `Icon` field is set on the toolset metadata

### Icons not appearing in MCP clients

1. Verify the client supports MCP tool icons
2. Check that the octicons package is properly generating base64 data URIs
3. Ensure the icon name matches a file in `pkg/octicons/icons/`

## CI Validation

The following tests run in CI to catch icon issues early:

### `pkg/octicons.TestEmbeddedIconsExist`

Verifies that all icons listed in `pkg/octicons/required_icons.txt` have corresponding PNG files embedded.

### `pkg/github.TestAllToolsetIconsExist`

Verifies that all toolset `Icon` fields reference icons that are properly embedded.

### `pkg/github.TestToolsetMetadataHasIcons`

Ensures all toolsets have an `Icon` field set.

If any of these tests fail:
1. Add the missing icon to `pkg/octicons/required_icons.txt`
2. Run `script/fetch-icons` to download the icon
3. Commit the new icon files
