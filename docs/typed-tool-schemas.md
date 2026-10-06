# Typed tool schemas

Typed tool registrations use concrete Go input and output types with the MCP
Go SDK's `mcp.AddTool` path. Inventory infers missing schemas and validates
arguments against the cached input schema before the handler runs; the SDK
adapts and validates typed output. Keep business rules that JSON Schema cannot express
in the handler or a preflight callback.

Input inference unwraps one pointer level, matching the SDK's object input
contract. Output pointers retain nullable schemas and may return JSON null.

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
`InputSchema` remains visible while inventory validates calls against the
runtime-only schema. Use `Preflight` for checks that need raw arguments or
request dependencies before typed decoding; it may return a derived context
for the handler. Input normalizers are only for compatibility transformations,
not a replacement for schema validation.

Inventory applies defaults from the runtime input schema before decoding. If an
omitted field must remain omitted, build the runtime schema with
`inventory.CloneSchemaWithoutDefaults` before applying validation-only
changes. Use `inventory.CloneSchema` when deriving other runtime-only schema
variants so numeric bound pointers and nested metadata are detached as well as
subschemas. Default removal visits each child once per parent. The advertised
schema can retain its defaults; the constructor caches the runtime schema
without mutating either caller-owned schema.

Availability and authorization guards run before preflight, normalization,
and input validation. A guard or preflight result is passed through the
registered SDK handler without decoding the original arguments or invoking
user code, so the SDK still finalizes multi-round-trip results with
`resultType: "input_required"` (or `"complete"`). The permissive SDK input
envelope used for this handoff is never advertised and does not replace the
original validation schema for real calls.

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
request-dependent output such as CSV. This is for intentional non-DTO responses
such as resources, errors, or CSV; ordinary declared structured success must
retain shared DTO serialization, with modern JSON text equal to
`structuredContent`, even when legacy handler text was a plain mutation message.
