//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os/exec"
	"testing"
	"time"
)

func TestMultiUserHTTPServer_Integration(t *testing.T) {
	// Start the server in multi-user mode on a random port (e.g. 18080)
	cmd := exec.Command("../cmd/github-mcp-server/github-mcp-server", "multi-user", "--port", "18080")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer cmd.Process.Kill()
	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Make a request without Authorization header
	resp, err := http.Post("http://localhost:18080/v1/mcp", "application/json", bytes.NewBufferString(`{"test":"noauth"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", resp.StatusCode)
	}

	// Make a request with Authorization header
	body, _ := json.Marshal(map[string]string{"test": "authed"})
	req, _ := http.NewRequest("POST", "http://localhost:18080/v1/mcp", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer testtoken123")
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp2.StatusCode == http.StatusUnauthorized {
		t.Errorf("expected not 401, got 401 (token should be accepted if server is running and token is valid)")
	}
}
