// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Entity represents a knowledge graph node with observations.
type Entity struct {
	Name         string   `json:"name"`
	EntityType   string   `json:"entityType"`
	Observations []string `json:"observations"`
}

// Relation represents a directed edge between two entities.
type Relation struct {
	From         string `json:"from"`
	To           string `json:"to"`
	RelationType string `json:"relationType"`
}

// Observation contains facts about an entity.
type Observation struct {
	EntityName string   `json:"entityName"`
	Contents   []string `json:"contents"`

	Observations []string `json:"observations,omitempty"` // Used for deletion operations
}

// KnowledgeGraph represents the complete graph structure.
type KnowledgeGraph struct {
	Entities  []Entity   `json:"entities"`
	Relations []Relation `json:"relations"`
}

// store provides persistence interface for knowledge base data.
type store interface {
	Read() ([]byte, error)
	Write(data []byte) error
}

// memoryStore implements in-memory storage that doesn't persist across restarts.
type memoryStore struct {
	data []byte
}

// Read returns the in-memory data.
func (ms *memoryStore) Read() ([]byte, error) {
	return ms.data, nil
}

// Write stores data in memory.
func (ms *memoryStore) Write(data []byte) error {
	ms.data = data
	return nil
}

// fileStore implements file-based storage for persistent knowledge base.
type fileStore struct {
	path string
}

// Read loads data from file, returning empty slice if file doesn't exist.
func (fs *fileStore) Read() ([]byte, error) {
	data, err := os.ReadFile(fs.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read file %s: %w", fs.path, err)
	}
	return data, nil
}

// Write saves data to file with 0600 permissions.
func (fs *fileStore) Write(data []byte) error {
	if err := os.WriteFile(fs.path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write file %s: %w", fs.path, err)
	}
	return nil
}

// knowledgeBase manages entities and relations with persistent storage.
type knowledgeBase struct {
	s store
}

// kbItem represents a single item in persistent storage (entity or relation).
type kbItem struct {
	Type string `json:"type"`

	// Entity fields (when Type == "entity")
	Name         string   `json:"name,omitempty"`
	EntityType   string   `json:"entityType,omitempty"`
	Observations []string `json:"observations,omitempty"`

	// Relation fields (when Type == "relation")
	From         string `json:"from,omitempty"`
	To           string `json:"to,omitempty"`
	RelationType string `json:"relationType,omitempty"`
}

// loadGraph deserializes the knowledge graph from storage.
func (k knowledgeBase) loadGraph() (KnowledgeGraph, error) {
	data, err := k.s.Read()
	if err != nil {
		return KnowledgeGraph{}, fmt.Errorf("failed to read from store: %w", err)
	}

	if len(data) == 0 {
		return KnowledgeGraph{}, nil
	}

	var items []kbItem
	if err := json.Unmarshal(data, &items); err != nil {
		return KnowledgeGraph{}, fmt.Errorf("failed to unmarshal from store: %w", err)
	}

	graph := KnowledgeGraph{}

	for _, item := range items {
		switch item.Type {
		case "entity":
			graph.Entities = append(graph.Entities, Entity{
				Name:         item.Name,
				EntityType:   item.EntityType,
				Observations: item.Observations,
			})
		case "relation":
			graph.Relations = append(graph.Relations, Relation{
				From:         item.From,
				To:           item.To,
				RelationType: item.RelationType,
			})
		}
	}

	return graph, nil
}

// saveGraph serializes and persists the knowledge graph to storage.
func (k knowledgeBase) saveGraph(graph KnowledgeGraph) error {
	items := make([]kbItem, 0, len(graph.Entities)+len(graph.Relations))

	for _, entity := range graph.Entities {
		items = append(items, kbItem{
			Type:         "entity",
			Name:         entity.Name,
			EntityType:   entity.EntityType,
			Observations: entity.Observations,
		})
	}

	for _, relation := range graph.Relations {
		items = append(items, kbItem{
			Type:         "relation",
			From:         relation.From,
			To:           relation.To,
			RelationType: relation.RelationType,
		})
	}

	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("failed to marshal items: %w", err)
	}

	if err := k.s.Write(itemsJSON); err != nil {
		return fmt.Errorf("failed to write to store: %w", err)
	}
	return nil
}

