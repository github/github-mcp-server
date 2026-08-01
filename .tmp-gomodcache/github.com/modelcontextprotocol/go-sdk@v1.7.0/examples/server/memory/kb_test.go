// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// stores provides test factories for both storage implementations.
func stores() map[string]func(t *testing.T) store {
	return map[string]func(t *testing.T) store{
		"file": func(t *testing.T) store {
			tempDir, err := os.MkdirTemp("", "kb-test-file-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			t.Cleanup(func() { os.RemoveAll(tempDir) })
			return &fileStore{path: filepath.Join(tempDir, "test-memory.json")}
		},
		"memory": func(t *testing.T) store {
			return &memoryStore{}
		},
	}
}

// TestKnowledgeBaseOperations verifies CRUD operations work correctly.
func TestKnowledgeBaseOperations(t *testing.T) {
	for name, newStore := range stores() {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			kb := knowledgeBase{s: s}

			// Verify empty graph loads correctly
			graph, err := kb.loadGraph()
			if err != nil {
				t.Fatalf("failed to load empty graph: %v", err)
			}
			if len(graph.Entities) != 0 || len(graph.Relations) != 0 {
				t.Errorf("expected empty graph, got %+v", graph)
			}

			// Create and verify entities
			testEntities := []Entity{
				{
					Name:         "Alice",
					EntityType:   "Person",
					Observations: []string{"Likes coffee"},
				},
				{
					Name:         "Bob",
					EntityType:   "Person",
					Observations: []string{"Likes tea"},
				},
			}

			createdEntities, err := kb.createEntities(testEntities)
			if err != nil {
				t.Fatalf("failed to create entities: %v", err)
			}
			if len(createdEntities) != 2 {
				t.Errorf("expected 2 created entities, got %d", len(createdEntities))
			}

			// Verify entities persist
			graph, err = kb.loadGraph()
			if err != nil {
				t.Fatalf("failed to read graph: %v", err)
			}
			if len(graph.Entities) != 2 {
				t.Errorf("expected 2 entities, got %d", len(graph.Entities))
			}

			// Create and verify relations
			testRelations := []Relation{
				{
					From:         "Alice",
					To:           "Bob",
					RelationType: "friend",
				},
			}

			createdRelations, err := kb.createRelations(testRelations)
			if err != nil {
				t.Fatalf("failed to create relations: %v", err)
			}
			if len(createdRelations) != 1 {
				t.Errorf("expected 1 created relation, got %d", len(createdRelations))
			}

			// Add observations to entities
			testObservations := []Observation{
				{
					EntityName: "Alice",
					Contents:   []string{"Works as developer", "Lives in New York"},
				},
			}

			addedObservations, err := kb.addObservations(testObservations)
			if err != nil {
				t.Fatalf("failed to add observations: %v", err)
			}
			if len(addedObservations) != 1 || len(addedObservations[0].Contents) != 2 {
				t.Errorf("expected 1 observation with 2 contents, got %+v", addedObservations)
			}

			// Search nodes by content
			searchResult, err := kb.searchNodes("developer")
			if err != nil {
				t.Fatalf("failed to search nodes: %v", err)
			}
			if len(searchResult.Entities) != 1 || searchResult.Entities[0].Name != "Alice" {
				t.Errorf("expected to find Alice when searching for 'developer', got %+v", searchResult)
			}

			// Retrieve specific nodes
			openResult, err := kb.openNodes([]string{"Bob"})
			if err != nil {
				t.Fatalf("failed to open nodes: %v", err)
			}
			if len(openResult.Entities) != 1 || openResult.Entities[0].Name != "Bob" {
				t.Errorf("expected to find Bob when opening 'Bob', got %+v", openResult)
			}

			// Remove specific observations
			deleteObs := []Observation{
				{
					EntityName:   "Alice",
					Observations: []string{"Works as developer"},
				},
			}
			err = kb.deleteObservations(deleteObs)
			if err != nil {
				t.Fatalf("failed to delete observations: %v", err)
			}

			// Confirm observation removal
			graph, _ = kb.loadGraph()
			aliceIndex := slices.IndexFunc(graph.Entities, func(e Entity) bool {
				return e.Name == "Alice"
			})
			if aliceIndex == -1 {
				t.Errorf("entity 'Alice' not found after deleting observation")
			} else {
				alice := graph.Entities[aliceIndex]
				if slices.Contains(alice.Observations, "Works as developer") {
					t.Errorf("observation 'Works as developer' should have been deleted")
				}
			}

			// Remove relations
			err = kb.deleteRelations(testRelations)
			if err != nil {
				t.Fatalf("failed to delete relations: %v", err)
			}

			// Confirm relation removal
			graph, _ = kb.loadGraph()
			if len(graph.Relations) != 0 {
				t.Errorf("expected 0 relations after deletion, got %d", len(graph.Relations))
			}

			// Remove entities
			err = kb.deleteEntities([]string{"Alice"})
			if err != nil {
				t.Fatalf("failed to delete entities: %v", err)
			}

			// Confirm entity removal
			graph, _ = kb.loadGraph()
			if len(graph.Entities) != 1 || graph.Entities[0].Name != "Bob" {
				t.Errorf("expected only Bob to remain after deleting Alice, got %+v", graph.Entities)
			}
		})
	}
}

