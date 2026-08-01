// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"encoding/json"

	internaljson "github.com/modelcontextprotocol/go-sdk/internal/json"
)

func assert(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}

// remarshal marshals from to JSON, and then unmarshals into to, which must be
// a pointer type.
func remarshal(from, to any) error {
	data, err := json.Marshal(from)
	if err != nil {
		return err
	}
	if err := internaljson.Unmarshal(data, to); err != nil {
		return err
	}
	return nil
}
