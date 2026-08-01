// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//go:generate -command weave go run golang.org/x/example/internal/cmd/weave@latest
//go:generate weave -o ../../docs/README.md ./README.src.md
//go:generate weave -o ../../docs/protocol.md ./protocol.src.md
//go:generate weave -o ../../docs/client.md ./client.src.md
//go:generate weave -o ../../docs/server.md ./server.src.md
//go:generate weave -o ../../docs/troubleshooting.md ./troubleshooting.src.md
//go:generate weave -o ../../docs/rough_edges.md ./rough_edges.src.md
//go:generate weave -o ../../docs/mcpgodebug.md ./mcpgodebug.src.md
//go:generate weave -o ../../docs/quick_start.md ./quick_start.src.md

// The doc package generates the documentation at /doc, via go:generate.
//
// Tests in this package are used for examples.
package docs