// TestSaveAndLoadGraph ensures data persists correctly across save/load cycles.
func TestSaveAndLoadGraph(t *testing.T) {
	for name, newStore := range stores() {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			kb := knowledgeBase{s: s}

			// Setup test data
			testGraph := KnowledgeGraph{
				Entities: []Entity{
					{
						Name:         "Charlie",
						EntityType:   "Person",
						Observations: []string{"Likes hiking"},
					},
				},
				Relations: []Relation{
					{
						From:         "Charlie",
						To:           "Mountains",
						RelationType: "enjoys",
					},
				},
			}

			// Persist to storage
			err := kb.saveGraph(testGraph)
			if err != nil {
				t.Fatalf("failed to save graph: %v", err)
			}

			// Reload from storage
			loadedGraph, err := kb.loadGraph()
			if err != nil {
				t.Fatalf("failed to load graph: %v", err)
			}

			// Verify data integrity
			if !reflect.DeepEqual(testGraph, loadedGraph) {
				t.Errorf("loaded graph does not match saved graph.\nExpected: %+v\nGot: %+v", testGraph, loadedGraph)
			}

			// Test malformed data handling
			if fs, ok := s.(*fileStore); ok {
				err := os.WriteFile(fs.path, []byte("invalid json"), 0o600)
				if err != nil {
					t.Fatalf("failed to write invalid json: %v", err)
				}

				_, err = kb.loadGraph()
				if err == nil {
					t.Errorf("expected error when loading invalid JSON, got nil")
				}
			}
		})
	}
}

// TestDuplicateEntitiesAndRelations verifies duplicate prevention logic.
func TestDuplicateEntitiesAndRelations(t *testing.T) {
	for name, newStore := range stores() {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			kb := knowledgeBase{s: s}

			// Setup initial state
			initialEntities := []Entity{
				{
					Name:         "Dave",
					EntityType:   "Person",
					Observations: []string{"Plays guitar"},
				},
			}

			_, err := kb.createEntities(initialEntities)
			if err != nil {
				t.Fatalf("failed to create initial entities: %v", err)
			}

			// Attempt duplicate creation
			duplicateEntities := []Entity{
				{
					Name:         "Dave",
					EntityType:   "Person",
					Observations: []string{"Sings well"},
				},
				{
					Name:         "Eve",
					EntityType:   "Person",
					Observations: []string{"Plays piano"},
				},
			}

			newEntities, err := kb.createEntities(duplicateEntities)
			if err != nil {
				t.Fatalf("failed when adding duplicate entities: %v", err)
			}

			// Verify only new entities created
			if len(newEntities) != 1 || newEntities[0].Name != "Eve" {
				t.Errorf("expected only 'Eve' to be created, got %+v", newEntities)
			}

			// Setup initial relation
			initialRelation := []Relation{
				{
					From:         "Dave",
					To:           "Eve",
					RelationType: "friend",
				},
			}

			_, err = kb.createRelations(initialRelation)
			if err != nil {
				t.Fatalf("failed to create initial relation: %v", err)
			}

			// Test relation deduplication
			duplicateRelations := []Relation{
				{
					From:         "Dave",
					To:           "Eve",
					RelationType: "friend",
				},
				{
					From:         "Eve",
					To:           "Dave",
					RelationType: "friend",
				},
			}

			newRelations, err := kb.createRelations(duplicateRelations)
			if err != nil {
				t.Fatalf("failed when adding duplicate relations: %v", err)
			}

			// Verify only new relations created
			if len(newRelations) != 1 || newRelations[0].From != "Eve" || newRelations[0].To != "Dave" {
				t.Errorf("expected only 'Eve->Dave' relation to be created, got %+v", newRelations)
			}
		})
	}
}

