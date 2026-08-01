// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	httpAddr       = flag.String("http", "", "if set, use streamable HTTP at this address, instead of stdin/stdout")
	memoryFilePath = flag.String("memory", "", "if set, persist the knowledge base to this file; otherwise, it will be stored in memory and lost on exit")
)

// HiArgs defines arguments for the greeting tool.
type HiArgs struct {
	Name string `json:"name"`
}

// CreateEntitiesArgs defines the create entities tool parameters.
type CreateEntitiesArgs struct {
	Entities []Entity `json:"entities" mcp:"entities to create"`
}

// CreateEntitiesResult returns newly created entities.
type CreateEntitiesResult struct {
	Entities []Entity `json:"entities"`
}

// CreateRelationsArgs defines the create relations tool parameters.
type CreateRelationsArgs struct {
	Relations []Relation `json:"relations" mcp:"relations to create"`
}

// CreateRelationsResult returns newly created relations.
type CreateRelationsResult struct {
	Relations []Relation `json:"relations"`
}

// AddObservationsArgs defines the add observations tool parameters.
type AddObservationsArgs struct {
	Observations []Observation `json:"observations" mcp:"observations to add"`
}

// AddObservationsResult returns newly added observations.
type AddObservationsResult struct {
	Observations []Observation `json:"observations"`
}

// DeleteEntitiesArgs defines the delete entities tool parameters.
type DeleteEntitiesArgs struct {
	EntityNames []string `json:"entityNames" mcp:"entities to delete"`
}

// DeleteObservationsArgs defines the delete observations tool parameters.
type DeleteObservationsArgs struct {
	Deletions []Observation `json:"deletions" mcp:"observations to delete"`
}

// DeleteRelationsArgs defines the delete relations tool parameters.
type DeleteRelationsArgs struct {
	Relations []Relation `json:"relations" mcp:"relations to delete"`
}

// SearchNodesArgs defines the search nodes tool parameters.
type SearchNodesArgs struct {
	Query string `json:"query" mcp:"query string"`
}

// OpenNodesArgs defines the open nodes tool parameters.
type OpenNodesArgs struct {
	Names []string `json:"names" mcp:"names of nodes to open"`
}

func main() {
	flag.Parse()

	// Initialize storage backend
	var kbStore store
	kbStore = &memoryStore{}
	if *memoryFilePath != "" {
		kbStore = &fileStore{path: *memoryFilePath}
	}
	kb := knowledgeBase{s: kbStore}

	// Setup MCP server with knowledge base tools
	server := mcp.NewServer(&mcp.Implementation{Name: "memory"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_entities",
		Description: "Create multiple new entities in the knowledge graph",
	}, kb.CreateEntities)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_relations",
		Description: "Create multiple new relations between entities",
	}, kb.CreateRelations)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_observations",
		Description: "Add new observations to existing entities",
	}, kb.AddObservations)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_entities",
		Description: "Remove entities and their relations",
	}, kb.DeleteEntities)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_observations",
		Description: "Remove specific observations from entities",
	}, kb.DeleteObservations)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_relations",
		Description: "Remove specific relations from the graph",
	}, kb.DeleteRelations)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_graph",
		Description: "Read the entire knowledge graph",
	}, kb.ReadGraph)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_nodes",
		Description: "Search for nodes based on query",
	}, kb.SearchNodes)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "open_nodes",
		Description: "Retrieve specific nodes by name",
	}, kb.OpenNodes)

	// Start server with appropriate transport
	if *httpAddr != "" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return server
		}, nil)
		log.Printf("MCP handler listening at %s", *httpAddr)
		http.ListenAndServe(*httpAddr, handler)
	} else {
		t := &mcp.LoggingTransport{Transport: &mcp.StdioTransport{}, Writer: os.Stderr}
		if err := server.Run(context.Background(), t); err != nil {
			log.Printf("Server failed: %v", err)
		}
	}
}
