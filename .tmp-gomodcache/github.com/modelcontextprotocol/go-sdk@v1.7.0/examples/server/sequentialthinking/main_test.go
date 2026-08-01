// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStartThinking(t *testing.T) {
	// Reset store for clean test
	store = NewSessionStore()

	ctx := context.Background()

	args := StartThinkingArgs{
		Problem:        "How to implement a binary search algorithm",
		SessionID:      "test_session",
		EstimatedSteps: 5,
	}

	result, _, err := StartThinking(ctx, nil, args)
	if err != nil {
		t.Fatalf("StartThinking() error = %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("No content in result")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("Expected TextContent")
	}

	if !strings.Contains(textContent.Text, "test_session") {
		t.Error("Result should contain session ID")
	}

	if !strings.Contains(textContent.Text, "How to implement a binary search algorithm") {
		t.Error("Result should contain the problem statement")
	}

	// Verify session was stored
	session, exists := store.Session("test_session")
	if !exists {
		t.Fatal("Session was not stored")
	}

	if session.Problem != args.Problem {
		t.Errorf("Expected problem %s, got %s", args.Problem, session.Problem)
	}

	if session.EstimatedTotal != 5 {
		t.Errorf("Expected estimated total 5, got %d", session.EstimatedTotal)
	}

	if session.Status != "active" {
		t.Errorf("Expected status 'active', got %s", session.Status)
	}
}

func TestContinueThinking(t *testing.T) {
	// Reset store and create initial session
	store = NewSessionStore()

	// First start a thinking session
	ctx := context.Background()
	startArgs := StartThinkingArgs{
		Problem:        "Test problem",
		SessionID:      "test_continue",
		EstimatedSteps: 3,
	}

	_, _, err := StartThinking(ctx, nil, startArgs)
	if err != nil {
		t.Fatalf("StartThinking() error = %v", err)
	}

	// Now continue thinking
	continueArgs := ContinueThinkingArgs{
		SessionID: "test_continue",
		Thought:   "First thought: I need to understand the problem",
	}

	result, _, err := ContinueThinking(ctx, nil, continueArgs)
	if err != nil {
		t.Fatalf("ContinueThinking() error = %v", err)
	}

	// Verify result
	if len(result.Content) == 0 {
		t.Fatal("No content in result")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("Expected TextContent")
	}

	if !strings.Contains(textContent.Text, "Step 1") {
		t.Error("Result should contain step number")
	}

	// Verify session was updated
	session, exists := store.Session("test_continue")
	if !exists {
		t.Fatal("Session not found")
	}

	if len(session.Thoughts) != 1 {
		t.Errorf("Expected 1 thought, got %d", len(session.Thoughts))
	}

	if session.Thoughts[0].Content != continueArgs.Thought {
		t.Errorf("Expected thought content %s, got %s", continueArgs.Thought, session.Thoughts[0].Content)
	}

	if session.CurrentThought != 1 {
		t.Errorf("Expected current thought 1, got %d", session.CurrentThought)
	}
}

func TestContinueThinkingWithCompletion(t *testing.T) {
	// Reset store and create initial session
	store = NewSessionStore()

	ctx := context.Background()
	startArgs := StartThinkingArgs{
		Problem:   "Simple test",
		SessionID: "test_completion",
	}

	_, _, err := StartThinking(ctx, nil, startArgs)
	if err != nil {
		t.Fatalf("StartThinking() error = %v", err)
	}

	// Continue with completion flag
	nextNeeded := false
	continueArgs := ContinueThinkingArgs{
		SessionID:  "test_completion",
		Thought:    "Final thought",
		NextNeeded: &nextNeeded,
	}

	result, _, err := ContinueThinking(ctx, nil, continueArgs)
	if err != nil {
		t.Fatalf("ContinueThinking() error = %v", err)
	}

	// Check completion message
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("Expected TextContent")
	}

	if !strings.Contains(textContent.Text, "completed") {
		t.Error("Result should indicate completion")
	}

	// Verify session status
	session, exists := store.Session("test_completion")
	if !exists {
		t.Fatal("Session not found")
	}

	if session.Status != "completed" {
		t.Errorf("Expected status 'completed', got %s", session.Status)
	}
}

