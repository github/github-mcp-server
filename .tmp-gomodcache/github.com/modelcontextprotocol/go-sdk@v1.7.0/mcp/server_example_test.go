// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//lint:file-ignore SA1019 examples exercise deprecated SEP-2577 logging APIs for demonstration.

package mcp_test

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"sync/atomic"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// !+prompts

func Example_prompts() {
	ctx := context.Background()

	promptHandler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "Hi prompt",
			Messages: []*mcp.PromptMessage{
				{
					Role:    "user",
					Content: &mcp.TextContent{Text: "Say hi to " + req.Params.Arguments["name"]},
				},
			},
		}, nil
	}

	// Create a server with a single prompt.
	s := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	prompt := &mcp.Prompt{
		Name: "greet",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "name",
				Description: "the name of the person to greet",
				Required:    true,
			},
		},
	}
	s.AddPrompt(prompt, promptHandler)

	// Create a client.
	c := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, nil)

	// Connect the server and client.
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := s.Connect(ctx, t1, nil); err != nil {
		log.Fatal(err)
	}
	cs, err := c.Connect(ctx, t2, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer cs.Close()

	// List the prompts.
	for p, err := range cs.Prompts(ctx, nil) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(p.Name)
	}

	// Get the prompt.
	res, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      "greet",
		Arguments: map[string]string{"name": "Pat"},
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, msg := range res.Messages {
		fmt.Println(msg.Role, msg.Content.(*mcp.TextContent).Text)
	}
	// Output:
	// greet
	// user Say hi to Pat
}

// !-prompts

// !+logging

func Example_logging() {
	ctx := context.Background()

	// Create a server.
	s := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)

	// Create a client that displays log messages.
	done := make(chan struct{}) // solely for the example
	var nmsgs atomic.Int32
	c := mcp.NewClient(
		&mcp.Implementation{Name: "client", Version: "v0.0.1"},
		&mcp.ClientOptions{
			LoggingMessageHandler: func(_ context.Context, r *mcp.LoggingMessageRequest) {
				m := r.Params.Data.(map[string]any)
				fmt.Println(m["msg"], m["value"])
				if nmsgs.Add(1) == 2 { // number depends on logger calls below
					close(done)
				}
			},
		})

	// Connect the server and client.
	t1, t2 := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, t1, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ss.Close()
	cs, err := c.Connect(ctx, t2, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer cs.Close()

	// Set the minimum log level to "info".
	if err := cs.SetLoggingLevel(ctx, &mcp.SetLoggingLevelParams{Level: "info"}); err != nil {
		log.Fatal(err)
	}

	// Get a slog.Logger for the server session.
	logger := slog.New(mcp.NewLoggingHandler(ss, nil))

	// Log some things.
	logger.Info("info shows up", "value", 1)
	logger.Debug("debug doesn't show up", "value", 2)
	logger.Warn("warn shows up", "value", 3)

	// Wait for them to arrive on the client.
	// In a real application, the log messages would appear asynchronously
	// while other work was happening.
	<-done

	// Output:
	// info shows up 1
	// warn shows up 3
}

// !-logging

// !+resources
func Example_resources() {
	ctx := context.Background()

	resources := map[string]string{
		"file:///a":     "a",
		"file:///dir/x": "x",
		"file:///dir/y": "y",
	}

	handler := func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		c, ok := resources[uri]
		if !ok {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: uri, Text: c}},
		}, nil
	}

	// Create a server with a single resource.
	s := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	s.AddResource(&mcp.Resource{URI: "file:///a"}, handler)
	s.AddResourceTemplate(&mcp.ResourceTemplate{URITemplate: "file:///dir/{f}"}, handler)

	// Create a client.
	c := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, nil)

	// Connect the server and client.
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := s.Connect(ctx, t1, nil); err != nil {
		log.Fatal(err)
	}
	cs, err := c.Connect(ctx, t2, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer cs.Close()

	// List resources and resource templates.
	for r, err := range cs.Resources(ctx, nil) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(r.URI)
	}
	for r, err := range cs.ResourceTemplates(ctx, nil) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(r.URITemplate)
	}

	// Read resources.
	for _, path := range []string{"a", "dir/x", "b"} {
		res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "file:///" + path})
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(res.Contents[0].Text)
		}
	}
	// Output:
	// file:///a
	// file:///dir/{f}
	// a
	// x
	// calling "resources/read": Resource not found
}

// !-resources

// !+mrtr

// Example_mrtr demonstrates the [Multi Round-Trip Requests] pattern
// (SEP-2322). A tool handler signals "I need more information from the user"
// by returning a [mcp.CallToolResult] whose `InputRequests` field carries the
// requests for the additional information. Each request can be an
// [mcp.ElicitParams] (ask the user a question),
// [mcp.CreateMessageParams] (sample from the client's LLM), or
// [mcp.ListRootsParams] (list the client's roots).
//
// On protocol version `2026-07-28` and later, the client's
// `clientMultiRoundTripMiddleware` fulfils each input request from the
// configured handler and retries the original call transparently. On earlier
// protocol versions, the server's `serverMultiRoundTripMiddleware` performs
// the equivalent dance from the server side by calling the legacy
// server-to-client API (`ServerSession.Elicit`, `CreateMessage`, or
// `ListRoots`) and re-invoking the handler with the response. Either way,
// the user-facing client call site sees only the final result.
//
// [Multi Round-Trip Requests]: https://modelcontextprotocol.io/specification/draft/basic/patterns#multi-round-trip-requests
func Example_mrtr() {
	ctx := context.Background()

	// Server: a "greet" tool that asks the user for their name before
	// returning a greeting. The handler runs twice per logical call:
	// once to issue the elicitation, once to consume the response.
	s := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "greet"}, func(_ context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		if len(req.Params.InputResponses) == 0 {
			// First call: ask the user for their name. The map key
			// ("user_name") is a label the handler chooses; the client must
			// echo it back in InputResponses. The wire-level method name
			// ("elicitation/create") is derived from the value's Go type.
			return &mcp.CallToolResult{
				InputRequests: mcp.InputRequestMap{
					"user_name": &mcp.ElicitParams{
						Message: "What's your name?",
						RequestedSchema: &jsonschema.Schema{
							Type: "object",
							Properties: map[string]*jsonschema.Schema{
								"name": {Type: "string"},
							},
						},
					},
				},
				// RequestState is an opaque token the client echoes back on
				// the retry, letting the handler resume its work without
				// per-session storage.
				RequestState: "step=1",
			}, nil, nil
		}
		// Retry: read the elicitation response and produce the final greeting.
		name := req.Params.InputResponses["user_name"].(*mcp.ElicitResult).Content["name"].(string)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Hello " + name}},
		}, nil, nil
	})

	// Client: declares an elicitation handler. The SDK's MRTR middleware
	// uses it to fulfil any input request the tool handler returns.
	c := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, &mcp.ClientOptions{
		ElicitationHandler: func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "accept", Content: map[string]any{"name": "MCP Go"}}, nil
		},
	})

	ct, st := mcp.NewInMemoryTransports()
	if _, err := s.Connect(ctx, st, nil); err != nil {
		log.Fatal(err)
	}
	cs, err := c.Connect(ctx, ct, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer cs.Close()

	// Single call site. The MRTR round trip is invisible to the caller.
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "greet"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.Content[0].(*mcp.TextContent).Text)
}

// !-mrtr
