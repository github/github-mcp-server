# Typed tool schemas

Typed tool registrations use concrete Go input and output types with the MCP
Go SDK's `mcp.AddTool` path. The SDK infers schemas when the tool definition
does not provide them, validates arguments before the handler runs, and
validates typed output. Keep business rules that JSON Schema cannot express
in the handler or a preflight callback.

```go
tool := github.NewToolWithSchemaOptions[workflowInput, workflowOutput](
    toolset,
    mcp.Tool{Name: "list_workflow_runs"},
    scopeAccess,
    inventory.TypedSchemaOptions{
        InputEnums: []inventory.SchemaEnum{
            {Path: "status", Values: github.WorkflowStatusValues()},
        },
    },
    listWorkflowRuns,
)
```

`SchemaEnum.Path` addresses inferred properties with dot-separated names and
uses `items` to descend into array elements. `EnumSchema` creates a standalone
string enum schema, while `WithEnum` returns a cloned explicit schema with an
enum applied at a path. Inferred and explicit schema helpers cache immutable
schemas; do not mutate the returned pointers.

When compatibility requires a broader runtime input contract than the one
advertised to clients, provide `ValidationInputSchema`. The tool's declared
`InputSchema` remains visible while the SDK validates calls against the
runtime-only schema. Use `Preflight` for checks that need raw arguments or
request dependencies before typed decoding; it may return a derived context
for the handler. Input normalizers are only for compatibility transformations,
not a replacement for schema validation.

The SDK applies defaults from the runtime input schema before decoding. If an
omitted field must remain omitted, build the runtime schema with
`inventory.CloneSchemaWithoutDefaults` before applying validation-only
changes. Use `inventory.CloneSchema` when deriving other runtime-only schema
variants so numeric bound pointers and nested metadata are detached as well as
subschemas. Default removal visits each child once per parent. The advertised
schema can retain its defaults; the constructor caches the runtime schema
without mutating either caller-owned schema.

The output schema and `structuredContent` are exposed only for a negotiated,
SDK-supported protocol version `2026-07-28` or later. Unknown versions are
treated as legacy, including unsupported future dates; older, absent, and
malformed versions also retain the legacy text result. Normal `Inventory.RegisterTools` and
`ServerTool.RegisterFunc` registrations select behavior per request. Use
`RegisterToolsForProtocolEra` or `RegisterFuncForProtocolEra` only when the
protocol era is already known before server construction, such as a stateless
request-scoped server.

If a handler intentionally returns non-JSON text or content blocks, set
`PreserveHandlerContent` in `TypedSchemaOptions`, or call
`inventory.PreserveToolHandlerContent(ctx)` from the handler middleware for
request-dependent output such as CSV.
