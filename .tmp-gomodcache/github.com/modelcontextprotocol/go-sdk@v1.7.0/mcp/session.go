// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

// hasSessionID is the interface which, if implemented by connections, informs
// the session about their session ID.
//
// TODO(rfindley): remove SessionID methods from connections, when it doesn't
// make sense. Or remove it from the Sessions entirely: why does it even need
// to be exposed?
type hasSessionID interface {
	SessionID() string
}

// ServerSessionState is the state of a session.
type ServerSessionState struct {
	// InitializeParams are the parameters from 'initialize'.
	InitializeParams *InitializeParams `json:"initializeParams"`

	// InitializedParams are the parameters from 'notifications/initialized'.
	InitializedParams *InitializedParams `json:"initializedParams"`

	// LogLevel is the logging level for the session.
	LogLevel LoggingLevel `json:"logLevel"`

	// TODO: resource subscriptions
}
