# AGENTS.md

## Project Overview

This repository contains the official Go SDK for the Model Context Protocol (MCP).
The SDK is designed to be idiomatic, future-proof, and extensible.

### Key Packages

-   `mcp`: The core package defining the primary APIs for constructing and using MCP clients and servers. This is where most logic resides.
-   `jsonrpc`: Provides the JSON-RPC 2.0 transport layer. Use this if implementing custom transports.
-   `auth`: Primitives for supporting OAuth.
-   `oauthex`: Extensions to the OAuth protocol, such as Protected Resource Metadata.
-   `internal`: Internal implementation details not exposed to users.
-   `examples`: Example clients and servers. Use these as references for usage patterns.

## Development Setup

The project uses the standard Go toolchain.

-   **Build**: `go build ./...`
-   **Test**: `go test ./...`

## Testing

-   **Unit Tests**: Run `go test ./...` to run all unit tests.
-   **Conformance Tests**: Use the following scripts to run the official MCP conformance tests against the SDK.
    -   `./scripts/server-conformance.sh` for server tests.
    -   `./scripts/client-conformance.sh` for client tests.
    -   The scripts download the latest conformance suite from npm by default.
    -   To get possible options pass the `--help` flag to the script.

## Development Guidelines

### Code Style

-   Follow standard Go conventions (Effective Go).
-   Use `gofmt` to format code.
-   Add copyright headers to all new Go files:
    ```go
    // Copyright 2025 The Go MCP SDK Authors. All rights reserved.
    // Use of this source code is governed by the license
    // that can be found in the LICENSE file.
    ```
-  Do not add comments to the code unless they are really necessary:
    -   Prefer self-documenting code.
    -   Focus on the "why" not the "what" in comments.

### Documentation

-   **README.md**: Do NOT edit `README.md` directly. It is generated from `internal/readme/README.src.md`.
    -   Edit `internal/readme/README.src.md`.
    -   Run `go generate ./internal/readme` to regenerate.
    -   Commit both files.
-   **docs/**: Do NOT edit `docs/` directory directly. It is generated from files in `internal/docs`.
    -   Edit `internal/docs/*.src.md`.
    -   Run `go generate ./internal/docs` to regenerate.
    -   Commit files from both directories.
