package github

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func Test_clientSupportsUI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		clientName string
		want       bool
	}{
		{name: "VS Code Insiders", clientName: "Visual Studio Code - Insiders", want: true},
		{name: "VS Code Stable", clientName: "Visual Studio Code", want: true},
		{name: "unknown client", clientName: "some-other-client", want: false},
		{name: "empty client name", clientName: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createMCPRequestWithSession(t, tt.clientName, nil)
			assert.Equal(t, tt.want, clientSupportsUI(&req))
		})
	}

	t.Run("nil request", func(t *testing.T) {
		assert.False(t, clientSupportsUI(nil))
	})

	t.Run("nil session", func(t *testing.T) {
		req := createMCPRequest(nil)
		assert.False(t, clientSupportsUI(&req))
	})
}

func Test_clientSupportsUI_nilClientInfo(t *testing.T) {
	t.Parallel()

	srv := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	st, _ := mcp.NewInMemoryTransports()
	session, err := srv.Connect(context.Background(), st, &mcp.ServerSessionOptions{
		State: &mcp.ServerSessionState{
			InitializeParams: &mcp.InitializeParams{
				ClientInfo: nil,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })

	req := mcp.CallToolRequest{Session: session}
	assert.False(t, clientSupportsUI(&req))
}
