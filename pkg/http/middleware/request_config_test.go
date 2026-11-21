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

func TestWithRequestConfigRepeatedBooleanHeaders(t *testing.T) {
	headersToTest := []struct {
		name   string
		header string
		get    func(context.Context) bool
	}{
		{"readonly", headers.MCPReadOnlyHeader, ghcontext.IsReadonly},
		{"lockdown", headers.MCPLockdownHeader, ghcontext.IsLockdownMode},
		{"insiders", headers.MCPInsidersHeader, ghcontext.IsInsidersMode},
	}
	tests := []struct {
		name   string
		values []string
		want   bool
	}{
		{"absent", nil, false},
		{"all false", []string{"", " false ", "0", " NO ", "off", "N", "f"}, false},
		{"false then true", []string{"false", "true"}, true},
		{"true then false", []string{"true", "false"}, true},
		{"empty then one", []string{"", "1"}, true},
		{"zero then yes", []string{"0", "yes"}, true},
		{"no then on", []string{"no", "on"}, true},
		{"off then y", []string{"off", "y"}, true},
		{"n then t", []string{"n", "t"}, true},
		{"mixed case and whitespace", []string{" F ", " TrUe "}, true},
		{"arbitrary truthy value", []string{"false", "enabled"}, true},
		{"comma value retains scalar semantics", []string{"false,false"}, true},
	}
	for _, header := range headersToTest {
		t.Run(header.name, func(t *testing.T) {
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					req := httptest.NewRequest(http.MethodGet, "/", nil)
					for _, value := range tc.values {
						req.Header.Add(header.header, value)
					}
					handler := WithRequestConfig(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
						assert.Equal(t, tc.want, header.get(r.Context()))
					}))
					handler.ServeHTTP(httptest.NewRecorder(), req)
				})
			}
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
			name:         "repeated query parameters combine comma separated values",
			url:          "/?features=first,%20second&features=&features=%20third,%20,%20fourth%20",
			wantFeatures: []string{"first", "second", "third", "fourth"},
			wantPresent:  true,
		},
		{
			name:         "empty first query value does not hide later features",
			url:          "/?features=&features=flag_from_query",
			wantFeatures: []string{"flag_from_query"},
			wantPresent:  true,
		},
		{
			name:         "repeated empty query values store an explicit empty selection",
			url:          "/?features=&features=%20,%20",
			wantFeatures: []string{},
			wantPresent:  true,
		},
		{
			name:         "header wins over repeated query parameters",
			url:          "/?features=first&features=second",
			headerSet:    true,
			headerValue:  "flag_from_header",
			wantFeatures: []string{"flag_from_header"},
			wantPresent:  true,
		},
		{
			name:         "empty header suppresses repeated query parameters",
			url:          "/?features=first&features=second",
			headerSet:    true,
			wantFeatures: []string{},
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
