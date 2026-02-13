package main

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
)

func TestReadJSONRPCResponse_DirectResponse(t *testing.T) {
	t.Parallel()
	input := `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}` + "\n"
	scanner := bufio.NewScanner(strings.NewReader(input))

	got, err := readJSONRPCResponse(scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}` {
		t.Fatalf("unexpected response: %s", got)
	}
}

func TestReadJSONRPCResponse_SkipsNotifications(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","method":"notifications/resources/list_changed","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}`,
		`{"jsonrpc":"2.0","id":42,"result":{"content":[{"type":"text","text":"hello"}]}}`,
	}, "\n") + "\n"
	scanner := bufio.NewScanner(strings.NewReader(input))

	got, err := readJSONRPCResponse(scanner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var msg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(got), &msg); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	// Verify we got the response with id:42, not a notification
	var id int
	if err := json.Unmarshal(msg["id"], &id); err != nil {
		t.Fatalf("failed to parse id: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected id 42, got %d", id)
	}
}

func TestReadJSONRPCResponse_NoResponse(t *testing.T) {
	t.Parallel()
	// Only notifications, no response
	input := `{"jsonrpc":"2.0","method":"notifications/resources/list_changed","params":{}}` + "\n"
	scanner := bufio.NewScanner(strings.NewReader(input))

	_, err := readJSONRPCResponse(scanner)
	if err == nil {
		t.Fatal("expected error for missing response, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected end of output") {
		t.Fatalf("expected 'unexpected end of output' error, got: %v", err)
	}
}

func TestReadJSONRPCResponse_EmptyInput(t *testing.T) {
	t.Parallel()
	scanner := bufio.NewScanner(strings.NewReader(""))

	_, err := readJSONRPCResponse(scanner)
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestReadJSONRPCResponse_InvalidJSON(t *testing.T) {
	t.Parallel()
	input := "not valid json\n"
	scanner := bufio.NewScanner(strings.NewReader(input))

	_, err := readJSONRPCResponse(scanner)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse JSON-RPC message") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestBuildInitializeRequest(t *testing.T) {
	t.Parallel()
	got, err := buildInitializeRequest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var msg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(got), &msg); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	// Verify required fields
	for _, field := range []string{"jsonrpc", "id", "method", "params"} {
		if _, ok := msg[field]; !ok {
			t.Errorf("missing required field %q", field)
		}
	}

	// Verify method
	var method string
	if err := json.Unmarshal(msg["method"], &method); err != nil {
		t.Fatalf("failed to parse method: %v", err)
	}
	if method != "initialize" {
		t.Errorf("expected method 'initialize', got %q", method)
	}

	// Verify params contain protocolVersion and clientInfo
	var params map[string]json.RawMessage
	if err := json.Unmarshal(msg["params"], &params); err != nil {
		t.Fatalf("failed to parse params: %v", err)
	}
	for _, field := range []string{"protocolVersion", "capabilities", "clientInfo"} {
		if _, ok := params[field]; !ok {
			t.Errorf("missing params field %q", field)
		}
	}

	var version string
	if err := json.Unmarshal(params["protocolVersion"], &version); err != nil {
		t.Fatalf("failed to parse protocolVersion: %v", err)
	}
	if version != "2024-11-05" {
		t.Errorf("expected protocolVersion '2024-11-05', got %q", version)
	}
}

func TestBuildInitializedNotification(t *testing.T) {
	t.Parallel()
	got := buildInitializedNotification()

	var msg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(got), &msg); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	// Must have jsonrpc and method
	var method string
	if err := json.Unmarshal(msg["method"], &method); err != nil {
		t.Fatalf("failed to parse method: %v", err)
	}
	if method != "notifications/initialized" {
		t.Errorf("expected method 'notifications/initialized', got %q", method)
	}

	// Must NOT have an id (it's a notification)
	if _, hasID := msg["id"]; hasID {
		t.Error("notification should not have an 'id' field")
	}
}
