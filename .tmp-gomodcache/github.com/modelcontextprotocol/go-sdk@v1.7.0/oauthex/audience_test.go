// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package oauthex

import "testing"

func TestMatchesResource(t *testing.T) {
	tests := []struct {
		name     string
		claims   []string
		resource string
		want     bool
	}{
		{
			name:     "exact match",
			claims:   []string{"https://mcp.example.com/"},
			resource: "https://mcp.example.com/",
			want:     true,
		},
		{
			name:     "claim has trailing slash, resource does not",
			claims:   []string{"https://mcp.example.com/"},
			resource: "https://mcp.example.com",
			want:     true,
		},
		{
			name:     "claim missing trailing slash, resource has one",
			claims:   []string{"https://mcp.example.com"},
			resource: "https://mcp.example.com/",
			want:     true,
		},
		{
			name:     "multiple claims, one matches",
			claims:   []string{"https://other.example.com/", "https://mcp.example.com"},
			resource: "https://mcp.example.com/",
			want:     true,
		},
		{
			name:     "no claims",
			claims:   nil,
			resource: "https://mcp.example.com/",
			want:     false,
		},
		{
			name:     "claim does not match resource",
			claims:   []string{"https://attacker.example.com/"},
			resource: "https://mcp.example.com/",
			want:     false,
		},
		{
			name:     "path mismatch is not tolerated",
			claims:   []string{"https://mcp.example.com/v2"},
			resource: "https://mcp.example.com",
			want:     false,
		},
		{
			name:     "scheme mismatch is not tolerated",
			claims:   []string{"http://mcp.example.com/"},
			resource: "https://mcp.example.com/",
			want:     false,
		},
		// The following document the intentional boundaries: §6.2.3 also
		// permits scheme/host case folding and default-port elision, but
		// MatchesResource deliberately does NOT, so two distinct
		// registered resources cannot collide via these normalizations.
		{
			name:     "host case difference is not tolerated",
			claims:   []string{"https://MCP.example.com/"},
			resource: "https://mcp.example.com/",
			want:     false,
		},
		{
			name:     "default-port elision is not tolerated",
			claims:   []string{"https://mcp.example.com:443/"},
			resource: "https://mcp.example.com/",
			want:     false,
		},
		{
			name:     "query string difference is not tolerated",
			claims:   []string{"https://mcp.example.com/?x=y"},
			resource: "https://mcp.example.com/",
			want:     false,
		},
		{
			name:     "surrounding whitespace is not tolerated (malformed claim)",
			claims:   []string{"  https://mcp.example.com/  "},
			resource: "https://mcp.example.com/",
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesResource(tt.claims, tt.resource)
			if got != tt.want {
				t.Errorf("MatchesResource(%v, %q) = %v, want %v", tt.claims, tt.resource, got, tt.want)
			}
		})
	}
}
