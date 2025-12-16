# Deprecated Tool Aliases

This document tracks tool renames in the GitHub MCP Server. When tools are renamed, the old names are preserved as aliases for backward compatibility. Using a deprecated alias will still work, but clients should migrate to the new canonical name.

## Current Deprecations

<!-- START AUTOMATED ALIASES -->
| Old Name | New Name |
|----------|----------|
| *(none currently)* | |
<!-- END AUTOMATED ALIASES -->

## How It Works

When a tool is renamed:

1. The old name is added to `DeprecatedToolAliases` in [pkg/github/deprecated_tool_aliases.go](../pkg/github/deprecated_tool_aliases.go)
2. Clients using the old name will receive the new tool
3. A deprecation notice is logged when the alias is used

## For Developers

To deprecate a tool name when renaming:

```go
var DeprecatedToolAliases = map[string]string{
    "old_tool_name": "new_tool_name",
}
```

The alias resolution happens at server startup, ensuring backward compatibility for existing client configurations.