// createEntities adds new entities to the graph, skipping duplicates by name.
// It returns the new entities that were actually added.
func (k knowledgeBase) createEntities(entities []Entity) ([]Entity, error) {
	graph, err := k.loadGraph()
	if err != nil {
		return nil, err
	}

	var newEntities []Entity
	for _, entity := range entities {
		if !slices.ContainsFunc(graph.Entities, func(e Entity) bool { return e.Name == entity.Name }) {
			newEntities = append(newEntities, entity)
			graph.Entities = append(graph.Entities, entity)
		}
	}

	if err := k.saveGraph(graph); err != nil {
		return nil, err
	}

	return newEntities, nil
}

// createRelations adds new relations to the graph, skipping exact duplicates.
// It returns the new relations that were actually added.
func (k knowledgeBase) createRelations(relations []Relation) ([]Relation, error) {
	graph, err := k.loadGraph()
	if err != nil {
		return nil, err
	}

	var newRelations []Relation
	for _, relation := range relations {
		exists := slices.ContainsFunc(graph.Relations, func(r Relation) bool {
			return r.From == relation.From &&
				r.To == relation.To &&
				r.RelationType == relation.RelationType
		})
		if !exists {
			newRelations = append(newRelations, relation)
			graph.Relations = append(graph.Relations, relation)
		}
	}

	if err := k.saveGraph(graph); err != nil {
		return nil, err
	}

	return newRelations, nil
}

// addObservations appends new observations to existing entities.
// It returns the new observations that were actually added.
func (k knowledgeBase) addObservations(observations []Observation) ([]Observation, error) {
	graph, err := k.loadGraph()
	if err != nil {
		return nil, err
	}

	var results []Observation

	for _, obs := range observations {
		entityIndex := slices.IndexFunc(graph.Entities, func(e Entity) bool { return e.Name == obs.EntityName })
		if entityIndex == -1 {
			return nil, fmt.Errorf("entity with name %s not found", obs.EntityName)
		}

		var newObservations []string
		for _, content := range obs.Contents {
			if !slices.Contains(graph.Entities[entityIndex].Observations, content) {
				newObservations = append(newObservations, content)
				graph.Entities[entityIndex].Observations = append(graph.Entities[entityIndex].Observations, content)
			}
		}

		results = append(results, Observation{
			EntityName: obs.EntityName,
			Contents:   newObservations,
		})
	}

	if err := k.saveGraph(graph); err != nil {
		return nil, err
	}

	return results, nil
}

// deleteEntities removes entities and their associated relations.
func (k knowledgeBase) deleteEntities(entityNames []string) error {
	graph, err := k.loadGraph()
	if err != nil {
		return err
	}

	// Create map for quick lookup
	entitiesToDelete := make(map[string]bool)
	for _, name := range entityNames {
		entitiesToDelete[name] = true
	}

	// Filter entities using slices.DeleteFunc
	graph.Entities = slices.DeleteFunc(graph.Entities, func(entity Entity) bool {
		return entitiesToDelete[entity.Name]
	})

	// Filter relations using slices.DeleteFunc
	graph.Relations = slices.DeleteFunc(graph.Relations, func(relation Relation) bool {
		return entitiesToDelete[relation.From] || entitiesToDelete[relation.To]
	})

	return k.saveGraph(graph)
}

// deleteObservations removes specific observations from entities.
func (k knowledgeBase) deleteObservations(deletions []Observation) error {
	graph, err := k.loadGraph()
	if err != nil {
		return err
	}

	for _, deletion := range deletions {
		entityIndex := slices.IndexFunc(graph.Entities, func(e Entity) bool {
			return e.Name == deletion.EntityName
		})
		if entityIndex == -1 {
			continue
		}

		// Create a map for quick lookup
		observationsToDelete := make(map[string]bool)
		for _, observation := range deletion.Observations {
			observationsToDelete[observation] = true
		}

		// Filter observations using slices.DeleteFunc
		graph.Entities[entityIndex].Observations = slices.DeleteFunc(graph.Entities[entityIndex].Observations, func(observation string) bool {
			return observationsToDelete[observation]
		})
	}

	return k.saveGraph(graph)
}

