// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// The distributed command is an example of a distributed MCP server.
//
// It forks multiple child processes (according to the -child_ports flag), each
// of which is a streamable HTTP MCP server with the 'inc' tool, and proxies
// incoming http requests to them.
//
// Distributed MCP servers must be stateless, because there's no guarantee that
// subsequent requests for a session land on the same backend. However, they
// may still have logical session IDs, as can be seen with verbose logging
// (-v).
//
// Example:
//
//	./distributed -http=localhost:8080 -child_ports=8081,8082
//
//lint:file-ignore SA1019 example server exercises deprecated SEP-2577 logging API for demonstration.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const childPortVar = "MCP_CHILD_PORT"

var (
	httpAddr   = flag.String("http", "", "if set, use streamable HTTP at this address, instead of stdin/stdout")
	childPorts = flag.String("child_ports", "", "comma-separated child ports to distribute to")
	verbose    = flag.Bool("v", false, "if set, enable verbose logging")
)

func main() {
	// This command runs as either a parent or a child, depending on whether
	// childPortVar is set (a.k.a. the fork-and-exec trick).
	//
	// Each child is a streamable HTTP server, and the parent is a reverse proxy.
	flag.Parse()
	if v := os.Getenv(childPortVar); v != "" {
		child(v)
	} else {
		parent()
	}
}

func parent() {
	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}

	if *httpAddr == "" {
		log.Fatal("must provide -http")
	}
	if *childPorts == "" {
		log.Fatal("must provide -child_ports")
	}

	// Ensure that children are cleaned up on CTRL-C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start the child processes.
	ports := strings.Split(*childPorts, ",")
	var wg sync.WaitGroup
	childURLs := make([]*url.URL, len(ports))
	for i, port := range ports {
		childURL := fmt.Sprintf("http://localhost:%s", port)
		childURLs[i], err = url.Parse(childURL)
		if err != nil {
			log.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, exe, os.Args[1:]...)
		cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", childPortVar, port))
		cmd.Stderr = os.Stderr

		wg.Go(func() {
			log.Printf("starting child %d at %s", i, childURL)
			if err := cmd.Run(); err != nil && ctx.Err() == nil {
				log.Printf("child %d failed: %v", i, err)
			} else {
				log.Printf("child %d exited gracefully", i)
			}
		})
	}

	// Start a reverse proxy that round-robin's requests to each backend.
	var nextBackend atomic.Int64
	server := http.Server{
		Addr: *httpAddr,
		Handler: &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				child := int(nextBackend.Add(1)) % len(childURLs)
				if *verbose {
					log.Printf("dispatching to localhost:%s", ports[child])
				}
				r.SetURL(childURLs[child])
			},
		},
	}

	wg.Go(func() {
		if err := server.ListenAndServe(); err != nil && ctx.Err() == nil {
			log.Printf("Server failed: %v", err)
		}
	})

	log.Printf("Serving at %s (CTRL-C to cancel)", *httpAddr)

	<-ctx.Done()
	stop() // restore the interrupt signal

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt the graceful shutdown.
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	// Wait for the subprocesses and http server to stop.
	wg.Wait()

	log.Println("Server shutdown gracefully.")
}

func child(port string) {
	// Create a server with a single tool that increments a counter and sends a notification.
	server := mcp.NewServer(&mcp.Implementation{Name: "counter"}, nil)

	var count atomic.Int64
	inc := func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, struct{ Count int64 }, error) {
		n := count.Add(1)
		if *verbose {
			log.Printf("request %d (session %s)", n, req.Session.ID())
		}
		// Send a notification in the context of the request
		// Hint: in stateless mode, at least log level 'info' is required to send notifications
		req.Session.Log(ctx, &mcp.LoggingMessageParams{Data: fmt.Sprintf("request %d (session %s)", n, req.Session.ID()), Level: "info"})
		return nil, struct{ Count int64 }{n}, nil
	}
	mcp.AddTool(server, &mcp.Tool{Name: "inc"}, inc)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless: true,
	})
	log.Printf("child listening on localhost:%s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("localhost:%s", port), handler))
}
