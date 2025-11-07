package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DocsSearchResult represents a single search result from GitHub Docs
type DocsSearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Breadcrumbs string `json:"breadcrumbs"`
	Content     string `json:"content,omitempty"`
}

// DocsSearchResponse represents the response from GitHub Docs search API
type DocsSearchResponse struct {
	Meta struct {
		Found struct {
			Value int `json:"value"`
		} `json:"found"`
		Took struct {
			PrettyMs string `json:"pretty_ms"`
		} `json:"took"`
	} `json:"meta"`
	Hits []DocsSearchResult `json:"hits"`
}

// SearchGitHubDocs creates a tool to search GitHub documentation.
func SearchGitHubDocs(t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("search_github_docs",
			mcp.WithDescription(t("TOOL_SEARCH_GITHUB_DOCS_DESCRIPTION", "Search GitHub's official documentation at docs.github.com. Use this to find help articles, guides, and API documentation for GitHub features and products.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_SEARCH_GITHUB_DOCS_USER_TITLE", "Search GitHub Docs"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query for GitHub documentation. Examples: 'actions workflow syntax', 'pull request review', 'GitHub Pages'"),
			),
			mcp.WithString("version",
				mcp.Description("GitHub version to search. Options: 'dotcom' (default, free/pro/team), 'ghec' (GitHub Enterprise Cloud), or a specific GHES version like '3.12'"),
			),
			mcp.WithString("language",
				mcp.Description("Language code for documentation. Options: 'en' (default), 'es', 'ja', 'pt', 'zh', 'ru', 'fr', 'ko', 'de'"),
			),
			mcp.WithNumber("max_results",
				mcp.Description("Maximum number of results to return (default: 10, max: 100)"),
			),
		),
		func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, err := RequiredParam[string](request, "query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			version, err := OptionalParam[string](request, "version")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if version == "" {
				version = "dotcom"
			}

			language, err := OptionalParam[string](request, "language")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if language == "" {
				language = "en"
			}

			maxResults, err := OptionalIntParam(request, "max_results")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Check if max_results was explicitly provided
			_, maxResultsProvided := request.GetArguments()["max_results"]
			if maxResultsProvided {
				// Validate max_results only if it was provided
				if maxResults < 1 || maxResults > 100 {
					return mcp.NewToolResultError("max_results must be between 1 and 100"), nil
				}
			} else {
				// Use default if not provided
				maxResults = 10
			}

			// Build the search URL with client_name parameter
			searchURL := fmt.Sprintf("https://docs.github.com/api/search/v1?version=%s&language=%s&query=%s&limit=%d&client_name=github-mcp-server",
				url.QueryEscape(version),
				url.QueryEscape(language),
				url.QueryEscape(query),
				maxResults,
			)

			// Make the HTTP request
			// #nosec G107 - URL is constructed from validated parameters with proper escaping
			resp, err := http.Get(searchURL)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to search GitHub Docs: %v", err)), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return mcp.NewToolResultError(fmt.Sprintf("GitHub Docs API returned status %d: %s", resp.StatusCode, string(body))), nil
			}

			// Parse the response
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to read response body: %v", err)), nil
			}

			var searchResp DocsSearchResponse
			if err := json.Unmarshal(body, &searchResp); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to parse response: %v", err)), nil
			}

			// Format the results
			result := map[string]interface{}{
				"total_results": searchResp.Meta.Found.Value,
				"search_time":   searchResp.Meta.Took.PrettyMs,
				"results":       searchResp.Hits,
			}

			resultJSON, err := json.Marshal(result)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to format results: %v", err)), nil
			}

			return mcp.NewToolResultText(string(resultJSON)), nil
		}
}