// deleteRelations removes specific relations from the graph.
func (k knowledgeBase) deleteRelations(relations []Relation) error {
	graph, err := k.loadGraph()
	if err != nil {
		return err
	}

	// Filter relations using slices.DeleteFunc and slices.ContainsFunc
	graph.Relations = slices.DeleteFunc(graph.Relations, func(existingRelation Relation) bool {
		return slices.ContainsFunc(relations, func(relationToDelete Relation) bool {
			return existingRelation.From == relationToDelete.From &&
				existingRelation.To == relationToDelete.To &&
				existingRelation.RelationType == relationToDelete.RelationType
		})
	})
	return k.saveGraph(graph)
}

// searchNodes filters entities and relations matching the query string.
func (k knowledgeBase) searchNodes(query string) (KnowledgeGraph, error) {
	graph, err := k.loadGraph()
	if err != nil {
		return KnowledgeGraph{}, err
	}

	queryLower := strings.ToLower(query)
	var filteredEntities []Entity

	// Filter entities
	for _, entity := range graph.Entities {
		if strings.Contains(strings.ToLower(entity.Name), queryLower) ||
			strings.Contains(strings.ToLower(entity.EntityType), queryLower) {
			filteredEntities = append(filteredEntities, entity)
			continue
		}

		// Check observations
		for _, observation := range entity.Observations {
			if strings.Contains(strings.ToLower(observation), queryLower) {
				filteredEntities = append(filteredEntities, entity)
				break
			}
		}
	}

	// Create map for quick entity lookup
	filteredEntityNames := make(map[string]bool)
	for _, entity := range filteredEntities {
		filteredEntityNames[entity.Name] = true
	}

	// Filter relations
	var filteredRelations []Relation
	for _, relation := range graph.Relations {
		if filteredEntityNames[relation.From] && filteredEntityNames[relation.To] {
			filteredRelations = append(filteredRelations, relation)
		}
	}

	return KnowledgeGraph{
		Entities:  filteredEntities,
		Relations: filteredRelations,
	}, nil
}

// openNodes returns entities with specified names and their interconnecting relations.
func (k knowledgeBase) openNodes(names []string) (KnowledgeGraph, error) {
	graph, err := k.loadGraph()
	if err != nil {
		return KnowledgeGraph{}, err
	}

	// Create map for quick name lookup
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	// Filter entities
	var filteredEntities []Entity
	for _, entity := range graph.Entities {
		if nameSet[entity.Name] {
			filteredEntities = append(filteredEntities, entity)
		}
	}

	// Create map for quick entity lookup
	filteredEntityNames := make(map[string]bool)
	for _, entity := range filteredEntities {
		filteredEntityNames[entity.Name] = true
	}

	// Filter relations
	var filteredRelations []Relation
	for _, relation := range graph.Relations {
		if filteredEntityNames[relation.From] && filteredEntityNames[relation.To] {
			filteredRelations = append(filteredRelations, relation)
		}
	}

	return KnowledgeGraph{
		Entities:  filteredEntities,
		Relations: filteredRelations,
	}, nil
}

func (k knowledgeBase) CreateEntities(ctx context.Context, req *mcp.CallToolRequest, args CreateEntitiesArgs) (*mcp.CallToolResult, CreateEntitiesResult, error) {
	var res mcp.CallToolResult

	entities, err := k.createEntities(args.Entities)
	if err != nil {
		return nil, CreateEntitiesResult{}, err
	}

	res.Content = []mcp.Content{
		&mcp.TextContent{Text: "Entities created successfully"},
	}
	return &res, CreateEntitiesResult{Entities: entities}, nil
}

