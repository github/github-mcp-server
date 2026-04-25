package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/headers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphQLFeaturesTransport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		features       []string
		expectedHeader string
		hasHeader      bool
	}{
		{
			name:           "no features in context",
			features:       nil,
			expectedHeader: "",
			hasHeader:      false,
		},
		{
			name:           "single feature in context",
			features:       []string{"issues_copilot_assignment_api_support"},
			expectedHeader: "issues_copilot_assignment_api_support",
			hasHeader:      true,
		},
		{
			name:           "multiple features in context",
			features:       []string{"feature1", "feature2", "feature3"},
			expectedHeader: "feature1, feature2, feature3",
			hasHeader:      true,
		},
		{
			name:           "empty features slice",
			features:       []string{},
			expectedHeader: "",
			hasHeader:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var capturedHeader string
			var headerExists bool

			// Create a test server that captures the request header
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedHeader = r.Header.Get(headers.GraphQLFeaturesHeader)
				headerExists = r.Header.Get(headers.GraphQLFeaturesHeader) != ""
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Create the transport
			transport := &GraphQLFeaturesTransport{
				Transport: http.DefaultTransport,
			}

			// Create a request
			ctx := context.Background()
			if tc.features != nil {
				ctx = ghcontext.WithGraphQLFeatures(ctx, tc.features...)
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, nil)
			require.NoError(t, err)

			// Execute the request
			resp, err := transport.RoundTrip(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Verify the header
			assert.Equal(t, tc.hasHeader, headerExists)
			if tc.hasHeader {
				assert.Equal(t, tc.expectedHeader, capturedHeader)
			}
		})
	}
}

func TestGraphQLFeaturesTransport_NilTransport(t *testing.T) {
	t.Parallel()

	var capturedHeader string

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get(headers.GraphQLFeaturesHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create the transport with nil Transport (should use DefaultTransport)
	transport := &GraphQLFeaturesTransport{
		Transport: nil,
	}

	// Create a request with features
	ctx := ghcontext.WithGraphQLFeatures(context.Background(), "test_feature")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, nil)
	require.NoError(t, err)

	// Execute the request
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify the header was added
	assert.Equal(t, "test_feature", capturedHeader)
}

func TestGraphQLFeaturesTransport_DoesNotMutateOriginalRequest(t *testing.T) {
	t.Parallel()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create the transport
	transport := &GraphQLFeaturesTransport{
		Transport: http.DefaultTransport,
	}

	// Create a request with features
	ctx := ghcontext.WithGraphQLFeatures(context.Background(), "test_feature")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, nil)
	require.NoError(t, err)

	// Store the original header value
	originalHeader := req.Header.Get(headers.GraphQLFeaturesHeader)

	// Execute the request
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify the original request was not mutated
	assert.Equal(t, originalHeader, req.Header.Get(headers.GraphQLFeaturesHeader))
}
