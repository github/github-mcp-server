package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	"github.com/github/github-mcp-server/v2/pkg/http/headers"
	"github.com/stretchr/testify/assert"
)

func TestWithRequestConfigRepeatedListHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header string
		get    func(context.Context) []string
	}{
		{"toolsets", headers.MCPToolsetsHeader, ghcontext.GetToolsets},
		{"tools", headers.MCPToolsHeader, ghcontext.GetTools},
		{"excluded tools", headers.MCPExcludeToolsHeader, ghcontext.GetExcludeTools},
		{"features", headers.MCPFeaturesHeader, ghcontext.GetHeaderFeatures},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?features=query_flag", nil)
			req.Header.Add(tc.header, "first, second")
			req.Header.Add(tc.header, " third, , fourth ")
			handler := WithRequestConfig(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				assert.Equal(t, []string{"first", "second", "third", "fourth"}, tc.get(r.Context()))
			}))
			handler.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
}

func TestWithRequestConfigFeatureSelection(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		headerSet    bool
		headerValue  string
		wantFeatures []string
		wantPresent  bool
	}{
		{
			name:         "query parameter only",
			url:          "/?features=mcp_holdback_consolidated_projects",
			wantFeatures: []string{"mcp_holdback_consolidated_projects"},
			wantPresent:  true,
		},
		{
			name:         "header only",
			url:          "/",
			headerSet:    true,
			headerValue:  "mcp_holdback_consolidated_projects",
			wantFeatures: []string{"mcp_holdback_consolidated_projects"},
			wantPresent:  true,
		},
		{
			name:         "header wins over query parameter, never combined",
			url:          "/?features=flag_from_query",
			headerSet:    true,
			headerValue:  "flag_from_header",
			wantFeatures: []string{"flag_from_header"},
			wantPresent:  true,
		},
		{
			name:         "empty header suppresses query parameter",
			url:          "/?features=flag_from_query",
			headerSet:    true,
			wantFeatures: []string{},
			wantPresent:  true,
		},
		{
			name:         "whitespace-only header suppresses query parameter",
			url:          "/?features=flag_from_query",
			headerSet:    true,
			headerValue:  "  , \t ",
			wantFeatures: []string{},
			wantPresent:  true,
		},
		{
			name:         "unknown header suppresses query parameter",
			url:          "/?features=flag_from_query",
			headerSet:    true,
			headerValue:  "unknown_from_header",
			wantFeatures: []string{"unknown_from_header"},
			wantPresent:  true,
		},
		{
			name:         "empty query value with header",
			url:          "/?features=",
			headerSet:    true,
			headerValue:  "flag_from_header",
			wantFeatures: []string{"flag_from_header"},
			wantPresent:  true,
		},
		{
			name:         "empty query value stores an explicit empty selection",
			url:          "/?features=",
			wantFeatures: []string{},
			wantPresent:  true,
		},
		{
			name:         "no channel present stores nothing",
			url:          "/",
			wantFeatures: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			handler := WithRequestConfig(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = ghcontext.GetHeaderFeatures(r.Context())
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodPost, tc.url, nil)
			if tc.headerSet {
				req.Header.Set(headers.MCPFeaturesHeader, tc.headerValue)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantFeatures, got)
			if tc.wantPresent {
				assert.NotNil(t, got)
			} else {
				assert.Nil(t, got)
			}
			assert.Contains(t, rec.Header().Values(headers.VaryHeader), headers.MCPFeaturesHeader)
		})
	}
}
