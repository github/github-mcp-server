# Support for MCP client features

%toc

## Roots

> **Note:** The roots feature is deprecated as of protocol version 2026-07-28
> ([SEP-2577](https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging)).
> It remains fully functional during the deprecation window (at least twelve
> months). The SDK continues to support roots for compatibility. New code
> should pass paths via tool parameters, resource URIs, or configuration
> instead.

MCP allows clients to specify a set of filesystem
["roots"](https://modelcontextprotocol.io/specification/2025-06-18/client/roots).
The SDK supports this as follows:

**Client-side**: The SDK client always has the `roots.listChanged` capability.
To add roots to a client, use the
[`Client.AddRoots`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#Client.AddRoots)
and
[`Client.RemoveRoots`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#Client.RemoveRoots)
methods. If any servers are already [connected](protocol.md#lifecycle) to the
client, a call to `AddRoot` or `RemoveRoots` will result in a
`notifications/roots/list_changed` notification to each connected server.

**Server-side**: To query roots from the server, use the
[`ServerSession.ListRoots`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ServerSession.ListRoots)
method. To receive notifications about root changes, set
[`ServerOptions.RootsListChangedHandler`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ServerOptions.RootsListChangedHandler).
For protocol versions `2026-07-28` and later, `ListRoots` requests are
delivered via the
[Multi Round-Trip Requests](protocol.md#multi-round-trip-requests-mrtr)
pattern.

%include ../../mcp/client_example_test.go roots -

## Sampling

> **Note:** The sampling feature is deprecated as of protocol version
> 2026-07-28
> ([SEP-2577](https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging)).
> It remains fully functional during the deprecation window (at least twelve
> months). The SDK continues to support sampling for compatibility. Servers
> that need LLM completions should call LLM provider APIs directly.

[Sampling](https://modelcontextprotocol.io/specification/2025-06-18/client/sampling)
is a way for servers to leverage the client's AI capabilities. It is
implemented in the SDK as follows:

**Client-side**: To add the `sampling` capability to a client, set 
[`ClientOptions.CreateMessageHandler`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientOptions.CreateMessageHandler).
This function is invoked whenever the server requests sampling.

**Server-side**: To use sampling from the server, call
[`ServerSession.CreateMessage`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ServerSession.CreateMessage).

For protocol versions `2026-07-28` and later, sampling requests are
delivered via the
[Multi Round-Trip Requests](protocol.md#multi-round-trip-requests-mrtr)
pattern.

%include ../../mcp/client_example_test.go sampling -

## Elicitation

[Elicitation](https://modelcontextprotocol.io/specification/2025-06-18/client/elicitation)
allows servers to request user inputs. It is implemented in the SDK as follows:

**Client-side**: To add the `elicitation` capability to a client, set
[`ClientOptions.ElicitationHandler`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientOptions.ElicitationHandler).
The elicitation handler must return a result that matches the requested schema;
otherwise, elicitation returns an error. If your handler supports [URL mode
elicitation](https://modelcontextprotocol.io/specification/2025-11-25/client/elicitation#url-mode-elicitation-requests),
you must declare that capability explicitly (see [Capabilities](#capabilities))

**Server-side**: To use elicitation from the server, call
[`ServerSession.Elicit`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ServerSession.Elicit).

For protocol versions `2026-07-28` and later, elicitation requests are
delivered via the
[Multi Round-Trip Requests](protocol.md#multi-round-trip-requests-mrtr)
pattern.

%include ../../mcp/client_example_test.go elicitation -

## Multi Round-Trip Requests

[SEP-2322](https://github.com/modelcontextprotocol/modelcontextprotocol/pull/2322)
introduces the MRTR pattern: server-to-client requests for sampling,
elicitation, and roots are no longer issued as fresh JSON-RPC requests but
are carried inside the in-flight reply of a `tools/call`, `prompts/get`, or
`resources/read`. The client must respond by retrying the original request
with the produced responses.

The SDK installs `clientMultiRoundTripMiddleware` for every client by
default. The middleware:

1. Inspects each `tools/call`/`prompts/get`/`resources/read` reply.
2. If the result's `NeedsInput()` is true, fans out the `InputRequests` map
   concurrently, calling the configured handler for each (`elicit`,
   `createMessage`/`createMessageWithTools`, or `listRoots`).
3. Threads the server-supplied opaque `RequestState` back unchanged.
4. Retries the original request with the responses set, repeating until the
   result no longer needs input.

The middleware is enabled by default. To opt out, set
[`ClientOptions.MultiRoundTrip.Disabled = true`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#MultiRoundTripOptions);
the client will then surface input-required results directly to the caller
(the returned `CallToolResult`, `GetPromptResult`, or `ReadResourceResult`
will report `NeedsInput() == true` and expose the server's `InputRequests`
and opaque `RequestState`). Your code must fulfil each request and re-issue
the original call with `InputResponses` set and `RequestState` echoed
back.

For legacy (`<= 2025-11-25`) servers, the SDK transparently sends server
requests on the legacy server-initiated channel; the MRTR machinery is a
no-op in that direction. For legacy clients talking to MRTR-style servers,
the server SDK applies the inverse compatibility shim — see the
[server-side documentation](server.md#multi-round-trip-requests-mrtr).

## Capabilities

Client capabilities are advertised to servers during the initialization
handshake. [By default](rough_edges.md), the SDK advertises the `logging`
capability. Additional capabilities are automatically added when server
features are added (e.g. via `AddTool`), or when handlers are set in the
`ServerOptions` struct (e.g., setting `CompletionHandler` adds the
`completions` capability), or may be configured explicitly.

### Capability inference

When handlers are set on `ClientOptions` (e.g., `CreateMessageHandler` or
`ElicitationHandler`), the corresponding capability is automatically added if
not already present, with a default configuration.

For elicitation, if the handler is set but no `Capabilities.Elicitation` is
specified, the client defaults to form elicitation. To enable URL elicitation
or both modes, [configure `Capabilities.Elicitation`
explicitly](#explicit-capabilities).

See the [`ClientCapabilities`
documentation](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientCapabilities)
for further details on inference.

### Explicit capabilities

To explicitly declare capabilities, or to override the [default inferred
capability](#capability-inference), set
[`ClientOptions.Capabilities`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp#ClientOptions.Capabilities).
This sets the initial client capabilities, before any capabilities are added
based on configured handlers. If a capability is already present in
`Capabilities`, adding a handler will not change its configuration.

This allows you to:

- **Disable default capabilities**: Pass an empty `&ClientCapabilities{}` to
  disable all defaults, including roots.
- **Disable listChanged notifications**: Set `ListChanged: false` on a
  capability to prevent the client from sending list-changed notifications
  when roots are added or removed.
- **Configure elicitation modes**: Specify which elicitation modes (form, URL)
  the client supports.

```go
// Configure elicitation modes and disable roots.
client := mcp.NewClient(impl, &mcp.ClientOptions{
    Capabilities: &mcp.ClientCapabilities{
        Elicitation: &mcp.ElicitationCapabilities{
            Form: &mcp.FormElicitationCapabilities{},
            URL:  &mcp.URLElicitationCapabilities{},
        },
    },
    ElicitationHandler: handler,
})
```

### Extensions

[SEP-2133](https://github.com/modelcontextprotocol/modelcontextprotocol/pull/2133)
adds an `extensions` map to `ClientCapabilities` and `ServerCapabilities` so
that optional capabilities outside the core protocol can be declared on the
wire. Keys are namespaced as `"{vendor-prefix}/{extension-name}"`; values
are per-extension settings objects.