func TestContinueThinkingRevision(t *testing.T) {
	// Setup session with existing thoughts
	store = NewSessionStore()
	session := &ThinkingSession{
		ID:      "test_revision",
		Problem: "Test problem",
		Thoughts: []*Thought{
			{Index: 1, Content: "Original thought", Created: time.Now()},
			{Index: 2, Content: "Second thought", Created: time.Now()},
		},
		CurrentThought: 2,
		EstimatedTotal: 3,
		Status:         "active",
		Created:        time.Now(),
		LastActivity:   time.Now(),
	}
	store.SetSession(session)

	ctx := context.Background()
	reviseStep := 1
	continueArgs := ContinueThinkingArgs{
		SessionID:  "test_revision",
		Thought:    "Revised first thought",
		ReviseStep: &reviseStep,
	}

	result, _, err := ContinueThinking(ctx, nil, continueArgs)
	if err != nil {
		t.Fatalf("ContinueThinking() error = %v", err)
	}

	// Verify revision message
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("Expected TextContent")
	}

	if !strings.Contains(textContent.Text, "Revised step 1") {
		t.Error("Result should indicate revision")
	}

	// Verify thought was revised
	updatedSession, _ := store.Session("test_revision")
	if updatedSession.Thoughts[0].Content != "Revised first thought" {
		t.Error("First thought should be revised")
	}

	if !updatedSession.Thoughts[0].Revised {
		t.Error("First thought should be marked as revised")
	}
}

// func TestContinueThinkingBranching(t *testing.T) {
// 	// Setup session with existing thoughts
// 	store = NewSessionStore()
// 	session := &ThinkingSession{
// 		ID:      "test_branch",
// 		Problem: "Test problem",
// 		Thoughts: []*Thought{
// 			{Index: 1, Content: "First thought", Created: time.Now()},
// 		},
// 		CurrentThought: 1,
// 		EstimatedTotal: 3,
// 		Status:         "active",
// 		Created:        time.Now(),
// 		LastActivity:   time.Now(),
// 		Branches:       []string{},
// 	}
// 	store.SetSession(session)

// 	ctx := context.Background()
// 	continueArgs := ContinueThinkingArgs{
// 		SessionID:    "test_branch",
// 		Thought:      "Alternative approach",
// 		CreateBranch: true,
// 	}

// 	continueParams := &mcp.CallToolParamsFor[ContinueThinkingArgs]{
// 		Name:      "continue_thinking",
// 		Arguments: continueArgs,
// 	}

// 	// Verify branch creation message
// 	textContent, ok := result.Content[0].(*mcp.TextContent)
// 	if !ok {
// 		t.Fatal("Expected TextContent")
// 	}

// 	if !strings.Contains(textContent.Text, "Created branch") {
// 		t.Error("Result should indicate branch creation")
// 	}

// 	// Verify branch was created
// 	updatedSession, _ := store.Session("test_branch")
// 	if len(updatedSession.Branches) != 1 {
// 		t.Errorf("Expected 1 branch, got %d", len(updatedSession.Branches))
// 	}

// 	branchID := updatedSession.Branches[0]
// 	if !strings.Contains(branchID, "test_branch_branch_") {
// 		t.Error("Branch ID should contain parent session ID")
// 	}

// 	// Verify branch session exists
// 	branchSession, exists := store.Session(branchID)
// 	if !exists {
// 		t.Fatal("Branch session should exist")
// 	}

// 	if len(branchSession.Thoughts) != 1 {
// 		t.Error("Branch should inherit parent thoughts")
// 	}
// }

func TestReviewThinking(t *testing.T) {
	// Setup session with thoughts
	store = NewSessionStore()
	session := &ThinkingSession{
		ID:      "test_review",
		Problem: "Complex problem",
		Thoughts: []*Thought{
			{Index: 1, Content: "First thought", Created: time.Now(), Revised: false},
			{Index: 2, Content: "Second thought", Created: time.Now(), Revised: true},
			{Index: 3, Content: "Final thought", Created: time.Now(), Revised: false},
		},
		CurrentThought: 3,
		EstimatedTotal: 3,
		Status:         "completed",
		Created:        time.Now(),
		LastActivity:   time.Now(),
		Branches:       []string{"test_review_branch_1"},
	}
	store.SetSession(session)

	ctx := context.Background()
	reviewArgs := ReviewThinkingArgs{
		SessionID: "test_review",
	}

	result, _, err := ReviewThinking(ctx, nil, reviewArgs)
	if err != nil {
		t.Fatalf("ReviewThinking() error = %v", err)
	}

	// Verify review content
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("Expected TextContent")
	}

	reviewText := textContent.Text

	if !strings.Contains(reviewText, "test_review") {
		t.Error("Review should contain session ID")
	}

	if !strings.Contains(reviewText, "Complex problem") {
		t.Error("Review should contain problem")
	}

	if !strings.Contains(reviewText, "completed") {
		t.Error("Review should contain status")
	}

	if !strings.Contains(reviewText, "Steps: 3 of ~3") {
		t.Error("Review should contain step count")
	}

	if !strings.Contains(reviewText, "First thought") {
		t.Error("Review should contain first thought")
	}

	if !strings.Contains(reviewText, "(revised)") {
		t.Error("Review should indicate revised thoughts")
	}

	if !strings.Contains(reviewText, "test_review_branch_1") {
		t.Error("Review should list branches")
	}
}

