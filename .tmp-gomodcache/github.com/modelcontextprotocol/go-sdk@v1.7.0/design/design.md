# Go SDK Design

This document discusses the design of a Go SDK for the [model context protocol](https://modelcontextprotocol.io/specification/2025-03-26). The [github.com/modelcontextprotocol/go-sdk/mcp](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp@master) package contains a prototype that we built to explore the MCP design space. Many of the ideas there are present in this document. However, we have diverged from and expanded on the APIs of that prototype, and this document should be considered canonical.

## Similarities and differences with mark3labs/mcp-go (and others)

The most popular unofficial MCP SDK for Go is [mark3labs/mcp-go](https://pkg.go.dev/github.com/mark3labs/mcp-go). As of this writing, it is imported by over 400 packages that span over 200 modules.

We admire mcp-go, and where possible tried to align with its design. However, the APIs here diverge in a number of ways in order to keep the official SDK minimal, allow for future spec evolution, and support additional features. We have noted significant differences from mcp-go in the sections below. Although the API here is not compatible with mcp-go, translating between them should be straightforward in most cases. (Later, we will provide a detailed translation guide.)

Thank you to everyone who contributes to mcp-go and other Go SDKs. We hope that we can collaborate to leverage all that we've learned about MCP and Go in an official SDK.

# Requirements

These may be obvious, but it's worthwhile to define goals for an official MCP SDK. An official SDK should aim to be:

- **complete**: it should be possible to implement every feature of the MCP spec, and these features should conform to all of the semantics described by the spec.
- **idiomatic**: as much as possible, MCP features should be modeled using features of the Go language and its standard library. Additionally, the SDK should repeat idioms from similar domains.
- **robust**: the SDK itself should be well tested and reliable, and should enable easy testability for its users.
- **future-proof**: the SDK should allow for future evolution of the MCP spec, in such a way that we can (as much as possible) avoid incompatible changes to the SDK API.
- **extensible**: to best serve the previous four concerns, the SDK should be minimal. However, it should admit extensibility using (for example) simple interfaces, middleware, or hooks.

# Design

In the sections below, we visit each aspect of the MCP spec, in approximately the order they are presented by the [official spec](https://modelcontextprotocol.io/specification/2025-03-26) For each, we discuss considerations for the Go implementation, and propose a Go API.

## Foundations

### Package layout

In the sections that follow, it is assumed that most of the MCP API lives in a single shared package, the `mcp` package. This is inconsistent with other MCP SDKs, but is consistent with Go packages like `net/http`, `net/rpc`, or `google.golang.org/grpc`. We believe that having a single package aids discoverability in package documentation and in the IDE. Furthermore, it avoids arbitrary decisions about package structure that may be rendered inaccurate by future evolution of the spec.

Functionality that is not directly related to MCP (like jsonschema or jsonrpc2) belongs in a separate package.

Therefore, this is the core package layout, assuming github.com/modelcontextprotocol/go-sdk as the module path.

- `github.com/modelcontextprotocol/go-sdk/mcp`: the bulk of the user facing API
- `github.com/modelcontextprotocol/go-sdk/jsonschema`: a jsonschema implementation, with validation
- `github.com/modelcontextprotocol/go-sdk/internal/jsonrpc2`: a fork of x/tools/internal/jsonrpc2_v2

The JSON-RPC implementation is hidden, to avoid tight coupling. As described in the next section, the only aspects of JSON-RPC that need to be exposed in the SDK are the message types, for the purposes of defining custom transports. We can expose these types by promoting them from the `mcp` package using aliases or wrappers.

**Difference from mcp-go**: Our `mcp` package includes all the functionality of mcp-go's `mcp`, `client`, `server` and `transport` packages.

### JSON-RPC and Transports

The MCP is defined in terms of client-server communication over bidirectional JSON-RPC message connections. Specifically, version `2025-03-26` of the spec defines two transports:

- **stdio**: communication with a subprocess over stdin/stdout.
- **streamable http**: communication over a relatively complicated series of text/event-stream GET and HTTP POST requests.

Additionally, version `2024-11-05` of the spec defined a simpler (yet stateful) HTTP transport:

- **sse**: client issues a hanging GET request and receives messages via `text/event-stream`, and sends messages via POST to a session endpoint.

Furthermore, the spec [states](https://modelcontextprotocol.io/specification/2025-03-26/basic/transports#custom-transports) that it must be possible for users to define their own custom transports.

Given the diversity of the transport implementations, they can be challenging to abstract. However, since JSON-RPC requires a bidirectional connection, we can use this to model the MCP transport abstraction:

```go
type (
	JSONRPCID       = jsonrpc2.ID
	JSONRPCMessage  = jsonrpc2.Message
	JSONRPCRequest  = jsonrpc2.Request
	JSONRPCResponse = jsonrpc2.Response
)

// A Transport is used to create a bidirectional connection between MCP client
// and server.
type Transport interface {
    Connect(ctx context.Context) (Connection, error)
}

// A Connection is a bidirectional jsonrpc2 connection.
type Connection interface {
    Read(ctx context.Context) (JSONRPCMessage, error)
    Write(ctx context.Context, JSONRPCMessage) error
    Close() error
}
```

Methods accept a Go `Context` and return an `error`, as is idiomatic for APIs that do I/O.

A `Transport` is something that connects a logical JSON-RPC connection, and nothing more. Connections must be closeable in order to implement client and server shutdown, and therefore conform to the `io.Closer` interface.

Other SDKs define higher-level transports, with, for example, methods to send a notification or make a call. Those are jsonrpc2 operations on top of the logical connection, and the lower-level interface is easier to implement in most cases, which means it is easier to implement custom transports.

For our prototype, we've used an internal `jsonrpc2` package based on the Go language server `gopls`, which we propose to fork for the MCP SDK. It already handles concerns like client/server connection, request lifecycle, cancellation, and shutdown.

**Differences from mcp-go**: The Go team has a battle-tested JSON-RPC implementation that we use for gopls, our Go LSP server. We are using the new version of this library as part of our MCP SDK. It handles all JSON-RPC 2.0 features, including cancellation.

The `Transport` interface here is lower-level than that of mcp-go, but serves a similar purpose. We believe the lower-level interface is easier to implement.

#### stdio transports

In the MCP Spec, the **stdio** transport uses newline-delimited JSON to communicate over stdin/stdout. It's possible to model both client side and server side of this communication with a shared type that communicates over an `io.ReadWriteCloser`. However, for the purposes of future-proofing, we should use a different types for client and server stdio transport.

The `CommandTransport` is the client side of the stdio transport, and connects by starting a command and streaming JSON-RPC messages over stdin/stdout.

```go
// A CommandTransport is a [Transport] that runs a command and communicates
// with it over stdin/stdout, using newline-delimited JSON.
type CommandTransport struct { Command *exec.Command }

// Connect starts the command, and connects to it over stdin/stdout.
func (*CommandTransport) Connect(ctx context.Context) (Connection, error) {
```

The `StdioTransport` is the server side of the stdio transport, and connects by binding to `os.Stdin` and `os.Stdout`.

```go
// A StdioTransport is a [Transport] that communicates using newline-delimited
// JSON over stdin/stdout.
type StdioTransport struct { }

func (t *StdioTransport) Connect(context.Context) (Connection, error)
```

#### HTTP transports

The HTTP transport APIs are even more asymmetrical. Since connections are initiated via HTTP requests, the client developer will create a transport, but the server developer will typically install an HTTP handler. Internally, the HTTP handler will create a logical transport for each new client connection.

Importantly, since they serve many connections, the HTTP handlers must accept a callback to get an MCP server for each new session. As described below, MCP servers can optionally connect to multiple clients. This allows customization of per-session servers: if the MCP server is stateless, the user can return the same MCP server for each connection. On the other hand, if any per-session customization is required, it is possible by returning a different `Server` instance for each connection.

Both the SSE and Streamable HTTP server transports are http.Handlers which serve messages to their associated connection. Consequently, they can be connected at most once.

```go
// SSEHTTPHandler is an http.Handler that serves SSE-based MCP sessions as defined by
// the 2024-11-05 version of the MCP protocol.
type SSEHTTPHandler struct { /* unexported fields */ }

// NewSSEHTTPHandler returns a new [SSEHTTPHandler] that is ready to serve HTTP.
//
// The getServer function is used to bind created servers for new sessions. It
// is OK for getServer to return the same server multiple times.
func NewSSEHTTPHandler(getServer func(request *http.Request) *Server) *SSEHTTPHandler

func (*SSEHTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request)

// Close prevents the SSEHTTPHandler from accepting new sessions, closes active
// sessions, and awaits their graceful termination.
func (*SSEHTTPHandler) Close() error
```

Notably absent are options to hook into low-level request handling for the purposes of authentication or context injection. These concerns are instead handled using standard HTTP middleware patterns. For middleware at the level of the MCP protocol, see [Middleware](#Middleware) below.

By default, the SSE handler creates messages endpoints with the `?sessionId=...` query parameter. Users that want more control over the management of sessions and session endpoints may write their own handler, and create `SSEServerTransport` instances themselves for incoming GET requests.

```go
// A SSEServerTransport is a logical SSE session created through a hanging GET
// request.
type SSEServerTransport struct {
    Endpoint string
    Response http.ResponseWriter
}

// ServeHTTP handles POST requests to the transport endpoint.
func (*SSEServerTransport) ServeHTTP(w http.ResponseWriter, req *http.Request)

// Connect sends the 'endpoint' event to the client.
// See [SSEServerTransport] for more details on the [Connection] implementation.
func (*SSEServerTransport) Connect(context.Context) (Connection, error)
```

The SSE client transport is simpler, and hopefully self-explanatory.

```go
type SSEClientTransport struct {
	// Endpoint is the SSE endpoint to connect to.
	Endpoint string
	// HTTPClient is the client to use for making HTTP requests. If nil,
	// http.DefaultClient is used.
	HTTPClient *http.Client
}

// Connect connects through the client endpoint.
func (*SSEClientTransport) Connect(ctx context.Context) (Connection, error)
```

The Streamable HTTP transports are similar to the SSE transport, albeit with a
more complicated implementation. For brevity, we summarize only the differences
from the equivalent SSE types:

```go
// The StreamableHTTPHandler interface is symmetrical to the SSEHTTPHandler.
type StreamableHTTPHandler struct { /* unexported fields */ }
func NewStreamableHTTPHandler(getServer func(request *http.Request) *Server) *StreamableHTTPHandler
func (*StreamableHTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request)
func (*StreamableHTTPHandler) Close() error

// Unlike the SSE transport, the streamable transport constructor accepts a
// session ID, not an endpoint, along with the HTTP response for the request
// that created the session. It is the caller's responsibility to delegate
// requests to this session.
type StreamableServerTransport struct {
	// SessionID is the ID of this session.
	SessionID string
	// Storage for events, to enable stream resumption.
	EventStore EventStore
}
func (*StreamableServerTransport) ServeHTTP(w http.ResponseWriter, req *http.Request)
func (*StreamableServerTransport) Connect(context.Context) (Connection, error)

// The streamable client handles reconnection transparently to the user.
type StreamableClientTransport struct {
	Endpoint         string
	HTTPClient       *http.Client
	ReconnectOptions *StreamableReconnectOptions
}

func (*StreamableClientTransport) Connect(context.Context) (Connection, error)
```

**Differences from mcp-go**: In mcp-go, server authors create an `MCPServer`, populate it with tools, resources and so on, and then wrap it in an `SSEServer` or `StdioServer`. Users can manage their own sessions with `RegisterSession` and `UnregisterSession`. Rather than use a server constructor to get a distinct server for each connection, there is a concept of a "session tool" that overlays tools for a specific session.

Here, we tried to differentiate the concept of a `Server`, `HTTPHandler`, and `Transport`, and provide per-session customization through either the `getServer` constructor or middleware. Additionally, individual handlers and transports here have a minimal API, and do not expose internal details. (Open question: are we oversimplifying?)

#### Other transports

We also provide a couple of transport implementations for special scenarios. An InMemoryTransport can be used when the client and server reside in the same process. A LoggingTransport is a middleware layer that logs RPC logs to a desired location, specified as an io.Writer.

```go
// An InMemoryTransport is a [Transport] that communicates over an in-memory
// network connection, using newline-delimited JSON.
type InMemoryTransport struct { /* ... */ }

// NewInMemoryTransports returns two InMemoryTransports that connect to each
// other.
func NewInMemoryTransports() (*InMemoryTransport, *InMemoryTransport)

// A LoggingTransport is a [Transport] that delegates to another transport,
// writing RPC logs to an io.Writer.
type LoggingTransport struct { 
    Delegate Transport
    Writer   io.Writer
}
```

### Protocol types

Types needed for the protocol are generated from the [JSON schema of the MCP spec](https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/schema/2025-03-26/schema.json).

These types will be included in the `mcp` package, but will be unexported unless they are needed for the user-facing API. Notably, JSON-RPC request types are elided, since they are handled by the `jsonrpc2` package and should not be observed by the user.

For user-provided data, we use `json.RawMessage` or `map[string]any`, depending on the use case.

For union types, which can't be represented in Go, we use an interface for `Content` (implemented by types like `TextContent`). For other union types like `ResourceContents`, we use a struct with optional fields.

For brevity, only a few examples are shown here:

```go
type ReadResourceParams struct {
	URI string `json:"uri"`
}

type CallToolResult struct {
	Meta    Meta      `json:"_meta,omitempty"`
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// A Content is a [TextContent], [ImageContent], [AudioContent] or
// [EmbeddedResource].
type Content interface {
	// (unexported methods)
}

// TextContent is a textual content.
type TextContent struct {
	Text string
}
// etc.
```

The `Meta` type includes a `map[string]any` for arbitrary data, and a `ProgressToken` field.

**Differences from mcp-go**: these types are largely similar, but our type generator flattens types rather than using struct embedding.

### Clients and Servers

Generally speaking, the SDK is used by creating a `Client` or `Server` instance, adding features to it, and connecting it to a peer.

However, the SDK must make a non-obvious choice in these APIs: are clients 1:1 with their logical connections? What about servers? Both clients and servers are stateful: users may add or remove roots from clients, and tools, prompts, and resources from servers. Additionally, handlers for these features may themselves be stateful, for example if a tool handler caches state from earlier requests in the session.

We believe that in the common case, any change to a client or server, such as adding a tool, is intended for all its peers. It is therefore more useful to allow multiple connections from a client, and to a server. This is similar to the `net/http` packages, in which an `http.Client` and `http.Server` each may handle multiple unrelated connections. When users add features to a client or server, all connected peers are notified of the change.

Supporting multiple connections to servers (and from clients) still allows for stateful components, as it is up to the user to decide whether or not to create distinct servers/clients for each connection. For example, if the user wants to create a distinct server for each new connection, they can do so in the `getServer` factory passed to transport handlers.

Following the terminology of the [spec](https://modelcontextprotocol.io/specification/2025-03-26/basic/transports#session-management), we call the logical connection between a client and server a "session." There must necessarily be a `ClientSession` and a `ServerSession`, corresponding to the APIs available from the client and server perspective, respectively.

```
Client                                                   Server
  ⇅                          (jsonrpc2)                     ⇅
ClientSession ⇄ Client Transport ⇄ Server Transport ⇄ ServerSession
```

Sessions are created from either `Client` or `Server` using the `Connect` method.

```go
type Client struct { /* ... */ }
func NewClient(impl *Implementation, opts *ClientOptions) *Client
func (*Client) Connect(context.Context, Transport) (*ClientSession, error)
func (*Client) Sessions() iter.Seq[*ClientSession]
// Methods for adding/removing client features are described below.

type ClientOptions struct { /* ... */ } // described below

type ClientSession struct { /* ... */ }
func (*ClientSession) Client() *Client
func (*ClientSession) Close() error
func (*ClientSession) Wait() error
// Methods for calling through the ClientSession are described below.
// For example: ClientSession.ListTools.

type Server struct { /* ... */ }
func NewServer(impl *Implementation, opts *ServerOptions) *Server
func (*Server) Connect(context.Context, Transport) (*ServerSession, error)
func (*Server) Sessions() iter.Seq[*ServerSession]
// Methods for adding/removing server features are described below.

type ServerOptions struct { /* ... */ } // described below

type ServerSession struct { /* ... */ }
func (*ServerSession) Server() *Server
func (*ServerSession) Close() error
func (*ServerSession) Wait() error
// Methods for calling through the ServerSession are described below.
// For example: ServerSession.ListRoots.
```

Here's an example of these APIs from the client side:

```go
client := mcp.NewClient(&mcp.Implementation{Name:"mcp-client", Version:"v1.0.0"}, nil)
// Connect to a server over stdin/stdout
transport := &mcp.CommandTransport{
    Command: exec.Command("myserver"},
}
session, err := client.Connect(ctx, transport)
if err != nil { ... }
// Call a tool on the server.
content, err := session.CallTool(ctx, "greet", map[string]any{"name": "you"}, nil)
...
return session.Close()
```

A server that can handle that client call would look like this:

```go
// Create a server with a single tool.
server := mcp.NewServer(&mcp.Implementation{Name:"greeter", Version:"v1.0.0"}, nil)
mcp.AddTool(server, &mcp.Tool{Name: "greet", Description: "say hi"}, SayHi)
// Run the server over stdin/stdout, until the client disconnects.
if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
    log.Fatal(err)
}
```

For convenience, we provide `Server.Run` to handle the common case of running a session until the client disconnects:

```go
func (*Server) Run(context.Context, Transport)
```

**Differences from mcp-go**: the Server APIs are similar to mcp-go, though the association between servers and transports is different. In mcp-go, a single server is bound to what we would call an `SSEHTTPHandler`, and reused for all sessions. Per-session behavior is implemented though a 'session tool' overlay. As discussed above, the transport abstraction here is differentiated from HTTP serving, and the `Server.Connect` method provides a consistent API for binding to an arbitrary transport. Servers here do not have methods for sending notifications or calls, because they are logically distinct from the `ServerSession`. In mcp-go, servers are `n:1`, but there is no abstraction of a server session: sessions are addressed in Server APIs through their `sessionID`: `SendNotificationToAllClients`, `SendNotificationToClient`, `SendNotificationToSpecificClient`.

The client API here is different, since clients and client sessions are conceptually distinct. The `ClientSession` is closer to mcp-go's notion of Client.

For both clients and servers, mcp-go uses variadic options to customize behavior, whereas an options struct is used here. We felt that in this case, an options struct would be more readable, and result in simpler package documentation.

### Spec Methods

In our SDK, RPC methods that are defined in the specification take a context and a params pointer as arguments, and return a result pointer and error:

```go
func (*ClientSession) ListTools(context.Context, *ListToolsParams) (*ListToolsResult, error)
```

Our SDK has a method for every RPC in the spec, their signatures all share this form. We do this, rather than providing more convenient shortcut signatures, to maintain backward compatibility if the spec makes backward-compatible changes such as adding a new property to the request parameters (as in [this commit](https://github.com/modelcontextprotocol/modelcontextprotocol/commit/2fce8a077688bf8011e80af06348b8fe1dae08ac), for example). To avoid boilerplate, we don't repeat this signature for RPCs defined in the spec; readers may assume it when we mention a "spec method."

`CallTool` is the only exception: for convenience when binding to Go argument types, `*CallToolParams[TArgs]` is generic, with a type parameter providing the Go type of the tool arguments. The spec method accepts a `*CallToolParams[json.RawMessage]`, but we provide a generic helper function. See the section on Tools below for details.

Why do we use params instead of the full JSON-RPC request? As much as possible, we endeavor to hide JSON-RPC details when they are not relevant to the business logic of your client or server. In this case, the additional information in the JSON-RPC request is just the request ID and method name; the request ID is irrelevant, and the method name is implied by the name of the Go method providing the API.

We believe that any change to the spec that would require callers to pass a new a parameter is not backward compatible. Therefore, it will always work to pass `nil` for any `XXXParams` argument that isn't currently necessary. For example, it is okay to call `Ping` like so:

```go
err := session.Ping(ctx, nil)
```

#### Iterator Methods

For convenience, iterator methods handle pagination for the `List` spec methods automatically, traversing all pages. If Params are supplied, iteration begins from the provided cursor (if present).

```go
func (*ClientSession) Tools(context.Context, *ListToolsParams) iter.Seq2[Tool, error]

func (*ClientSession) Prompts(context.Context, *ListPromptsParams) iter.Seq2[Prompt, error]

func (*ClientSession) Resources(context.Context, *ListResourceParams) iter.Seq2[Resource, error]

func (*ClientSession) ResourceTemplates(context.Context, *ListResourceTemplatesParams) iter.Seq2[ResourceTemplate, error]
```

### Middleware

We provide a mechanism to add MCP-level middleware on the both the client and server side. Receiving middleware runs after the request has been parsed but before any normal handling. It is analogous to traditional HTTP server middleware. Sending middleware runs after a call to a method but before the request is sent. It is an alternative to transport middleware that exposes MCP types instead of raw JSON-RPC 2.0 messages. It is useful for tracing and setting progress tokens, for example.

```go
// A MethodHandler handles MCP messages.
// For methods, exactly one of the return values must be nil.
// For notifications, both must be nil.
type MethodHandler func(ctx context.Context, method string, req Request) (result Result, err error)

// Middleware is a function from MethodHandlers to MethodHandlers.
type Middleware func(MethodHandler) MethodHandler

// AddMiddleware wraps the client/server's current method handler using the provided
// middleware. Middleware is applied from right to left, so that the first one
// is executed first.
//
// For example, AddMiddleware(m1, m2, m3) augments the server method handler as
// m1(m2(m3(handler))).
func (c *Client) AddSendingMiddleware(middleware ...Middleware)
func (c *Client) AddReceivingMiddleware(middleware ...Middleware)
func (s *Server) AddSendingMiddleware(middleware ...Middleware)
func (s *Server) AddReceivingMiddleware(middleware ...Middleware)
```

As an example, this code adds server-side logging:

```go
func withLogging(h mcp.MethodHandler) mcp.MethodHandler{
    return func(ctx context.Context, method string, req mcp.Request) (res mcp.Result, err error) {
        log.Printf("request: %s %v", method, params)
        defer func() { log.Printf("response: %v, %v", res, err) }()
        return h(ctx, s , method, params)
    }
}

server.AddReceivingMiddleware(withLogging)
```

**Differences from mcp-go**: Version 0.26.0 of mcp-go defines 24 server hooks. Each hook consists of a field in the `Hooks` struct, a `Hooks.Add` method, and a type for the hook function. These are rarely used. The most common is `OnError`, which occurs fewer than ten times in open-source code.

#### Rate Limiting

Rate limiting can be configured using middleware. Please see [examples/rate-limiting](<https://github.com/modelcontextprotocol/go-sdk/tree/main/examples/rate-limiting>) for an example on how to implement this.

### Errors

With the exception of tool handler errors, protocol errors are handled transparently as Go errors: errors in server-side feature handlers are propagated as errors from calls from the `ClientSession`, and vice-versa.

Protocol errors wrap a `JSONRPCError` type which exposes its underlying error code.

```go
type JSONRPCError struct {
	Code    int64           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}
```

As described by the [spec](https://modelcontextprotocol.io/specification/2025-03-26/server/tools#error-handling), tool execution errors are reported in tool results.

**Differences from mcp-go**: the `JSONRPCError` type here does not include ID and Method, which can be inferred from the caller. Otherwise, this behavior is similar.

### Cancellation

Cancellation is implemented transparently using context cancellation. The user can cancel an operation by cancelling the associated context:

```go
ctx, cancel := context.WithCancel(ctx)
go session.CallTool(ctx, "slow", map[string]any{}, nil)
cancel()
```

When this client call is cancelled, a `"notifications/cancelled"` notification is sent to the server. However, the client call returns immediately with `ctx.Err()`: it does not wait for the result from the server.

The server observes a client cancellation as a cancelled context.

### Progress handling

A caller can request progress notifications by setting the `Meta.ProgressToken` field on any request.

```go
type XXXParams struct { // where XXX is each type of call
  Meta Meta
  ...
}

type Meta struct {
  Data          map[string]any
  ProgressToken any // string or int
}
```

Handlers can notify their peer about progress by calling the `NotifyProgress` method. The notification is only sent if the peer requested it by providing a progress token.

```go
func (*ClientSession) NotifyProgress(context.Context, *ProgressNotification)
func (*ServerSession) NotifyProgress(context.Context, *ProgressNotification)
```

### Ping / KeepAlive

Both `ClientSession` and `ServerSession` expose a `Ping` method to call "ping" on their peer.

```go
func (c *ClientSession) Ping(ctx context.Context, *PingParams) error
func (c *ServerSession) Ping(ctx context.Context, *PingParams) error
```

Additionally, client and server sessions can be configured with automatic keepalive behavior. If the `KeepAlive` option is set to a non-zero duration, it defines an interval for regular "ping" requests. If the peer fails to respond to pings originating from the keepalive check, the session is automatically closed.

```go
type ClientOptions struct {
  ...
  KeepAlive time.Duration
}

type ServerOptions struct {
  ...
  KeepAlive time.Duration
}
```

**Differences from mcp-go**: in mcp-go the `Ping` method is only provided for client, not server, and the keepalive option is only provided for SSE servers (as a variadic option).

## Client Features

### Roots

Clients support the MCP Roots feature, including roots-changed notifications. Roots can be added and removed from a `Client` with `AddRoots` and `RemoveRoots`:

```go
// AddRoots adds the given roots to the client,
// replacing any with the same URIs,
// and notifies any connected servers.
func (*Client) AddRoots(roots ...*Root)

// RemoveRoots removes the roots with the given URIs.
// and notifies any connected servers if the list has changed.
// It is not an error to remove a nonexistent root.
func (*Client) RemoveRoots(uris ...string)
```

Server sessions can call the spec method `ListRoots` to get the roots. If a server installs a `RootsChangedHandler`, it will be called when the client sends a roots-changed notification, which happens whenever the list of roots changes after a connection has been established.

```go
type ServerOptions {
  ...
  // If non-nil, called when a client sends a roots-changed notification.
  RootsChangedHandler func(context.Context, *ServerSession, *RootsChangedParams)
}
```

The `Roots` method provides a [cached](https://modelcontextprotocol.io/specification/2025-03-26/client/roots#implementation-guidelines) iterator of the root set, invalidated when roots change.

```go
func (*ServerSession) Roots(context.Context) (iter.Seq[*Root, error])
```

### Sampling

Clients that support sampling are created with a `CreateMessageHandler` option for handling server calls. To perform sampling, a server session calls the spec method `CreateMessage`.

```go
type ClientOptions struct {
  ...
  CreateMessageHandler func(context.Context, *ClientSession, *CreateMessageParams) (*CreateMessageResult, error)
}
```

## Server Features

### Tools

A `Tool` is a logical MCP tool, generated from the MCP spec.

A tool handler accepts `CallToolParams` and returns a `CallToolResult`. However, since we want to bind tools to Go input types, it is convenient in associated APIs to have a generic version of `CallToolParams`, with a type parameter `In` for the tool argument type, as well as a generic version of for `CallToolResult`. This allows tool APIs to manage the marshalling and unmarshalling of tool inputs for their caller.

```go
type CallToolParamsFor[In any] struct {
	Meta      Meta   `json:"_meta,omitempty"`
	Arguments In     `json:"arguments,omitempty"`
	Name      string `json:"name"`
}

type Tool struct {
	Annotations *ToolAnnotations   `json:"annotations,omitempty"`
	Description string             `json:"description,omitempty"`
	InputSchema *jsonschema.Schema `json:"inputSchema"`
	Name string                    `json:"name"`
}

// A ToolHandlerFor handles a call to tools/call with typed arguments and results.
type ToolHandlerFor[In, Out any] func(context.Context, *ServerRequest[*CallToolParamsFor[In]]) (*CallToolResultFor[Out], error)

```

Add tools to a server with the `AddTool` method or function. The function is generic and infers schemas from the handler
arguments:

```go
func (s *Server) AddTool(t *Tool, h ToolHandler)
func AddTool[In, Out any](s *Server, t *Tool, h ToolHandlerFor[In, Out])
```

```go
mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "add numbers"}, addHandler)
mcp.AddTool(server, &mcp.Tool{Name: "subtract", Description: "subtract numbers"}, subHandler)
```

The `AddTool` method requires an input schema, and optionally an output one. It will not modify them.
The handler should accept a `CallToolParams` and return a `CallToolResult`.
```go
t := &Tool{Name: ..., Description: ..., InputSchema: &jsonschema.Schema{...}}
server.AddTool(t, myHandler)
```

Tools can be removed by name with `RemoveTools`:

```go
server.RemoveTools("add", "subtract")
```

A tool's input schema, expressed as a [JSON Schema](https://json-schema.org), provides a way to validate the tool's input. One of the challenges in defining tools is the need to associate them with a Go function, yet support the arbitrary complexity of JSON Schema. To achieve this, we have seen two primary approaches:

1. Use reflection to generate the tool's input schema from a Go type (à la `metoro-io/mcp-golang`)
2. Explicitly build the input schema (à la `mark3labs/mcp-go`).

Both of these have their advantages and disadvantages. Reflection is nice, because it allows you to bind directly to a Go API, and means that the JSON schema of your API is compatible with your Go types by construction. It also means that concerns like parsing and validation can be handled automatically. However, it can become cumbersome to express the full breadth of JSON schema using Go types or struct tags, and sometimes you want to express things that aren’t naturally modeled by Go types, like unions. Explicit schemas are simple and readable, and give the caller full control over their tool definition, but involve significant boilerplate.

We provide both ways. The `jsonschema.For[T]` function will infer a schema, and it is called by the `AddTool` generic function.
Users can also call it themselves, or build a schema directly as a struct literal. They can still use the `AddTool` function to
create a typed handler, since `AddTool` doesn't touch schemas that are already present.


If the tool's `InputSchema` is nil, it is inferred from the `In` type parameter. If the `OutputSchema` is nil, it is inferred from the `Out` type parameter (unless `Out` is `any`).

For example, given this handler:
```go
type AddParams struct {
    X int `json:"x"`
    Y int `json:"y"`
}

func addHandler(ctx context.Context, req *mcp.ServerRequest[*mcp.CallToolParamsFor[AddParams]]) (*mcp.CallToolResultFor[int], error) {
    return &mcp.CallToolResultFor[int]{StructuredContent: req.Params.Arguments.X + req.Params.Arguments.Y}, nil
}
```

You can add it to a server like this:
```go
mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "add numbers"}, addHandler)
```
The input schema will be inferred from `AddParams`, and the output schema from `int`.

Since all the fields of the Tool struct are exported, a Tool can also be created directly with assignment or a struct literal.

Client sessions can call the spec method `ListTools` or an iterator method `Tools` to list the available tools, and use spec method `CallTool` to call tools. Similar to `ServerTool.Handler`, `CallTool` expects `*CallToolParams[json.RawMessage]`, but we provide a generic `CallTool` helper to operate on typed arguments.

```go
func (cs *ClientSession) CallTool(context.Context, *CallToolParams[json.RawMessage]) (*CallToolResult, error)
```

**Differences from mcp-go**: We provide a full JSON Schema implementation for validating tool input schemas against incoming arguments. The `jsonschema.Schema` type provides exported features for all keywords in the JSON Schema draft2020-12 spec. Tool definers can use it to construct any schema they want. The `jsonschema.For[T]` function can infer a schema from a Go struct. These combined features eliminate the need for variadic arguments to construct tool schemas.

For registering tools, we provide only a `Server.AddTool` method; mcp-go's `SetTools`, `AddTool`, `AddSessionTool`, and `AddSessionTools` are deemed unnecessary. (Similarly for Delete/Remove). The `AddTool` generic function combines schema inference with registration, providing a easy way to register many tools.

### Prompts

Use `AddPrompt` to add a prompt to the server, and `RemovePrompts`
to remove them by name.

```go
type codeReviewArgs struct {
  Code string `json:"code"`
}

func codeReviewHandler(context.Context, *ServerSession, *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {...}

server.AddPrompt(
  &mcp.Prompt{Name: "code_review", Description: "review code"},
  codeReviewHandler,
)

server.RemovePrompts("code_review")
```

Client sessions can call the spec method `ListPrompts` or the iterator method `Prompts` to list the available prompts, and the spec method `GetPrompt` to get one.

**Differences from mcp-go**: We provide `RemovePrompts` to remove prompts from the server.

### Resources and resource templates

In our design, each resource and resource template is associated with a function that reads it, with this signature:

```go
type ResourceHandler func(context.Context, *ServerSession, *ReadResourceParams) (*ReadResourceResult, error)
```

The arguments include the `ServerSession` so the handler can observe the client's roots. The handler should return the resource contents in a `ReadResourceResult`, calling either `NewTextResourceContents` or `NewBlobResourceContents`. If the handler omits the URI or MIME type, the server will populate them from the resource.

To add a resource or resource template to a server, users call the `AddResource` and `AddResourceTemplate` methods. We also provide methods to remove them.

```go
func (*Server) AddResource(*Resource, ResourceHandler)
func (*Server) AddResourceTemplate(*ResourceTemplate, ResourceHandler)

func (s *Server) RemoveResources(uris ...string)
func (s *Server) RemoveResourceTemplates(uriTemplates ...string)
```

The `ReadResource` method finds a resource or resource template matching the argument URI and calls its associated handler.

Server sessions also support the spec methods `ListResources` and `ListResourceTemplates`, and the corresponding iterator methods `Resources` and `ResourceTemplates`.

**Differences from mcp-go**: The `ResourceHandler` returns a `ReadResourceResult`, rather than just its content, for compatibility with future evolution of the spec.

#### Subscriptions

##### Client-Side Usage

Use the Subscribe and Unsubscribe methods on a ClientSession to start or stop receiving updates for a specific resource.

```go
func (*ClientSession) Subscribe(context.Context, *SubscribeParams) error
func (*ClientSession) Unsubscribe(context.Context, *UnsubscribeParams) error
```

To process incoming update notifications, you must provide a ResourceUpdatedHandler in your ClientOptions. The SDK calls this function automatically whenever the server sends a notification for a resource you're subscribed to.

```go
type ClientOptions struct {
  ...
  ResourceUpdatedHandler func(context.Context, *ClientSession, *ResourceUpdatedNotificationParams)
}
```

##### Server-Side Implementation

The server does not implement resource subscriptions. It passes along subscription requests to the user, and supplies a method to notify clients of changes. It tracks which sessions have subscribed to which resources so the user doesn't have to.

If a server author wants to support resource subscriptions, they must provide handlers to be called when clients subscribe and unsubscribe. It is an error to provide only one of these handlers.

```go
type ServerOptions struct {
  ...
	// Function called when a client session subscribes to a resource.
	SubscribeHandler func(context.Context, *ServerRequest[*SubscribeParams]) error
	// Function called when a client session unsubscribes from a resource.
	UnsubscribeHandler func(context.Context, *ServerRequest[*UnsubscribeParams]) error
}
```

User code should call `ResourceUpdated` when a subscribed resource changes.

```go
func (*Server) ResourceUpdated(context.Context, *ResourceUpdatedNotificationParams) error
```

The server routes these notifications to the server sessions that subscribed to the resource.

### ListChanged notifications

When a list of tools, prompts or resources changes as the result of an AddXXX or RemoveXXX call, the server informs all its connected clients by sending the corresponding type of notification. A client will receive these notifications if it was created with the corresponding option:

```go
type ClientOptions struct {
  ...
	ToolListChangedHandler      func(context.Context, *ClientRequest[*ToolListChangedParams])
	PromptListChangedHandler    func(context.Context, *ClientRequest[*PromptListChangedParams])
  // For both resources and resource templates.
	ResourceListChangedHandler  func(context.Context, *ClientRequest[*ResourceListChangedParams])
}
```

**Differences from mcp-go**: mcp-go instead provides a general `OnNotification` handler. For type-safety, and to hide JSON RPC details, we provide feature-specific handlers here.

### Completion

Clients call the spec method `Complete` to request completions. If a server installs a `CompletionHandler`, it will be called when the client sends a completion request.

```go
type ServerOptions struct {
  ...
  // If non-nil, called when a client sends a completion request.
	CompletionHandler func(context.Context, *ServerRequest[*CompleteParams]) (*CompleteResult, error)
}
```

#### Security Considerations

Implementors of the CompletionHandler MUST adhere to the following security guidelines:

- **Validate all completion inputs**: The CompleteRequest received by the handler may contain arbitrary data from the client. Before processing, thoroughly validate all fields.

- **Implement appropriate rate limiting**: Completion requests can be high-frequency, especially in interactive user experiences. Without rate limiting, a malicious client could potentially overload the server, leading to denial-of-service (DoS) attacks. Consider applying rate limits per client session, IP address, or API key, depending on your deployment model. This can be implemented within the CompletionHandler itself or via middleware (see [Middleware](#middleware)) that wraps the handler.

- **Control access to sensitive suggestions**: Completion suggestions should only expose information that the requesting client is authorized to access. If your completion logic draws from sensitive data sources (e.g., internal file paths, user data, restricted API endpoints), ensure that the CompletionHandler performs proper authorization checks before generating or returning suggestions. This might involve checking the ServerSession context for authentication details or consulting an external authorization system.

- **Prevent completion-based information disclosure**: Be mindful that even seemingly innocuous completion suggestions can inadvertently reveal sensitive information. For example, suggesting internal system paths or confidential identifiers could be an attack vector. Ensure that the generated CompleteResult contains only appropriate and non-sensitive information for the given client and context. Avoid revealing internal data structures or error messages that could aid an attacker.

**Differences from mcp-go**: the client API is similar. mcp-go has not yet defined its server-side behavior.

### Logging

MCP specifies a notification for servers to log to clients. Server sessions implement this with the `LoggingMessage` method. It honors the minimum log level established by the client session's `SetLevel` call.

As a convenience, we also provide a `slog.Handler` that allows server authors to write logs with the `log/slog` package::

```go
// A LoggingHandler is a [slog.Handler] for MCP.
type LoggingHandler struct {...}

// LoggingHandlerOptions are options for a LoggingHandler.
type LoggingHandlerOptions struct {
	// The value for the "logger" field of logging notifications.
	LoggerName string
	// Limits the rate at which log messages are sent.
	// If zero, there is no rate limiting.
	MinInterval time.Duration
}

// NewLoggingHandler creates a [LoggingHandler] that logs to the given [ServerSession] using a
// [slog.JSONHandler].
func NewLoggingHandler(ss *ServerSession, opts *LoggingHandlerOptions) *LoggingHandler
```

Server-to-client logging is configured with `ServerOptions`:

```go
type ServerOptions {
  ...
  // The value for the "logger" field of the notification.
  LoggerName string
  // Log notifications to a single ClientSession will not be
  // sent more frequently than this duration.
  LoggingInterval time.Duration
}
```

A call to a log method like `Info` is translated to a `LoggingMessageNotification` as follows:

- The attributes and the message populate the "data" property with the output of a `slog.JSONHandler`: The result is always a JSON object, with the key "msg" for the message.

- If the `LoggerName` server option is set, it populates the "logger" property.

- The standard slog levels `Info`, `Debug`, `Warn` and `Error` map to the corresponding levels in the MCP spec. The other spec levels map to integers between the slog levels. For example, "notice" is level 2 because it is between "warning" (slog value 4) and "info" (slog value 0). The `mcp` package defines consts for these levels. To log at the "notice" level, a handler would call `Log(ctx, mcp.LevelNotice, "message")`.

A client that wishes to receive log messages must provide a handler:

```go
type ClientOptions struct {
  ...
  LoggingMessageHandler func(context.Context, *ClientSession, *LoggingMessageParams)
}
```

### Pagination

Servers initiate pagination for `ListTools`, `ListPrompts`, `ListResources`, and `ListResourceTemplates`, dictating the page size and providing a `NextCursor` field in the Result if more pages exist. The SDK implements keyset pagination, using the unique ID of the feature as the key for a stable sort order and encoding the cursor as an opaque string.

For server implementations, the page size for the list operation may be configured via the `ServerOptions.PageSize` field. PageSize must be a non-negative integer. If zero, a sensible default is used.

```go
type ServerOptions {
  ...
  PageSize int
}
```

Client requests for List methods include an optional Cursor field for pagination. Server responses for List methods include a `NextCursor` field if more pages exist.

In addition to the `List` methods, the SDK provides an iterator method for each list operation. This simplifies pagination for clients by automatically handling the underlying pagination logic. See [Iterator Methods](#iterator-methods) above.

**Differences with mcp-go**: the PageSize configuration is set with a configuration field rather than a variadic option. Additionally, this design proposes pagination by default, as this is likely desirable for most servers

# Governance and Community

While the sections above propose an initial implementation of the Go SDK, MCP is evolving rapidly. SDKs need to keep pace, by implementing changes to the spec, fixing bugs, and accommodating new and emerging use-cases. This section proposes how the SDK project can be managed so that it can change safely and transparently.

Initially, the Go SDK repository will be administered by the Go team and Anthropic, and they will be the Approvers (the set of people able to merge PRs to the SDK). The policies here are also intended to satisfy necessary constraints of the Go team's participation in the project.

The content in this section will also be included in a CONTRIBUTING.md file in the repo root.

## Hosting, copyright, and license

The SDK will be hosted under github.com/modelcontextprotocol/go-sdk, MIT license, copyright "Go SDK Authors". Each Go file in the repository will have a standard copyright header. For example:

```go
// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
```

## Issues and Contributing

The SDK will use its GitHub issue tracker for bug tracking, and pull requests for contributions.

Contributions to the SDK will be welcomed, and will be accepted provided they are high quality and consistent with the direction and philosophy of the SDK outlined above. An official SDK must be conservative in the changes it accepts, to defend against compatibility problems, security vulnerabilities, and churn. To avoid being declined, PRs should be associated with open issues, and those issues should either be labeled 'Help Wanted', or the PR author should ask on the issue before contributing.

### Proposals

A proposal is an issue that proposes a new API for the SDK, or a change to the signature or behavior of an existing API. Proposals will be labeled with the 'Proposal' label, and require an explicit approval before being accepted (applied through the 'Proposal-Accepted' label). Proposals will remain open for at least a week to allow discussion before being accepted or declined by an Approver.

Proposals that are straightforward and uncontroversial may be approved based on GitHub discussion. However, proposals that are deemed to be sufficiently unclear or complicated will be deferred to a regular steering meeting (see below).

This process is similar to the [Go proposal process](https://github.com/golang/proposal), but is necessarily lighter weight to accommodate the greater rate of change expected for the SDK.

### Steering meetings

On a regular basis, we will host a virtual steering meeting to discuss outstanding proposals and other changes to the SDK. These 1hr meetings and their agenda will be announced in advance, and open to all to join. The meetings will be recorded, and recordings and meeting notes will be made available afterward.

This process is similar to the [Go Tools call](https://go.dev/wiki/golang-tools), though it is expected that meetings will at least initially occur on a more frequent basis (likely biweekly).

### Discord

Discord (either the public or private Anthropic discord servers) should only be used for logistical coordination or answering questions. Design discussion and decisions should occur in GitHub issues or public steering meetings.

### Antitrust considerations

It is important that the SDK avoids bias toward specific integration paths or providers. Therefore, the CONTRIBUTING.md file will include an antitrust policy that outlines terms and practices intended to avoid such bias, or the appearance thereof. (The details of this policy will be determined by Google and Anthropic lawyers).

## Releases and Versioning

The SDK will consist of a single Go module, and will be released through versioned Git tags. Accordingly, it will follow semantic versioning.

Up until the v1.0.0 release, the SDK may be unstable and may change in breaking ways. An initial v1.0.0 release will occur when the SDK is deemed by Approvers to be stable, production ready, and sufficiently complete (though some unimplemented features may remain). Subsequent to that release, new APIs will be added in minor versions, and breaking changes will require a v2 release of the module (and therefore should be avoided). All releases will have corresponding release notes in GitHub.

It is desirable that releases occur frequently, and that a v1.0.0 release is achieved as quickly as possible.

If feasible, the SDK will support all versions of the MCP spec. However, if breaking changes to the spec make this infeasible, preference will be given to the most recent version of the MCP spec.

## Ongoing evaluation

On an ongoing basis, the administrators of the SDK will evaluate whether it is keeping pace with changes to the MCP spec and meeting its goals of openness and transparency. If it is not meeting these goals, either because it exceeds the bandwidth of its current Approvers, or because the processes here are inadequate, these processes will be re-evaluated. At this time, the Approvers set may be expanded to include additional community members, based on their history of strong contribution.