// TestErrorHandling verifies proper error responses for invalid operations.
func TestErrorHandling(t *testing.T) {
	t.Run("FileStoreWriteError", func(t *testing.T) {
		// Test file write to invalid path
		kb := knowledgeBase{
			s: &fileStore{path: filepath.Join("nonexistent", "directory", "file.json")},
		}

		testEntities := []Entity{
			{Name: "TestEntity"},
		}

		_, err := kb.createEntities(testEntities)
		if err == nil {
			t.Errorf("expected error when writing to non-existent directory, got nil")
		}
	})

	for name, newStore := range stores() {
		t.Run(fmt.Sprintf("AddObservationToNonExistentEntity_%s", name), func(t *testing.T) {
			s := newStore(t)
			kb := knowledgeBase{s: s}

			// Setup valid entity for comparison
			_, err := kb.createEntities([]Entity{{Name: "RealEntity"}})
			if err != nil {
				t.Fatalf("failed to create test entity: %v", err)
			}

			// Test invalid entity reference
			nonExistentObs := []Observation{
				{
					EntityName: "NonExistentEntity",
					Contents:   []string{"This shouldn't work"},
				},
			}

			_, err = kb.addObservations(nonExistentObs)
			if err == nil {
				t.Errorf("expected error when adding observations to non-existent entity, got nil")
			}
		})
	}
}

// TestFileFormatting verifies the JSON storage format structure.
func TestFileFormatting(t *testing.T) {
	for name, newStore := range stores() {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			kb := knowledgeBase{s: s}

			// Setup test entity
			testEntities := []Entity{
				{
					Name:         "FileTest",
					EntityType:   "TestEntity",
					Observations: []string{"Test observation"},
				},
			}

			_, err := kb.createEntities(testEntities)
			if err != nil {
				t.Fatalf("failed to create test entity: %v", err)
			}

			// Extract raw storage data
			data, err := s.Read()
			if err != nil {
				t.Fatalf("failed to read from store: %v", err)
			}

			// Validate JSON format
			var items []kbItem
			err = json.Unmarshal(data, &items)
			if err != nil {
				t.Fatalf("failed to parse store data JSON: %v", err)
			}

			// Check data structure
			if len(items) != 1 {
				t.Fatalf("expected 1 item in memory file, got %d", len(items))
			}

			item := items[0]
			if item.Type != "entity" ||
				item.Name != "FileTest" ||
				item.EntityType != "TestEntity" ||
				len(item.Observations) != 1 ||
				item.Observations[0] != "Test observation" {
				t.Errorf("store item format incorrect: %+v", item)
			}
		})
	}
}

