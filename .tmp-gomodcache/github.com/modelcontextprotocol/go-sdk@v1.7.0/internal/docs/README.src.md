# Overview

The SDK consists of several importable packages:

- The
  [`github.com/modelcontextprotocol/go-sdk/mcp`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp)
  package defines the primary APIs for constructing and using MCP clients and
  servers.
- The
  [`github.com/modelcontextprotocol/go-sdk/jsonrpc`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/jsonrpc) package is for users implementing
  their own transports.
- The
  [`github.com/modelcontextprotocol/go-sdk/auth`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/auth)
  package provides some primitives for supporting OAuth.
- The
  [`github.com/modelcontextprotocol/go-sdk/oauthex`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/oauthex)
  package provides extensions to the OAuth protocol, such as ProtectedResourceMetadata.


These docs mirror the [official MCP spec](https://modelcontextprotocol.io/specification/2025-06-18).
Use the index below to learn how the SDK implements a particular aspect of the
protocol.

## Base Protocol

1. [Lifecycle (Clients, Servers, and Sessions)](protocol.md#lifecycle).
1. [Transports](protocol.md#transports)
    1. [Stdio transport](protocol.md#stdio-transport)
    1. [Streamable transport](protocol.md#streamable-transport)
    1. [Custom transports](protocol.md#stateless-mode)
1. [Authorization](protocol.md#authorization)
1. [Security](protocol.md#security)
1. [Utilities](protocol.md#utilities)
    1. [Cancellation](protocol.md#cancellation)
    1. [Ping](protocol.md#ping)
    1. [Progress](protocol.md#progress)

## Client Features

1. [Roots](client.md#roots)
1. [Sampling](client.md#sampling)
1. [Elicitation](client.md#elicitation)

## Server Features

1. [Prompts](server.md#prompts)
1. [Resources](server.md#resources)
1. [Tools](server.md#tools)
1. [Utilities](server.md#utilities)
    1. [Completion](server.md#completion)
    1. [Logging](server.md#logging)
    1. [Pagination](server.md#pagination)

# TroubleShooting

See [troubleshooting.md](troubleshooting.md) for a troubleshooting guide.

# Backwards compatibility

See [mcpgodebug.md](mcpgodebug.md) for a list of backwards incompatible behavior changes
and description how they can be temporarily undone.

# Rough edges

See [rough_edges.md](rough_edges.md) for a list of rough edges or API
oversights that can't be addressed due to our compatibility promise. We'll
revisit these if/when we move to a v2 of the SDK.