func (k knowledgeBase) CreateRelations(ctx context.Context, req *mcp.CallToolRequest, args CreateRelationsArgs) (*mcp.CallToolResult, CreateRelationsResult, error) {
	var res mcp.CallToolResult

	relations, err := k.createRelations(args.Relations)
	if err != nil {
		return nil, CreateRelationsResult{}, err
	}

	res.Content = []mcp.Content{
		&mcp.TextContent{Text: "Relations created successfully"},
	}

	return &res, CreateRelationsResult{Relations: relations}, nil
}

func (k knowledgeBase) AddObservations(ctx context.Context, req *mcp.CallToolRequest, args AddObservationsArgs) (*mcp.CallToolResult, AddObservationsResult, error) {
	var res mcp.CallToolResult

	observations, err := k.addObservations(args.Observations)
	if err != nil {
		return nil, AddObservationsResult{}, err
	}

	res.Content = []mcp.Content{
		&mcp.TextContent{Text: "Observations added successfully"},
	}

	return &res, AddObservationsResult{
		Observations: observations,
	}, nil
}

func (k knowledgeBase) DeleteEntities(ctx context.Context, req *mcp.CallToolRequest, args DeleteEntitiesArgs) (*mcp.CallToolResult, any, error) {
	var res mcp.CallToolResult

	err := k.deleteEntities(args.EntityNames)
	if err != nil {
		return nil, nil, err
	}

	res.Content = []mcp.Content{
		&mcp.TextContent{Text: "Entities deleted successfully"},
	}

	return &res, nil, nil
}

func (k knowledgeBase) DeleteObservations(ctx context.Context, req *mcp.CallToolRequest, args DeleteObservationsArgs) (*mcp.CallToolResult, any, error) {
	var res mcp.CallToolResult

	err := k.deleteObservations(args.Deletions)
	if err != nil {
		return nil, nil, err
	}

	res.Content = []mcp.Content{
		&mcp.TextContent{Text: "Observations deleted successfully"},
	}

	return &res, nil, nil
}

func (k knowledgeBase) DeleteRelations(ctx context.Context, req *mcp.CallToolRequest, args DeleteRelationsArgs) (*mcp.CallToolResult, struct{}, error) {
	var res mcp.CallToolResult

	err := k.deleteRelations(args.Relations)
	if err != nil {
		return nil, struct{}{}, err
	}

	res.Content = []mcp.Content{
		&mcp.TextContent{Text: "Relations deleted successfully"},
	}

	return &res, struct{}{}, nil
}

func (k knowledgeBase) ReadGraph(ctx context.Context, req *mcp.CallToolRequest, args any) (*mcp.CallToolResult, KnowledgeGraph, error) {
	var res mcp.CallToolResult

	graph, err := k.loadGraph()
	if err != nil {
		return nil, KnowledgeGraph{}, err
	}

	res.Content = []mcp.Content{
		&mcp.TextContent{Text: "Graph read successfully"},
	}

	return &res, graph, nil
}

func (k knowledgeBase) SearchNodes(ctx context.Context, req *mcp.CallToolRequest, args SearchNodesArgs) (*mcp.CallToolResult, KnowledgeGraph, error) {
	var res mcp.CallToolResult

	graph, err := k.searchNodes(args.Query)
	if err != nil {
		return nil, KnowledgeGraph{}, err
	}

	res.Content = []mcp.Content{
		&mcp.TextContent{Text: "Nodes searched successfully"},
	}

	return &res, graph, nil
}

func (k knowledgeBase) OpenNodes(ctx context.Context, req *mcp.CallToolRequest, args OpenNodesArgs) (*mcp.CallToolResult, KnowledgeGraph, error) {
	var res mcp.CallToolResult

	graph, err := k.openNodes(args.Names)
	if err != nil {
		return nil, KnowledgeGraph{}, err
	}

	res.Content = []mcp.Content{
		&mcp.TextContent{Text: "Nodes opened successfully"},
	}
	return &res, graph, nil
}
