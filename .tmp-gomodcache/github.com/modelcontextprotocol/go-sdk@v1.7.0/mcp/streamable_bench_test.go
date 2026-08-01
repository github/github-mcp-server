// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp_test

import (
	"flag"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func BenchmarkStreamableServing(b *testing.B) {
	// This benchmark measures how fast we can handle a single tool on a
	// streamable server, including tool validation and stream management.
	customSchemas := map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[Probability](): {Type: "number", Minimum: jsonschema.Ptr(0.0), Maximum: jsonschema.Ptr(1.0)},
		reflect.TypeFor[WeatherType](): {Type: "string", Enum: []any{Sunny, PartlyCloudy, Cloudy, Rainy, Snowy}},
	}
	opts := &jsonschema.ForOptions{TypeSchemas: customSchemas}
	in, err := jsonschema.For[WeatherInput](opts)
	if err != nil {
		b.Fatal(err)
	}
	out, err := jsonschema.For[WeatherOutput](opts)
	if err != nil {
		b.Fatal(err)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:         "weather",
		InputSchema:  in,
		OutputSchema: out,
	}, WeatherTool)

	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	ctx := b.Context()
	session, err := mcp.NewClient(testImpl, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer session.Close()
	b.ResetTimer()
	for range b.N {
		if _, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "weather",
			Arguments: WeatherInput{
				Location: Location{Name: "somewhere"},
				Days:     7,
			},
		}); err != nil {
			b.Errorf("CallTool failed: %v", err)
		}
	}
}

var streamableHeap = flag.String("streamable_heap", "", "if set, write streamable heap profiles with this prefix")

func BenchmarkStreamableServing_BadSessions(b *testing.B) {
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)

	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})

	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	ctx := b.Context()

	if *streamableHeap != "" {
		writeHeap := func(file string) {
			// GC a couple times to ensure accurate heap.
			runtime.GC()
			runtime.GC()
			f, err := os.Create(file)
			if err != nil {
				log.Fatal("could not create memory profile: ", err)
			}
			defer func() {
				if err := f.Close(); err != nil {
					b.Errorf("writing heap file %q: %v", file, err)
				}
			}()
			if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
				b.Errorf("could not write heap profile: %v", err)
			}
		}
		writeHeap(*streamableHeap + ".before")
		defer writeHeap(*streamableHeap + ".after")
	}

	b.ResetTimer()
	for range b.N {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, httpServer.URL, strings.NewReader("{}"))
		if err != nil {
			b.Fatal(err)
		}
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Accept", "text/event-stream")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		if got, want := resp.StatusCode, http.StatusBadRequest; got != want {
			b.Fatalf("POST got status %d, want %d", got, want)
		}
		if got := resp.Header.Get("Mcp-Session-Id"); got != "" {
			b.Fatalf("POST got unexpected session ID")
		}
		resp.Body.Close()
	}
}
