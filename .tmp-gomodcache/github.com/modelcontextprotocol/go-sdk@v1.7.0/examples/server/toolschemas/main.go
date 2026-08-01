// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// The toolschemas example demonstrates how to create tools using both the
// low-level [ToolHandler] and high level [ToolHandlerFor], as well as how to
// customize schemas in both cases.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input is the input into all the tools handlers below.
type Input struct {
	Name string `json:"name" jsonschema:"the person to greet"`
}

// Output is the structured output of the tool.
//
// Not every tool needs to have structured output.
type Output struct {
	Greeting string `json:"greeting" jsonschema:"the greeting to send to the user"`
}

// simpleGreeting is an [mcp.ToolHandlerFor] that only cares about input and output.
func simpleGreeting(_ context.Context, _ *mcp.CallToolRequest, input Input) (*mcp.CallToolResult, Output, error) {
	return nil, Output{"Hi " + input.Name}, nil
}

// manualGreeter handles the parsing and validation of input and output manually.
//
// Therefore, it needs to close over its resolved schemas, to use them in
// validation.
type manualGreeter struct {
	inputSchema  *jsonschema.Resolved
	outputSchema *jsonschema.Resolved
}

func (t *manualGreeter) greet(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// errf produces a 'tool error', embedding the error in a CallToolResult.
	errf := func(format string, args ...any) *mcp.CallToolResult {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}},
			IsError: true,
		}
	}
	// Handle the parsing and validation of input and output.
	//
	// Note that errors here are treated as tool errors, not protocol errors.

	// First, unmarshal to a map[string]any and validate.
	if err := unmarshalAndValidate(req.Params.Arguments, t.inputSchema); err != nil {
		return errf("invalid input: %v", err), nil
	}

	// Now unmarshal again to input.
	var input Input
	if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
		return errf("failed to unmarshal arguments: %v", err), nil
	}
	output := Output{Greeting: "Hi " + input.Name}
	outputJSON, err := json.Marshal(output)
	if err != nil {
		return errf("output failed to marshal: %v", err), nil
	}
	//
	if err := unmarshalAndValidate(outputJSON, t.outputSchema); err != nil {
		return errf("invalid output: %v", err), nil
	}

	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(outputJSON)}},
		StructuredContent: output,
	}, nil
}

// unmarshalAndValidate unmarshals data to a map[string]any, then validates that against res.
func unmarshalAndValidate(data []byte, res *jsonschema.Resolved) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	return res.Validate(m)
}

var (
	inputSchema = &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"name": {Type: "string", MaxLength: jsonschema.Ptr(10)},
		},
	}
	outputSchema = &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"greeting": {Type: "string"},
		},
	}
)

func newManualGreeter() (*manualGreeter, error) {
	resIn, err := inputSchema.Resolve(nil)
	if err != nil {
		return nil, err
	}
	resOut, err := outputSchema.Resolve(nil)
	if err != nil {
		return nil, err
	}
	return &manualGreeter{
		inputSchema:  resIn,
		outputSchema: resOut,
	}, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "greeter"}, nil)

	// Add the 'greeting' tool in a few different ways.

	// First, we can just use [mcp.AddTool], and get the out-of-the-box handling
	// it provides for schema inference, validation, parsing, and packing the
	// result.
	mcp.AddTool(server, &mcp.Tool{Name: "simple greeting"}, simpleGreeting)

	// Alternatively, we can create our schemas entirely manually, and add them
	// using [mcp.Server.AddTool]. Since we're using the 'raw' API, we have to do
	// the parsing and validation ourselves
	manual, err := newManualGreeter()
	if err != nil {
		log.Fatal(err)
	}
	server.AddTool(&mcp.Tool{
		Name:         "manual greeting",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	}, manual.greet)

	// We can even use raw schema values. In this case, note that we're not
	// validating the input at all.
	server.AddTool(&mcp.Tool{
		Name:        "unvalidated greeting",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"user":{"type":"string"}}}`),
	}, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Note: no validation!
		var args struct{ User string }
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Hi " + args.User}},
		}, nil
	})

	// Finally, note that we can also use custom schemas with a ToolHandlerFor.
	// We can do this in two ways: by using one of the schema values constructed
	// above, or by using jsonschema.For and adjusting the resulting schema.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "customized greeting 1",
		InputSchema: inputSchema,
		// OutputSchema will still be derived from Output.
	}, simpleGreeting)

	customSchema, err := jsonschema.For[Input](nil)
	if err != nil {
		log.Fatal(err)
	}
	customSchema.Properties["name"].MaxLength = jsonschema.Ptr(10)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "customized greeting 2",
		InputSchema: customSchema,
	}, simpleGreeting)

	// Now run the server.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("Server failed: %v", err)
	}
}
