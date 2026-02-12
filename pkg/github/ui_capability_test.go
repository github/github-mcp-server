package github

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestClientSupportsUI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      *mcp.CallToolRequest
		expected bool
	}{
		{
			name:     "nil request",
			req:      nil,
			expected: false,
		},
		{
			name:     "nil session",
			req:      &mcp.CallToolRequest{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, clientSupportsUI(tt.req))
		})
	}
}
