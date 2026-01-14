package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphQLFeaturesTransport(t *testing.T) {
	tests := []struct {
		name              string
		features          []string
		expectHeader      bool
		expectedHeaderVal string
	}{
		{
			name:              "adds single feature to header",
			features:          []string{"issues_copilot_assignment_api_support"},
			expectHeader:      true,
			expectedHeaderVal: "issues_copilot_assignment_api_support",
		},
		{
			name:              "adds multiple features to header",
			features:          []string{"feature1", "feature2", "feature3"},
			expectHeader:      true,
			expectedHeaderVal: "feature1, feature2, feature3",
		},
		{
			name:         "no header when no features in context",
			features:     nil,
			expectHeader: false,
		},
		{
			name:         "no header when empty features slice",
			features:     []string{},
			expectHeader: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server that captures the request
			var capturedReq *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedReq = r
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Create HTTP client with GraphQLFeaturesTransport
			client := &http.Client{
				Transport: &GraphQLFeaturesTransport{
					Transport: http.DefaultTransport,
				},
			}

			// Create request with or without features in context
			ctx := context.Background()
			if tt.features != nil {
				ctx = withGraphQLFeatures(ctx, tt.features...)
			}

			req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
			require.NoError(t, err)

			// Make request
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Verify header
			if tt.expectHeader {
				assert.Equal(t, tt.expectedHeaderVal, capturedReq.Header.Get("GraphQL-Features"))
			} else {
				assert.Empty(t, capturedReq.Header.Get("GraphQL-Features"))
			}
		})
	}
}

func TestGraphQLFeaturesTransport_NilTransport(t *testing.T) {
	// Test that nil Transport falls back to http.DefaultTransport
	var capturedReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{
		Transport: &GraphQLFeaturesTransport{
			Transport: nil, // Explicitly nil
		},
	}

	ctx := withGraphQLFeatures(context.Background(), "test_feature")
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "test_feature", capturedReq.Header.Get("GraphQL-Features"))
}

func TestGraphQLFeaturesTransport_PreservesOtherHeaders(t *testing.T) {
	// Test that the transport doesn't interfere with other headers
	var capturedReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{
		Transport: &GraphQLFeaturesTransport{
			Transport: http.DefaultTransport,
		},
	}

	ctx := withGraphQLFeatures(context.Background(), "feature1")
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	require.NoError(t, err)

	// Add custom headers
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("User-Agent", "test-agent")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify all headers are preserved
	assert.Equal(t, "feature1", capturedReq.Header.Get("GraphQL-Features"))
	assert.Equal(t, "Bearer test-token", capturedReq.Header.Get("Authorization"))
	assert.Equal(t, "test-agent", capturedReq.Header.Get("User-Agent"))
}