// TestMCPServerIntegration tests the knowledge base through MCP server layer.
func TestMCPServerIntegration(t *testing.T) {
	for name, newStore := range stores() {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			kb := knowledgeBase{s: s}

			// Create mock server session
			ctx := context.Background()

			createResult, out, err := kb.CreateEntities(ctx, nil, CreateEntitiesArgs{
				Entities: []Entity{
					{
						Name:         "TestPerson",
						EntityType:   "Person",
						Observations: []string{"Likes testing"},
					},
				},
			})
			if err != nil {
				t.Fatalf("MCP CreateEntities failed: %v", err)
			}
			if createResult.IsError {
				t.Fatalf("MCP CreateEntities returned error: %v", createResult.Content)
			}
			if len(out.Entities) != 1 {
				t.Errorf("expected 1 entity created, got %d", len(out.Entities))
			}

			// Test ReadGraph through MCP
			readResult, outg, err := kb.ReadGraph(ctx, nil, nil)
			if err != nil {
				t.Fatalf("MCP ReadGraph failed: %v", err)
			}
			if readResult.IsError {
				t.Fatalf("MCP ReadGraph returned error: %v", readResult.Content)
			}
			if len(outg.Entities) != 1 {
				t.Errorf("expected 1 entity in graph, got %d", len(outg.Entities))
			}

			relationsResult, outr, err := kb.CreateRelations(ctx, nil, CreateRelationsArgs{
				Relations: []Relation{
					{
						From:         "TestPerson",
						To:           "Testing",
						RelationType: "likes",
					},
				},
			})
			if err != nil {
				t.Fatalf("MCP CreateRelations failed: %v", err)
			}
			if relationsResult.IsError {
				t.Fatalf("MCP CreateRelations returned error: %v", relationsResult.Content)
			}
			if len(outr.Relations) != 1 {
				t.Errorf("expected 1 relation created, got %d", len(outr.Relations))
			}

			obsResult, outo, err := kb.AddObservations(ctx, nil, AddObservationsArgs{
				Observations: []Observation{
					{
						EntityName: "TestPerson",
						Contents:   []string{"Works remotely", "Drinks coffee"},
					},
				},
			})
			if err != nil {
				t.Fatalf("MCP AddObservations failed: %v", err)
			}
			if obsResult.IsError {
				t.Fatalf("MCP AddObservations returned error: %v", obsResult.Content)
			}
			if len(outo.Observations) != 1 {
				t.Errorf("expected 1 observation result, got %d", len(outo.Observations))
			}

			searchResult, outg, err := kb.SearchNodes(ctx, nil, SearchNodesArgs{
				Query: "coffee",
			})
			if err != nil {
				t.Fatalf("MCP SearchNodes failed: %v", err)
			}
			if searchResult.IsError {
				t.Fatalf("MCP SearchNodes returned error: %v", searchResult.Content)
			}
			if len(outg.Entities) != 1 {
				t.Errorf("expected 1 entity from search, got %d", len(outg.Entities))
			}

			openResult, outg, err := kb.OpenNodes(ctx, nil, OpenNodesArgs{
				Names: []string{"TestPerson"},
			})
			if err != nil {
				t.Fatalf("MCP OpenNodes failed: %v", err)
			}
			if openResult.IsError {
				t.Fatalf("MCP OpenNodes returned error: %v", openResult.Content)
			}
			if len(outg.Entities) != 1 {
				t.Errorf("expected 1 entity from open, got %d", len(outg.Entities))
			}

			deleteObsResult, _, err := kb.DeleteObservations(ctx, nil, DeleteObservationsArgs{
				Deletions: []Observation{
					{
						EntityName:   "TestPerson",
						Observations: []string{"Works remotely"},
					},
				},
			})
			if err != nil {
				t.Fatalf("MCP DeleteObservations failed: %v", err)
			}
			if deleteObsResult.IsError {
				t.Fatalf("MCP DeleteObservations returned error: %v", deleteObsResult.Content)
			}

			deleteRelResult, _, err := kb.DeleteRelations(ctx, nil, DeleteRelationsArgs{
				Relations: []Relation{
					{
						From:         "TestPerson",
						To:           "Testing",
						RelationType: "likes",
					},
				},
			})
			if err != nil {
				t.Fatalf("MCP DeleteRelations failed: %v", err)
			}
			if deleteRelResult.IsError {
				t.Fatalf("MCP DeleteRelations returned error: %v", deleteRelResult.Content)
			}

			deleteEntResult, _, err := kb.DeleteEntities(ctx, nil, DeleteEntitiesArgs{
				EntityNames: []string{"TestPerson"},
			})
			if err != nil {
				t.Fatalf("MCP DeleteEntities failed: %v", err)
			}
			if deleteEntResult.IsError {
				t.Fatalf("MCP DeleteEntities returned error: %v", deleteEntResult.Content)
			}

			// Verify final state
			_, outg, err = kb.ReadGraph(ctx, nil, nil)
			if err != nil {
				t.Fatalf("Final MCP ReadGraph failed: %v", err)
			}
			if len(outg.Entities) != 0 {
				t.Errorf("expected empty graph after deletion, got %d entities", len(outg.Entities))
			}
		})
	}
}

// TestMCPErrorHandling tests error scenarios through MCP layer.
func TestMCPErrorHandling(t *testing.T) {
	for name, newStore := range stores() {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			kb := knowledgeBase{s: s}

			ctx := context.Background()

			_, _, err := kb.AddObservations(ctx, nil, AddObservationsArgs{
				Observations: []Observation{
					{
						EntityName: "NonExistentEntity",
						Contents:   []string{"This should fail"},
					},
				},
			})
			if err == nil {
				t.Errorf("expected MCP AddObservations to return error for non-existent entity")
			} else {
				// Verify the error message contains expected text
				want := "entity with name NonExistentEntity not found"
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected error message to contain '%s', got: %v", want, err)
				}
			}
		})
	}
}

// TestMCPResponseFormat verifies MCP response format consistency.
func TestMCPResponseFormat(t *testing.T) {
	s := &memoryStore{}
	kb := knowledgeBase{s: s}

	ctx := context.Background()

	result, out, err := kb.CreateEntities(ctx, nil, CreateEntitiesArgs{
		Entities: []Entity{
			{Name: "FormatTest", EntityType: "Test"},
		},
	})
	if err != nil {
		t.Fatalf("CreateEntities failed: %v", err)
	}

	// Verify response has both Content and StructuredContent
	if len(result.Content) == 0 {
		t.Errorf("expected Content field to be populated")
	}
	if len(out.Entities) == 0 {
		t.Errorf("expected StructuredContent.Entities to be populated")
	}

	// Verify Content contains simple success message
	if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
		expectedMessage := "Entities created successfully"
		if textContent.Text != expectedMessage {
			t.Errorf("expected Content field to contain '%s', got '%s'", expectedMessage, textContent.Text)
		}
	} else {
		t.Errorf("expected Content[0] to be TextContent")
	}
}
