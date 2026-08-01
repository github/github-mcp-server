# MCP Go SDK

[![Open in GitHub Codespaces](https://github.com/codespaces/badge.svg)](https://codespaces.new/modelcontextprotocol/go-sdk)

[![PkgGoDev](https://pkg.go.dev/badge/github.com/modelcontextprotocol/go-sdk)](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/modelcontextprotocol/go-sdk/badge)](https://scorecard.dev/viewer/?uri=github.com/modelcontextprotocol/go-sdk)

This repository contains an implementation of the official Go software
development kit (SDK) for the Model Context Protocol (MCP).

## Package / Feature documentation

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

The SDK endeavors to implement the full MCP spec. The [`docs/`](/docs/) directory
contains feature documentation, mapping the MCP spec to the packages above.

## Version Compatibility

The following table shows which versions of the Go SDK support which versions of the MCP specification:

| SDK Version     | Latest MCP Spec   | All Supported MCP Specs                                          |
|-----------------|-------------------|------------------------------------------------------------------|
| v1.7.0+         | 2026-07-28       | 2026-07-28, 2025-11-25\*, 2025-06-18, 2025-03-26, 2024-11-05    |
| v1.4.0 - v1.6.1 | 2025-11-25\*      | 2025-11-25\*, 2025-06-18, 2025-03-26, 2024-11-05                 |
| v1.2.0 - v1.3.1 | 2025-11-25\*\*    | 2025-11-25\*\*, 2025-06-18, 2025-03-26, 2024-11-05               |
| v1.0.0 - v1.1.0 | 2025-06-18        | 2025-06-18, 2025-03-26, 2024-11-05                               |

\* Client side OAuth has experimental support.

\*\* Partial support for 2025-11-25 (client side OAuth and Sampling with tools not available).

The roots, sampling, and logging features are deprecated as of protocol version
2026-07-28 by
[SEP-2577](https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging).
The SDK continues to support them for compatibility during the deprecation
window (at least twelve months). See the individual feature documentation for
migration guidance.

New releases of the SDK target only supported versions of Go. See
https://go.dev/doc/devel/release#policy for more information.

## Getting started

To get started creating an MCP server, create an `mcp.Server` instance, add
features to it, and then run it over an `mcp.Transport`. For example, this
server adds a single simple tool, and then connects it to clients over
stdin/stdout:

%include server/server.go -

To communicate with that server, create an `mcp.Client` and connect it to the
corresponding server, by running the server command and communicating over its
stdin/stdout:

%include client/client.go -

The [`examples/`](/examples/) directory contains more example clients and
servers.

## Contributing

We welcome contributions to the SDK! Please see
[CONTRIBUTING.md](/CONTRIBUTING.md) for details of how to contribute.

## Acknowledgements / Alternatives

Several third party Go MCP SDKs inspired the development and design of this
official SDK, and continue to be viable alternatives, notably
[mcp-go](https://github.com/mark3labs/mcp-go), originally authored by Ed Zynda.
We are grateful to Ed as well as the other contributors to mcp-go, and to
authors and contributors of other SDKs such as
[mcp-golang](https://github.com/metoro-io/mcp-golang) and
[go-mcp](https://github.com/ThinkInAIXYZ/go-mcp). Thanks to their work, there
is a thriving ecosystem of Go MCP clients and servers.

## License

This project is licensed under Apache 2.0 for new contributions, with existing
code under MIT - see the [LICENSE](./LICENSE) file for details.
