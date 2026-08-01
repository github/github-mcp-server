// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// !+loggingtransport

func ExampleLoggingTransport() {
	ctx := context.Background()
	t1, t2 := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	serverSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, nil)
	var b bytes.Buffer
	logTransport := &mcp.LoggingTransport{Transport: t2, Writer: &b}
	clientSession, err := client.Connect(ctx, logTransport, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer clientSession.Close()

	// Sort for stability: reads are concurrent to writes.
	for _, line := range slices.Sorted(strings.SplitSeq(b.String(), "\n")) {
		fmt.Println(line)
	}

	// Output:
	// read: {"jsonrpc":"2.0","id":1,"result":{"resultType":"complete","_meta":{"io.modelcontextprotocol/serverInfo":{"name":"server","version":"v0.0.1"}},"ttlMs":0,"cacheScope":"public","supportedVersions":["2026-07-28","2025-11-25","2025-06-18","2025-03-26","2024-11-05"],"capabilities":{"logging":{}}}}
	// write: {"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/clientCapabilities":{"roots":{"listChanged":true}},"io.modelcontextprotocol/clientInfo":{"name":"client","version":"v0.0.1"},"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}
}

// !-loggingtransport
