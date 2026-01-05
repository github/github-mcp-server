// Package scopes provides OAuth scope checking utilities for GitHub MCP Server.
//
// This file contains utilities for filtering tools based on token scopes.
// For PATs, we cannot issue OAuth scope challenges, so we hide tools that
// require scopes the token doesn't have.
//
package scopes
