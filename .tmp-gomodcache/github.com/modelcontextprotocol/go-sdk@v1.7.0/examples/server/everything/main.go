// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// The everything server implements all supported features of an MCP server.
//
//lint:file-ignore SA1019 example server exercises deprecated SEP-2577 APIs (roots, sampling, logging) for demonstration.
package main

import (
	"context"
	_ "embed"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	httpAddr  = flag.String("http", "", "if set, use streamable HTTP at this address, instead of stdin/stdout")
	pprofAddr = flag.String("pprof", "", "if set, host the pprof debugging server at this address")
)

func main() {
	flag.Parse()

	if *pprofAddr != "" {
		// For debugging memory leaks, add an endpoint to trigger a few garbage
		// collection cycles and ensure the /debug/pprof/heap endpoint only reports
		// reachable memory.
		http.DefaultServeMux.Handle("/gc", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			for range 3 {
				runtime.GC()
			}
			fmt.Fprintln(w, "GC'ed")
		}))
		go func() {
			// DefaultServeMux was mutated by the /debug/pprof import.
			http.ListenAndServe(*pprofAddr, http.DefaultServeMux)
		}()
	}

	opts := &mcp.ServerOptions{
		Instructions:      "Use this server!",
		CompletionHandler: complete, // support completions by setting this handler
	}

	// Add an icon to the server implementation.
	icons := mcpIcons()
	server := mcp.NewServer(&mcp.Implementation{Name: "everything", WebsiteURL: "https://example.com", Icons: icons}, opts)

	// Add tools that exercise different features of the protocol.
	mcp.AddTool(server, &mcp.Tool{Name: "greet", Description: "say hi"}, contentTool)
	mcp.AddTool(server, &mcp.Tool{Name: "greet (structured)"}, structuredTool)                                // returns structured output
	mcp.AddTool(server, &mcp.Tool{Name: "greet (with Icons)", Icons: icons}, structuredTool)                  // tool with icons
	mcp.AddTool(server, &mcp.Tool{Name: "greet (content with ResourceLink)"}, resourceLinkContentTool(icons)) // tool that returns content with a resource link
	mcp.AddTool(server, &mcp.Tool{Name: "ping"}, pingingTool)                                                 // performs a ping
	mcp.AddTool(server, &mcp.Tool{Name: "log"}, loggingTool)                                                  // performs a log
	mcp.AddTool(server, &mcp.Tool{Name: "sample"}, samplingTool)                                              // performs sampling
	mcp.AddTool(server, &mcp.Tool{Name: "elicit (form)"}, elicitFormTool)                                     // performs form elicitation
	mcp.AddTool(server, &mcp.Tool{Name: "elicit (url)"}, elicitURLTool)                                       // performs url elicitation
	mcp.AddTool(server, &mcp.Tool{Name: "roots"}, rootsTool)                                                  // lists roots

	// Add a basic prompt.
	server.AddPrompt(&mcp.Prompt{Name: "greet"}, prompt)
	server.AddPrompt(&mcp.Prompt{Name: "greet (with Icons)", Icons: icons}, prompt) // greet prompt with icons

	// Add an embedded resource.
	server.AddResource(&mcp.Resource{
		Name:     "info",
		MIMEType: "text/plain",
		URI:      "embedded:info",
	}, embeddedResource)
	server.AddResource(&mcp.Resource{ // text resource with icons
		Name:     "info (with Icons)",
		MIMEType: "text/plain",
		URI:      "embedded:info",
		Icons:    icons,
	}, embeddedResource)

	// Add a resource template.
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "Resource template (with Icon)",
		MIMEType:    "text/plain",
		URITemplate: "http://example.com/~{resource_name}/",
		Icons:       icons,
	}, embeddedResource)

	// Serve over stdio, or streamable HTTP if -http is set.
	if *httpAddr != "" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return server
		}, nil)
		log.Printf("MCP handler listening at %s", *httpAddr)
		if *pprofAddr != "" {
			log.Printf("pprof listening at http://%s/debug/pprof", *pprofAddr)
		}
		log.Fatal(http.ListenAndServe(*httpAddr, handler))
	} else {
		t := &mcp.LoggingTransport{Transport: &mcp.StdioTransport{}, Writer: os.Stderr}
		if err := server.Run(context.Background(), t); err != nil {
			log.Printf("Server failed: %v", err)
		}
	}
}

func prompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
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

var embeddedResources = map[string]string{
	"info": "This is the hello example server.",
}

func embeddedResource(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	u, err := url.Parse(req.Params.URI)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "embedded" {
		return nil, fmt.Errorf("wrong scheme: %q", u.Scheme)
	}
	key := u.Opaque
	text, ok := embeddedResources[key]
	if !ok {
		return nil, fmt.Errorf("no embedded resource named %q", key)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: req.Params.URI, MIMEType: "text/plain", Text: text},
		},
	}, nil
}

type args struct {
	Name string `json:"name" jsonschema:"the name to say hi to"`
}

// contentTool is a tool that returns unstructured content.
//
// Since its output type is 'any', no output schema is created.
func contentTool(ctx context.Context, req *mcp.CallToolRequest, args args) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Hi " + args.Name},
		},
	}, nil, nil
}

// resourceLinkContentTool returns a ResourceLink content with icons.
func resourceLinkContentTool(icons []mcp.Icon) func(ctx context.Context, req *mcp.CallToolRequest, args args) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args args) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.ResourceLink{
					Name:     "greeting",
					Title:    "A friendly greeting",
					MIMEType: "text/plain",
					URI:      "data:text/plain,Hi%20" + url.PathEscape(args.Name),
					Icons:    icons,
				},
			},
		}, nil, nil
	}
}

type result struct {
	Message string `json:"message" jsonschema:"the message to convey"`
}

// structuredTool returns a structured result.
func structuredTool(ctx context.Context, req *mcp.CallToolRequest, args *args) (*mcp.CallToolResult, *result, error) {
	return nil, &result{Message: "Hi " + args.Name}, nil
}

func pingingTool(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	if err := req.Session.Ping(ctx, nil); err != nil {
		return nil, nil, fmt.Errorf("ping failed")
	}
	return nil, nil, nil
}

func loggingTool(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	if err := req.Session.Log(ctx, &mcp.LoggingMessageParams{
		Data:  "something happened!",
		Level: "error",
	}); err != nil {
		return nil, nil, fmt.Errorf("log failed")
	}
	return nil, nil, nil
}

func rootsTool(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	res, err := req.Session.ListRoots(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("listing roots failed: %v", err)
	}
	var allroots []string
	for _, r := range res.Roots {
		allroots = append(allroots, fmt.Sprintf("%s:%s", r.Name, r.URI))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: strings.Join(allroots, ",")},
		},
	}, nil, nil
}

func samplingTool(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	res, err := req.Session.CreateMessage(ctx, new(mcp.CreateMessageParams))
	if err != nil {
		return nil, nil, fmt.Errorf("sampling failed: %v", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			res.Content,
		},
	}, nil, nil
}

func elicitFormTool(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	res, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
		Message: "provide a random string",
		RequestedSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"random": {Type: "string"},
			},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("eliciting failed: %v", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: res.Content["random"].(string)},
		},
	}, nil, nil
}

var elicitations atomic.Int32

func elicitURLTool(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	elicitID := fmt.Sprintf("%d", elicitations.Add(1))
	_, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
		Message:       "submit a string",
		URL:           fmt.Sprintf("http://localhost:6062?id=%s", elicitID),
		ElicitationID: elicitID,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("eliciting failed: %v", err)
	}
	// TODO: actually wait for the elicitation form to be submitted.
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "(elicitation pending)"},
		},
	}, nil, nil
}

func complete(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return &mcp.CompleteResult{
		Completion: mcp.CompletionResultDetails{
			Total:  1,
			Values: []string{req.Params.Argument.Value + "x"},
		},
	}, nil
}

//go:embed mcp.png
var mcpIconData []byte

func mcpIcons() []mcp.Icon {
	return []mcp.Icon{{
		Source:   "data:image/png;base64," + base64.StdEncoding.EncodeToString(mcpIconData),
		MIMEType: "image/png",
		Sizes:    []string{"48x48"},
		Theme:    mcp.IconThemeLight,
	}}
}
