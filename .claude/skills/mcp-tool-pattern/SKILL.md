---
name: mcp-tool-pattern
description: Reference for adding or modifying MCP tools in pkg/github/. Triggers when editing any file under pkg/github/ that defines or registers a tool (returns inventory.ServerTool, calls NewTool(...), or AllTools()).
---

# MCP Tool Pattern (github-mcp-server)

You are editing tool code in `pkg/github/`. Follow CLAUDE.md and this
checklist.

## Canonical constructor

```go
func MyTool(t translations.TranslationHelperFunc) inventory.ServerTool {
    return NewTool(
        ToolsetMetadataRepos, // pick the right toolset from tools.go
        mcp.Tool{
            Name:        "my_tool",                       // snake_case
            Description: t("TOOL_MY_TOOL_DESCRIPTION", "..."),
            Annotations: &mcp.ToolAnnotations{
                Title:        t("TOOL_MY_TOOL_USER_TITLE", "..."),
                ReadOnlyHint: true,  // false for writes — be honest
            },
            InputSchema: &jsonschema.Schema{ /* ... */ },
        },
        []scopes.Scope{scopes.Repo},  // minimum OAuth scopes
        func(ctx context.Context, deps ToolDependencies,
             req *mcp.CallToolRequest, args map[string]any,
        ) (*mcp.CallToolResult, any, error) {
            owner, err := RequiredParam[string](args, "owner")
            if err != nil { return utils.NewToolResultError(err.Error()), nil, nil }

            client, err := deps.GetClient(ctx)
            if err != nil {
                return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
            }
            // ... api call ...
            return utils.NewToolResultText(body), nil, nil
        },
    )
}
```

## Must-do on every tool change

1. Register in `AllTools()` (`pkg/github/tools.go:158`).
2. Test in `pkg/github/<domain>_test.go` with `toolsnaps.Test(tool.Name, tool)`.
3. `UPDATE_TOOLSNAPS=true go test ./...` and commit the new
   `pkg/github/__toolsnaps__/<name>.snap`.
4. `script/generate-docs` and commit the README update.
5. Renaming an existing tool? Add an alias in
   `pkg/github/deprecated_tool_aliases.go`.

## Common mistakes to avoid

- **Closure-captured client**: always use `deps.GetClient(ctx)`. Tools that
  capture a single client at registration time break remote-server
  per-request injection.
- **ReadOnlyHint lying**: a tool that calls `client.Issues.Create` is NOT
  read-only. The `--read-only` flag filter trusts this hint.
- **Forgetting the snapshot**: PR CI runs `mcp-diff` which compares schemas
  against base. New tool without a snapshot = red CI.
- **Translation defaults missing**: `t("KEY", "")` is wrong. Always pass a
  full English default as the second arg.
- **Required when it should be optional**: review what the GitHub API
  actually requires; over-restrictive schemas annoy agents.
- **No pagination wrapper on listing endpoints**: wrap with
  `WithPagination(schema)` so `page`/`perPage` are added consistently.

## Parameter helper cheat sheet

| Use case | Helper |
|---|---|
| Required string/int/bool | `RequiredParam[T](args, "name")` |
| Optional, zero default | `OptionalParam[T](args, "name")` |
| Optional with explicit default | `OptionalIntParamWithDefault`, `OptionalBoolParamWithDefault` |
| Required int (typed) | `RequiredInt(args, "name")` |
| String array | `OptionalStringArrayParam(args, "name")` |
| Listing endpoint | `WithPagination(&jsonschema.Schema{...})` |

All helpers live in `pkg/github/server.go`.

## Error path

| Situation | Return |
|---|---|
| Invalid param from agent | `utils.NewToolResultError(err.Error()), nil, nil` |
| GitHub REST error | `ghErrors.NewGitHubAPIErrorResponse(ctx, "msg", resp, err), nil, nil` |
| GitHub GraphQL error | `ghErrors.NewGitHubAPIErrorResponse(ctx, "msg", nil, err), nil, nil` |
| Marshal failed (programmer bug) | `nil, nil, fmt.Errorf("...: %w", err)` |

The third return (Go `error`) is for the MCP framework. Anything an agent
should see goes in the first return with `IsError: true`.
