package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/google/go-querystring/query"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func ListProjects(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_projects",
			mcp.WithDescription(t("TOOL_LIST_PROJECTS_DESCRIPTION", "List Projects for a user or organization")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_LIST_PROJECTS_USER_TITLE", "List projects"), ReadOnlyHint: ToBoolPtr(true)}),
			mcp.WithString("owner_type", mcp.Required(), mcp.Description("Owner type"), mcp.Enum("user", "organization")),
			mcp.WithString("owner", mcp.Required(), mcp.Description("If owner_type == user it is the handle for the GitHub user account. If owner_type == organization it is the name of the organization. The name is not case sensitive.")),
			mcp.WithString("query", mcp.Description("Filter projects by a search query (matches title and description)")),
			mcp.WithString("before", mcp.Description("Cursor for items before (backwards pagination)")),
			mcp.WithString("after", mcp.Description("Cursor for items after (forward pagination)")),
			mcp.WithNumber("per_page", mcp.Description("Number of results per page (max 100, default: 30)")),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ownerType, err := RequiredParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			queryStr, err := OptionalParam[string](req, "query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			beforeCursor, err := OptionalParam[string](req, "before")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			afterCursor, err := OptionalParam[string](req, "after")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			perPage, err := OptionalIntParamWithDefault(req, "per_page", 30)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var url string
			if ownerType == "organization" {
				url = fmt.Sprintf("/orgs/%s/projectsV2", owner)
			} else {
				url = fmt.Sprintf("/users/%s/projectsV2", owner)
			}
			projects := []github.ProjectV2{}

			opts := ListProjectsOptions{PerPage: perPage}
			if afterCursor != "" {
				opts.After = afterCursor
			}
			if beforeCursor != "" {
				opts.Before = beforeCursor
			}
			if queryStr != "" {
				opts.Query = queryStr
			}
			url, err = addOptions(url, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to add options to request: %w", err)
			}

			httpRequest, err := client.NewRequest("GET", url, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}

			resp, err := client.Do(ctx, httpRequest, &projects)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list projects",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list projects: %s", string(body))), nil
			}
			r, err := json.Marshal(projects)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

type ListProjectsOptions struct {
	// A cursor, as given in the Link header. If specified, the query only searches for events before this cursor.
	Before string `url:"before,omitempty"`

	// A cursor, as given in the Link header. If specified, the query only searches for events after this cursor.
	After string `url:"after,omitempty"`

	// For paginated result sets, the number of results to include per page.
	PerPage int `url:"per_page,omitempty"`

	// Query Limit results to projects of the specified type.
	Query string `url:"q,omitempty"`
}

// addOptions adds the parameters in opts as URL query parameters to s. opts
// must be a struct whose fields may contain "url" tags.
func addOptions(s string, opts any) (string, error) {
	v := reflect.ValueOf(opts)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return s, nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return s, err
	}

	qs, err := query.Values(opts)
	if err != nil {
		return s, err
	}

	u.RawQuery = qs.Encode()
	return u.String(), nil
}
