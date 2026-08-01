// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import "time"

// This file exports some helpers for mutating internals of the command
// transport for testing.

// SetDefaultTerminateDuration sets the default command terminate duration,
// and returns a function to reset it to the default.
func SetDefaultTerminateDuration(d time.Duration) (reset func()) {
	initial := defaultTerminateDuration
	defaultTerminateDuration = d
	return func() {
		defaultTerminateDuration = initial
	}
}
