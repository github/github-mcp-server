package scopes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetToolScopeMap(t *testing.T) {
	// Reset and set up a test map
	SetGlobalToolScopeMap(ToolScopeMap{
		"test_tool": &ToolScopeInfo{
			RequiredScopes: []string{"read:org"},
			AcceptedScopes: []string{"read:org", "write:org", "admin:org"},
		},
	})

	m, err := GetToolScopeMap()
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Greater(t, len(m), 0, "expected at least one tool in the scope map")

	testTool, ok := m["test_tool"]
	require.True(t, ok, "expected test_tool to be in the scope map")
	assert.Contains(t, testTool.RequiredScopes, "read:org")
	assert.Contains(t, testTool.AcceptedScopes, "read:org")
	assert.Contains(t, testTool.AcceptedScopes, "admin:org")
}

func TestGetToolScopeInfo(t *testing.T) {
	// Set up test scope map
	SetGlobalToolScopeMap(ToolScopeMap{
		"search_orgs": &ToolScopeInfo{
			RequiredScopes: []string{"read:org"},
			AcceptedScopes: []string{"read:org", "write:org", "admin:org"},
		},
	})

	info, err := GetToolScopeInfo("search_orgs")
	require.NoError(t, err)
	require.NotNil(t, info)

	// Non-existent tool should return nil
	info, err = GetToolScopeInfo("nonexistent_tool")
	require.NoError(t, err)
	assert.Nil(t, info)
}

func TestToolScopeInfo_HasAcceptedScope(t *testing.T) {
	testCases := []struct {
		name       string
		scopeInfo  *ToolScopeInfo
		userScopes []string
		expected   bool
	}{
		{
			name: "has exact required scope",
			scopeInfo: &ToolScopeInfo{
				RequiredScopes: []string{"read:org"},
				AcceptedScopes: []string{"read:org", "write:org", "admin:org"},
			},
			userScopes: []string{"read:org"},
			expected:   true,
		},
		{
			name: "has parent scope (admin:org grants read:org)",
			scopeInfo: &ToolScopeInfo{
				RequiredScopes: []string{"read:org"},
				AcceptedScopes: []string{"read:org", "write:org", "admin:org"},
			},
			userScopes: []string{"admin:org"},
			expected:   true,
		},
		{
			name: "has parent scope (write:org grants read:org)",
			scopeInfo: &ToolScopeInfo{
				RequiredScopes: []string{"read:org"},
				AcceptedScopes: []string{"read:org", "write:org", "admin:org"},
			},
			userScopes: []string{"write:org"},
			expected:   true,
		},
		{
			name: "missing required scope",
			scopeInfo: &ToolScopeInfo{
				RequiredScopes: []string{"read:org"},
				AcceptedScopes: []string{"read:org", "write:org", "admin:org"},
			},
			userScopes: []string{"repo"},
			expected:   false,
		},
		{
			name: "no scope required",
			scopeInfo: &ToolScopeInfo{
				RequiredScopes: []string{},
				AcceptedScopes: []string{},
			},
			userScopes: []string{},
			expected:   true,
		},
		{
			name:       "nil scope info",
			scopeInfo:  nil,
			userScopes: []string{},
			expected:   true,
		},
		{
			name: "repo scope for tool requiring repo",
			scopeInfo: &ToolScopeInfo{
				RequiredScopes: []string{"repo"},
				AcceptedScopes: []string{"repo"},
			},
			userScopes: []string{"repo"},
			expected:   true,
		},
		{
			name: "missing repo scope",
			scopeInfo: &ToolScopeInfo{
				RequiredScopes: []string{"repo"},
				AcceptedScopes: []string{"repo"},
			},
			userScopes: []string{"public_repo"},
			expected:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.scopeInfo.HasAcceptedScope(tc.userScopes...)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestToolScopeInfo_MissingScopes(t *testing.T) {
	testCases := []struct {
		name           string
		scopeInfo      *ToolScopeInfo
		userScopes     []string
		expectedLen    int
		expectedScopes []string
	}{
		{
			name: "has required scope - no missing",
			scopeInfo: &ToolScopeInfo{
				RequiredScopes: []string{"read:org"},
				AcceptedScopes: []string{"read:org", "write:org", "admin:org"},
			},
			userScopes:     []string{"read:org"},
			expectedLen:    0,
			expectedScopes: nil,
		},
		{
			name: "missing scope",
			scopeInfo: &ToolScopeInfo{
				RequiredScopes: []string{"read:org"},
				AcceptedScopes: []string{"read:org", "write:org", "admin:org"},
			},
			userScopes:     []string{"repo"},
			expectedLen:    1,
			expectedScopes: []string{"read:org"},
		},
		{
			name: "no scope required - no missing",
			scopeInfo: &ToolScopeInfo{
				RequiredScopes: []string{},
				AcceptedScopes: []string{},
			},
			userScopes:     []string{},
			expectedLen:    0,
			expectedScopes: nil,
		},
		{
			name:           "nil scope info - no missing",
			scopeInfo:      nil,
			userScopes:     []string{},
			expectedLen:    0,
			expectedScopes: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			missing := tc.scopeInfo.MissingScopes(tc.userScopes...)
			assert.Len(t, missing, tc.expectedLen)
			if tc.expectedScopes != nil {
				for _, expected := range tc.expectedScopes {
					assert.Contains(t, missing, expected)
				}
			}
		})
	}
}
