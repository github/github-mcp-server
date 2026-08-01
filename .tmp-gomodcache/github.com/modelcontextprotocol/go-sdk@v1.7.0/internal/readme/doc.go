// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//go:generate -command weave go run golang.org/x/example/internal/cmd/weave@latest
//go:generate weave -o ../../README.md ./README.src.md
//go:generate weave -o ../../CONTRIBUTING.md ./contributing.src.md

// The readme package is used to generate README.md at the top-level of this
// repo. Regenerate the README with go generate.
package readme
