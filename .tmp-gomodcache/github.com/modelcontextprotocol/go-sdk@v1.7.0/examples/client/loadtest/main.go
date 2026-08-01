// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// The load command load tests a streamable MCP server
//
// Usage: loadtest <URL>
//
// For example:
//
//	loadtest -tool=greet -args='{"name": "foo"}' http://localhost:8080
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	duration = flag.Duration("duration", 1*time.Minute, "duration of the load test")
	tool     = flag.String("tool", "", "tool to call")
	jsonArgs = flag.String("args", "", "JSON arguments to pass")
	workers  = flag.Int("workers", 10, "number of concurrent workers")
	timeout  = flag.Duration("timeout", 1*time.Second, "request timeout")
	qps      = flag.Int("qps", 100, "tool calls per second, per worker")
	verbose  = flag.Bool("v", false, "if set, enable verbose logging")
	cleanup  = flag.Bool("cleanup", true, "whether to clean up sessions at the end of the test")
)

func main() {
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage: loadtest [flags] <URL>")
		fmt.Fprintf(out, "Load test a streamable HTTP server (CTRL-C to end early)")
		fmt.Fprintln(out)
		fmt.Fprintf(out, "Example: loadtest -tool=greet -args='{\"name\": \"foo\"}' http://localhost:8080\n")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Flags:")
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 || *tool == "" {
		flag.Usage()
		os.Exit(2)
	}

	parentCtx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()
	parentCtx, stop := signal.NotifyContext(parentCtx, os.Interrupt)
	defer stop()

	var (
		start   = time.Now()
		success atomic.Int64
		failure atomic.Int64
	)

	// Run the test.
	var wg sync.WaitGroup
	for range *workers {
		wg.Go(func() {
			client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)
			cs, err := client.Connect(parentCtx, &mcp.StreamableClientTransport{Endpoint: args[0]}, nil)
			if err != nil {
				log.Fatal(err)
			}
			if *cleanup {
				defer cs.Close()
			}

			ticker := time.NewTicker(1 * time.Second / time.Duration(*qps))
			defer ticker.Stop()

			for range ticker.C {
				ctx, cancel := context.WithTimeout(parentCtx, *timeout)
				defer cancel()

				res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: *tool, Arguments: json.RawMessage(*jsonArgs)})
				if err != nil {
					if parentCtx.Err() != nil {
						return // test ended
					}
					failure.Add(1)
					if *verbose {
						log.Printf("FAILURE: %v", err)
					}
				} else {
					success.Add(1)
					if *verbose {
						data, err := json.Marshal(res)
						if err != nil {
							log.Fatalf("marshalling result: %v", err)
						}
						log.Printf("SUCCESS: %s", string(data))
					}
				}
			}
		})
	}
	wg.Wait()
	stop() // restore the interrupt signal

	// Print stats.
	var (
		dur  = time.Since(start)
		succ = success.Load()
		fail = failure.Load()
	)
	fmt.Printf("Results (in %s):\n", dur)
	fmt.Printf("\tsuccess: %d (%g QPS)\n", succ, float64(succ)/dur.Seconds())
	fmt.Printf("\tfailure: %d (%g QPS)\n", fail, float64(fail)/dur.Seconds())
}
