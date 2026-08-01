// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// IOTransport is a simplified implementation of a transport that communicates using
// newline-delimited JSON over an io.Reader and io.Writer. It is similar to ioTransport
// in transport.go and serves as a demonstration of how to implement a custom transport.
type IOTransport struct {
	r *bufio.Reader
	w io.Writer
}

// NewIOTransport creates a new IOTransport with the given io.Reader and io.Writer.
func NewIOTransport(r io.Reader, w io.Writer) *IOTransport {
	return &IOTransport{
		r: bufio.NewReader(r),
		w: w,
	}
}

// ioConn is a connection that uses newlines to delimit messages. It implements [mcp.Connection].
type ioConn struct {
	r *bufio.Reader
	w io.Writer
}

// Connect implements [mcp.Transport.Connect] by creating a new ioConn.
func (t *IOTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	return &ioConn{
		r: t.r,
		w: t.w,
	}, nil
}

// Read implements [mcp.Connection.Read], assuming messages are newline-delimited JSON.
func (t *ioConn) Read(context.Context) (jsonrpc.Message, error) {
	data, err := t.r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	return jsonrpc.DecodeMessage(data[:len(data)-1])
}

// Write implements [mcp.Connection.Write], appending a newline delimiter after the message.
func (t *ioConn) Write(_ context.Context, msg jsonrpc.Message) error {
	data, err := jsonrpc.EncodeMessage(msg)
	if err != nil {
		return err
	}

	_, err1 := t.w.Write(data)
	_, err2 := t.w.Write([]byte{'\n'})
	return errors.Join(err1, err2)
}

// Close implements [mcp.Connection.Close]. Since this is a simplified example, it is a no-op.
func (t *ioConn) Close() error {
	return nil
}

// SessionID implements [mcp.Connection.SessionID]. Since this is a simplified example,
// it returns an empty session ID.
func (t *ioConn) SessionID() string {
	return ""
}

// HiArgs is the argument type for the SayHi tool.
type HiArgs struct {
	Name string `json:"name" mcp:"the name to say hi to"`
}

// SayHi is a tool handler that responds with a greeting.
func SayHi(ctx context.Context, req *mcp.CallToolRequest, args HiArgs) (*mcp.CallToolResult, struct{}, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Hi " + args.Name},
		},
	}, struct{}{}, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "greeter"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "greet", Description: "say hi"}, SayHi)

	// Run the server with a custom IOTransport using stdio as the io.Reader and io.Writer.
	transport := &IOTransport{
		r: bufio.NewReader(os.Stdin),
		w: os.Stdout,
	}
	err := server.Run(context.Background(), transport)
	if err != nil {
		log.Println("[ERROR]: Failed to run server:", err)
	}
}
