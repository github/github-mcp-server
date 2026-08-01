# Support for MCP server features

%toc

## Prompts

MCP servers can provide LLM prompt templates (called simply
[_prompts_](https://modelcontextprotocol.io/specification/2025-06-18/server/prompts))
to clients. Every prompt has a required name which identifies it, and a set of
named arguments, which are strings.

**Client-side**: To list the server's prompts, use the 
[`ClientSession.Prompts`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.Prompts)
iterator, or the lower-level
[`ClientSession.ListPrompts`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.ListPrompts)
(see [pagination](#pagination) below). Set
[`ClientOptions.PromptListChangedHandler`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientOptions.PromptListChangedHandler)
to be notified of changes in the list of prompts.

Call
[`ClientSession.GetPrompt`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.GetPrompt)
to retrieve a prompt by name, providing arguments for expansion. 

**Server-side**: Use
[`Server.AddPrompt`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#Server.AddPrompt)
to add a prompt to the server along with its handler.
The server will have the `prompts` capability if any prompt is added before the
server is connected to a client, or if
[`ServerOptions.HasPrompts`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ServerOptions.HasPrompts)
is explicitly set. When a prompt is added, any clients already connected to the
server will be notified via a `notifications/prompts/list_changed`
notification.

%include ../../mcp/server_example_test.go prompts -

## Resources

In MCP terms, a _resource_ is some data referenced by a URI.
MCP servers can serve resources to clients.
They can register resources individually, or register a _resource template_
that uses a URI pattern to describe a collection of resources.


**Client-side**:
Call [`ClientSession.ReadResource`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.ReadResource)
to read a resource.
The SDK ensures that a read succeeds only if the URI matches a registered resource exactly,
or matches the URI pattern of a resource template.

To list a server's resources and resource templates, use the 
[`ClientSession.Resources`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.Resources)
and
[`ClientSession.ResourceTemplates`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.ResourceTemplates)
iterators, or the lower-level `ListXXX` calls (see [pagination](#pagination)).
Set
[`ClientOptions.ResourceListChangedHandler`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientOptions.ResourceListChangedHandler)
to be notified of changes in the lists of resources or resource templates.

Clients can be notified when the contents of a resource changes by subscribing to the resource's URI.
Call
[`ClientSession.Subscribe`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.Subscribe)
to subscribe to a resource
and
[`ClientSession.Unsubscribe`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.Unsubscribe)
to unsubscribe.
Set
[`ClientOptions.ResourceUpdatedHandler`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientOptions.ResourceUpdatedHandler)
to be notified of changes to subscribed resources. On `2026-07-28` and
later sessions the SDK delivers these notifications over a
`subscriptions/listen` stream instead of the legacy `resources/subscribe`
RPC; see [Subscriptions
(`subscriptions/listen`)](protocol.md#subscriptions-subscriptionslisten)
for the wire-level details.

**Server-side**:
Use
[`Server.AddResource`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#Server.AddResource)
or
[`Server.AddResourceTemplate`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#Server.AddResourceTemplate)
to add a resource or resource template to the server along with its handler.
A
[`ResourceHandler`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ResourceHandler)
maps a URI to the contents of a resource, which can include text, binary data,
or both. 
If `AddResource` or `AddResourceTemplate` is called before a server is connected, the server will have the
`resources` capability.
The server will have the `resources` capability if any resource or resource template is added before the
server is connected to a client, or if
[`ServerOptions.HasResources`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ServerOptions.HasResources)
is explicitly set. When a prompt is added, any clients already connected to the
server will be notified via a `notifications/resources/list_changed`
notification.


%include ../../mcp/server_example_test.go resources -

## Tools

MCP servers can provide
[tools](https://modelcontextprotocol.io/specification/2025-06-18/server/tools)
to allow clients to interact with external systems or functionality. Tools are
effectively remote function calls, and the Go SDK provides mechanisms to bind
them to ordinary Go functions.

**Client-side**: To list the server's tools, use the
[`ClientSession.Tools`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.Tools)
iterator, or the lower-level
[`ClientSession.ListTools`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.ListTools)
(see [pagination](#pagination)). Set
[`ClientOptions.ToolListChangedHandler`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientOptions.ToolListChangedHandler)
to be notified of changes in the list of tools.

To call a tool, use
[`ClientSession.CallTool`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.CallTool)
with `CallToolParams` holding the name and arguments of the tool to call.

```go
res, err := session.CallTool(ctx, &mcp.CallToolParams{
	Name:      "my_tool",
	Arguments: map[string]any{"name": "user"},
})
```

Arguments may be any value that can be marshaled to JSON.

**Server-side**: the basic API for adding a tool is symmetrical with the API
for prompts or resources:
[`Server.AddTool`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#Server.AddTool)
adds a
[`Tool`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#Tool) to
the server along with its
[`ToolHandler`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ToolHandler)
to handle it. The server will have the `tools` capability if any tool is added
before the server is connected to a client, or if
[`ServerOptions.HasTools`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ServerOptions.HasPrompts)
is explicitly set. When a tool is added, any clients already connected to the
server will be notified via a `notifications/tools/list_changed` notification.

However, the `Server.AddTool` API leaves it to the user to implement the tool
handler correctly according to the spec, providing very little out of the box.
In order to implement a tool, the user must do all of the following:

- Provide a tool input and output schema.
- Validate the tool arguments against its input schema.
- Unmarshal the input schema into a Go value
- Execute the tool logic.
- Marshal the tool's structured output (if any) to JSON, and store it in the
  result's `StructuredOutput` field as well as the unstructured `Content` field.
- Validate that output JSON against the tool's output schema.
- If any tool errors occurred, pack them into the unstructured content and set
  `IsError` to `true.`

For this reason, the SDK provides a generic
[`AddTool`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#AddTool)
function that handles this for you. It can bind a tool to any function with the
following shape:

```go
func(_ context.Context, request *CallToolRequest, input In) (result *CallToolResult, output Out, _ error)
```

This is like a `ToolHandler`, but with an extra arbitrary `In` input parameter,
and `Out` output parameter.

Such a function can then be bound to the server using `AddTool`:

```go
mcp.AddTool(server, &mcp.Tool{Name: "my_tool"}, handler)
```

This does the following automatically:

- If `Tool.InputSchema` is unset, the input schema is inferred from the `In`
  type, which must be a struct or map.
- If `Tool.OutputSchema` is unset and the `Out` type is not `any`, the output
  schema is inferred from the `Out` type. Per SEP-2106, `Out` may be any Go
  type whose inferred schema is a valid JSON Schema (struct, map, slice,
  primitive, etc.).
- Optional `jsonschema` struct tags provide argument and output descriptions.
- Tool arguments are validated against the input schema.
- Tool arguments are marshaled into the `In` value.
- Tool output (the `Out` value) is marshaled into the result's
  `StructuredOutput`, as well as the unstructured `Content`.
- Output is validated against the tool's output schema.
- If an ordinary error is returned, it is stored int the `CallToolResult` and
  `IsError` is set to `true`.

In fact, under ordinary circumstances, the user can ignore `CallToolRequest`
and `CallToolResult`.

For a more realistic example, consider a tool that retrieves the weather:

%include ../../mcp/tool_example_test.go weathertool -

In this case, we want to customize part of the inferred schema, though we can
still infer the rest. Since we want to control the inference ourselves, we set
the `Tool.InputSchema` explicitly:

%include ../../mcp/tool_example_test.go customschemas -

_See [mcp/tool_example_test.go](https://github.com/modelcontextprotocol/go-sdk/blob/main/mcp/tool_example_test.go) for the full
example, or [examples/server/toolschemas](https://github.com/modelcontextprotocol/go-sdk/blob/main/examples/server/toolschemas/main.go)
for more examples of customizing tool schemas._

**Stateless server deployments:** Some deployments create a new
[`Server`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#Server)
for each incoming request, re-registering tools every time. To avoid repeated
schema generation, create a
[`SchemaCache`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#SchemaCache)
and share it across server instances:

```go
var schemaCache = mcp.NewSchemaCache() // create once at startup

func handleRequest(w http.ResponseWriter, r *http.Request) {
    s := mcp.NewServer(impl, &mcp.ServerOptions{SchemaCache: schemaCache})
    mcp.AddTool(s, myTool, myHandler)
    // ...
}
```

## Multi Round-Trip Requests

[SEP-2322](https://github.com/modelcontextprotocol/modelcontextprotocol/pull/2322)
defines a new pattern for server-to-client requests (sampling, elicitation,
roots). Instead of issuing a fresh JSON-RPC request mid-flight, the server
returns an `InputRequiredResult` from its in-flight handler — the
`InputRequests` field of `CallToolResult`, `GetPromptResult`, or
`ReadResourceResult` carries the requests for additional information. The
client responds by retrying the original call with `InputResponses` set.

The SDK supports this pattern from both sides without requiring callers to
choose:

- **Returning input requests from a handler.** Set `InputRequests` on the
  result you return.
  Leave `Content` / `StructuredContent` empty — returning
  both at once is a server bug and the SDK returns `-32603 InternalError`.
- **Calling the legacy APIs.** `ServerSession.Elicit`,
  `ServerSession.CreateMessage(WithTools)`, and `ServerSession.ListRoots`
  remain available and work for both new and legacy clients.

### Talking to legacy clients

The server installs `serverMultiRoundTripMiddleware` automatically. For
clients on a protocol version earlier than `2026-07-28`, the middleware
intercepts any `InputRequiredResult` your handler returns, fulfils each
input request itself by calling the legacy server-initiated APIs
(`Elicit`, `CreateMessage`, `ListRoots`), and re-invokes your handler
exactly once with the responses already populated. This means a handler
written in the MRTR style works against both old and new clients without
code changes.

### Example

The following example shows a "greet" tool whose handler asks the user
for their name via elicitation before producing the final greeting. The
handler runs twice — once to issue the elicitation, once to consume the
response — but the client call site sees a single `CallTool` returning the
final result, because the SDK's MRTR middleware handles the round trip on
either side of the wire (depending on the negotiated protocol version).

%include ../../mcp/server_example_test.go mrtr -

## Cacheable list results

[SEP-2549](https://github.com/modelcontextprotocol/modelcontextprotocol/pull/2549)
adds `ttlMs` and `cacheScope` fields to the results of `tools/list`,
`prompts/list`, `resources/list`, `resources/templates/list`, and
`resources/read`. Both fields complement existing list
results: clients use `ttlMs` as a freshness hint to reduce polling,
and `cacheScope` (`"public"` or `"private"`) controls whether shared
intermediaries may cache the response.

**Server-side defaults.** After paginating, the SDK sets `CacheScope = "public"`
and leaves `TTLMs = 0` (which the client cache treats as "immediately
stale"). A handler that wants its responses cached must set `TTLMs`
explicitly on the returned result.

```go
mcp.AddTool(server, &mcp.Tool{Name: "expensive"}, func(...) (*mcp.CallToolResult, any, error) {
    // ...
})
// Override the default for tools/list:
server.AddSendingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
    return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
        res, err := next(ctx, method, req)
        if lr, ok := res.(*mcp.ListToolsResult); ok {
            lr.TTLMs = 30_000 // 30s cache freshness hint
        }
        return res, err
    }
})
```

## Utilities

### Completion

To support the
[completion](https://modelcontextprotocol.io/specification/2025-06-18/server/utilities/completion)
capability, the server needs a completion handler.

**Client-side**: completion is called using the
[`ClientSession.Complete`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.Complete)
method.

**Server-side**: completion is enabled by setting
[`ServerOptions.CompletionHandler`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ServerOptions.CompletionHandler).
If this field is set to a non-nil value, the server will advertise the
`completions` server capability, and use this handler to respond to completion
requests.

%include ../../examples/server/completion/main.go completionhandler -

### Logging

> **Note:** The logging feature is deprecated as of protocol version
> 2026-07-28
> ([SEP-2577](https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging)).
> It remains fully functional during the deprecation window (at least twelve
> months). The SDK continues to support logging for compatibility. Servers
> should migrate to stderr logging (for STDIO transports) or OpenTelemetry.

MCP servers can send logging messages to MCP clients.
(This form of logging is distinct from server-side logging, where the
server produces logs that remain server-side, for use by server maintainers.)

**Server-side**:
The minimum log level is part of the server state.
For stateful sessions, there is no default log level: no log messages will be sent
until the client calls `SetLevel` (see below).
For stateful sessions, the level defaults to "info".

[`ServerSession.Log`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ServerSession.Log) is the low-level way for servers to log to clients.
It sends a logging notification to the client if the level of the message
is at least the minimum log level.

For a simpler API, use [`NewLoggingHandler`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#NewLoggingHandler) to obtain a [`slog.Handler`](https://pkg.go.dev/log/slog#Handler).
By setting [`LoggingHandlerOptions.MinInterval`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#LoggingHandlerOptions.MinInterval), the handler can be rate-limited
to avoid spamming clients with too many messages.

Servers always report the logging capability.


**Client-side**:
Set [`ClientOptions.LoggingMessageHandler`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientOptions.LoggingMessageHandler) to receive log messages.

Call [`ClientSession.SetLevel`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.SetLevel) to change the log level for a session.

%include ../../mcp/server_example_test.go logging -

## Capabilities

Server capabilities are advertised to clients during the initialization
handshake. By default, the SDK advertises only the `logging` capability.
Additional capabilities are automatically added when features are registered
(e.g., adding a tool adds the `tools` capability).

### Capability inference

When features such as tools, prompts, or resources are added to the server
(e.g., via `Server.AddTool`), their capability is automatically inferred, with
default value `{listChanged:true}`. Similarly, if the
`ServerOptions.SubscribeHandler` or `ServerOptions.CompletionHandler` are set,
the corresponding capability is added.

### Explicit capabilities

To explicitly declare capabilities, or to override the [default inferred
capability](#capability-inference), set
[`ServerOptions.Capabilities`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ServerOptions.Capabilities).
This sets the default server capabilities, before any capabilities are added
based on configured handlers. If a capability is already present as a field in
`Capabilities`, adding a feature or handler will not change its configuration.

This allows you to:

- **Disable default capabilities**: Pass an empty `&ServerCapabilities{}` to
  disable all defaults, including logging.
- **Disable listChanged notifications**: Set `ListChanged: false` on a
  capability to prevent the server from sending list-changed notifications
  when features are added or removed.
- **Pre-declare capabilities**: Declare capabilities before features are
  registered, useful for servers that load features dynamically.

```go
// Disable listChanged notifications for tools
server := mcp.NewServer(impl, &mcp.ServerOptions{
    Capabilities: &mcp.ServerCapabilities{
        Logging: &mcp.LoggingCapabilities{},
        Tools:   &mcp.ToolCapabilities{ListChanged: false},
    },
})
```

**Deprecated**: The `HasPrompts`, `HasResources`, and `HasTools` fields on
`ServerOptions` are deprecated. Use `Capabilities` instead.

### Extensions

[SEP-2133](https://github.com/modelcontextprotocol/modelcontextprotocol/pull/2133)
adds an `extensions` map to `ServerCapabilities` so that optional
capabilities outside the core protocol can be declared on the wire. Keys
are namespaced as `"{vendor-prefix}/{extension-name}"`; values are
per-extension settings objects.

### Pagination

Server-side feature lists may be
[paginated](https://modelcontextprotocol.io/specification/2025-06-18/server/utilities/pagination),
using cursors. The SDK supports this by default.

**Client-side**: The `ClientSession` provides methods returning
[iterators](https://go.dev/blog/range-functions) for each feature type.
These iterators are an `iter.Seq2[Feature, error]`, where the error value
indicates whether page retrieval failed.

- [`ClientSession.Prompts`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.Prompts)
  iterates prompts.
- [`ClientSession.Resource`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.Resource)
  iterates resources.
- [`ClientSession.ResourceTemplates`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.ResourceTemplates)
  iterates resource templates.
- [`ClientSession.Tools`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientSession.Tools)
  iterates tools.

The `ClientSession` also exposes `ListXXX` methods for fine-grained control
over pagination.

**Server-side**: pagination is on by default, so in general nothing is required
server-side. However, you may use
[`ServerOptions.PageSize`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ServerOptions.PageSize)
to customize the page size.
