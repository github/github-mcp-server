package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// GitHubInProcessTransport creates an in-process transport that uses our GitHubMCPServer
// This ensures that completion requests go through our HandleMessage override
type GitHubInProcessTransport struct {
	server               *GitHubMCPServer
	notificationHandler  func(mcp.JSONRPCNotification)
}

// NewGitHubInProcessTransport creates a new in-process transport for GitHubMCPServer
func NewGitHubInProcessTransport(server *GitHubMCPServer) *GitHubInProcessTransport {
	return &GitHubInProcessTransport{
		server: server,
	}
}

func (c *GitHubInProcessTransport) Start(ctx context.Context) error {
	return nil
}

func (c *GitHubInProcessTransport) SendRequest(ctx context.Context, request transport.JSONRPCRequest) (*transport.JSONRPCResponse, error) {
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	requestBytes = append(requestBytes, '\n')

	// This is the key part: call HandleMessage on our GitHubMCPServer
	// which will route completion requests to our handler
	respMessage := c.server.HandleMessage(ctx, requestBytes)
	respByte, err := json.Marshal(respMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response message: %w", err)
	}
	rpcResp := transport.JSONRPCResponse{}
	err = json.Unmarshal(respByte, &rpcResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response message: %w", err)
	}

	return &rpcResp, nil
}

func (c *GitHubInProcessTransport) SendNotification(ctx context.Context, notification mcp.JSONRPCNotification) error {
	// For in-process transport, we can just forward notifications to the handler
	if c.notificationHandler != nil {
		c.notificationHandler(notification)
	}
	return nil
}

func (c *GitHubInProcessTransport) SetNotificationHandler(handler func(mcp.JSONRPCNotification)) {
	c.notificationHandler = handler
}

func (c *GitHubInProcessTransport) Close() error {
	return nil
}

// NewInProcessClientWithGitHubServer creates a client that works with GitHubMCPServer
func NewInProcessClientWithGitHubServer(server *GitHubMCPServer) (*client.Client, error) {
	ghTransport := NewGitHubInProcessTransport(server)
	return client.NewClient(ghTransport), nil
}