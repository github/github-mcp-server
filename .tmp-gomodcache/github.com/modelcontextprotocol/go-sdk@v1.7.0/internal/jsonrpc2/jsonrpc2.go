// Copyright 2018 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package jsonrpc2 is a minimal implementation of the JSON RPC 2 spec.
// https://www.jsonrpc.org/specification
// It is intended to be compatible with other implementations at the wire level.
package jsonrpc2

import (
	"context"
	"errors"
)

// ErrNotHandled is returned from a Handler or Preempter to indicate it did
// not handle the request.
//
// If a Handler returns ErrNotHandled, the server replies with
// ErrMethodNotFound.
var ErrNotHandled = errors.New("JSON RPC not handled")

// Preempter handles messages on a connection before they are queued to the main
// handler.
// Primarily this is used for cancel handlers or notifications for which out of
// order processing is not an issue.
type Preempter interface {
	// Preempt is invoked for each incoming request before it is queued for handling.
	//
	// If Preempt returns ErrNotHandled, the request will be queued,
	// and eventually passed to a Handle call.
	//
	// Otherwise, the result and error are processed as if returned by Handle.
	//
	// Preempt must not block. (The Context passed to it is for Values only.)
	Preempt(ctx context.Context, req *Request) (result any, err error)
}

// Handler handles messages on a connection.
type Handler interface {
	// Handle is invoked sequentially for each incoming request that has not
	// already been handled by a Preempter.
	//
	// If the Request has a nil ID, Handle must return a nil result,
	// and any error may be logged but will not be reported to the caller.
	//
	// If the Request has a non-nil ID, Handle must return either a
	// non-nil, JSON-marshalable result, or a non-nil error.
	//
	// The Context passed to Handle will be canceled if the
	// connection is broken or the request is canceled or completed.
	// (If Handle returns ErrAsyncResponse, ctx will remain uncanceled
	// until either Cancel or Respond is called for the request's ID.)
	Handle(ctx context.Context, req *Request) (result any, err error)
}

// A HandlerFunc implements the Handler interface for a standalone Handle function.
type HandlerFunc func(ctx context.Context, req *Request) (any, error)

func (f HandlerFunc) Handle(ctx context.Context, req *Request) (any, error) {
	return f(ctx, req)
}

var _ Handler = HandlerFunc(nil)