func TestThinkingHistory(t *testing.T) {
	// Setup test sessions
	store = NewSessionStore()
	session1 := &ThinkingSession{
		ID:             "session1",
		Problem:        "Problem 1",
		Thoughts:       []*Thought{{Index: 1, Content: "Thought 1", Created: time.Now()}},
		CurrentThought: 1,
		EstimatedTotal: 2,
		Status:         "active",
		Created:        time.Now(),
		LastActivity:   time.Now(),
	}
	session2 := &ThinkingSession{
		ID:             "session2",
		Problem:        "Problem 2",
		Thoughts:       []*Thought{{Index: 1, Content: "Thought 1", Created: time.Now()}},
		CurrentThought: 1,
		EstimatedTotal: 3,
		Status:         "completed",
		Created:        time.Now(),
		LastActivity:   time.Now(),
	}
	store.SetSession(session1)
	store.SetSession(session2)

	ctx := context.Background()

	// Test listing all sessions
	result, err := ThinkingHistory(ctx, &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "thinking://sessions",
		},
	})
	if err != nil {
		t.Fatalf("ThinkingHistory() error = %v", err)
	}

	if len(result.Contents) != 1 {
		t.Fatal("Expected 1 content item")
	}

	content := result.Contents[0]
	if content.MIMEType != "application/json" {
		t.Error("Expected JSON MIME type")
	}

	// Parse and verify sessions list
	var sessions []*ThinkingSession
	err = json.Unmarshal([]byte(content.Text), &sessions)
	if err != nil {
		t.Fatalf("Failed to parse sessions JSON: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(sessions))
	}

	// Test getting specific session
	result, err = ThinkingHistory(ctx, &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "thinking://session1"},
	})
	if err != nil {
		t.Fatalf("ThinkingHistory() error = %v", err)
	}

	var retrievedSession ThinkingSession
	err = json.Unmarshal([]byte(result.Contents[0].Text), &retrievedSession)
	if err != nil {
		t.Fatalf("Failed to parse session JSON: %v", err)
	}

	if retrievedSession.ID != "session1" {
		t.Errorf("Expected session ID 'session1', got %s", retrievedSession.ID)
	}

	if retrievedSession.Problem != "Problem 1" {
		t.Errorf("Expected problem 'Problem 1', got %s", retrievedSession.Problem)
	}
}

func TestInvalidOperations(t *testing.T) {
	store = NewSessionStore()
	ctx := context.Background()

	// Test continue thinking with non-existent session
	continueArgs := ContinueThinkingArgs{
		SessionID: "nonexistent",
		Thought:   "Some thought",
	}

	_, _, err := ContinueThinking(ctx, nil, continueArgs)
	if err == nil {
		t.Error("Expected error for non-existent session")
	}

	// Test review with non-existent session
	reviewArgs := ReviewThinkingArgs{
		SessionID: "nonexistent",
	}

	_, _, err = ReviewThinking(ctx, nil, reviewArgs)
	if err == nil {
		t.Error("Expected error for non-existent session in review")
	}

	// Test invalid revision step
	session := &ThinkingSession{
		ID:             "test_invalid",
		Problem:        "Test",
		Thoughts:       []*Thought{{Index: 1, Content: "Thought", Created: time.Now()}},
		CurrentThought: 1,
		EstimatedTotal: 2,
		Status:         "active",
		Created:        time.Now(),
		LastActivity:   time.Now(),
	}
	store.SetSession(session)

	reviseStep := 5 // Invalid step number
	invalidReviseArgs := ContinueThinkingArgs{
		SessionID:  "test_invalid",
		Thought:    "Revised",
		ReviseStep: &reviseStep,
	}

	_, _, err = ContinueThinking(ctx, nil, invalidReviseArgs)
	if err == nil {
		t.Error("Expected error for invalid revision step")
	}
}
