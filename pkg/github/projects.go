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
			mcp.WithDescription(t("TOOL_LIST_PROJECTS_DESCRIPTION", "List Projects for a user or org")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_LIST_PROJECTS_USER_TITLE", "List projects"), ReadOnlyHint: ToBoolPtr(true)}),
			mcp.WithString("owner_type", mcp.Required(), mcp.Description("Owner type"), mcp.Enum("user", "org")),
			mcp.WithString("owner", mcp.Required(), mcp.Description("If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive.")),
			mcp.WithString("query", mcp.Description("Filter projects by a search query (matches title and description)")),
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
			perPage, err := OptionalIntParamWithDefault(req, "per_page", 30)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var url string
			if ownerType == "org" {
				url = fmt.Sprintf("orgs/%s/projectsV2", owner)
			} else {
				url = fmt.Sprintf("users/%s/projectsV2", owner)
			}
			projects := []github.ProjectV2{}

			opts := listProjectsOptions{PerPage: perPage}

			if queryStr != "" {
				opts.Query = queryStr
			}
			if perPage > 0 {
				opts.PerPage = perPage
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

func GetProject(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_project",
			mcp.WithDescription(t("TOOL_GET_PROJECT_DESCRIPTION", "Get Project for a user or org")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_GET_PROJECT_USER_TITLE", "Get project"), ReadOnlyHint: ToBoolPtr(true)}),
			mcp.WithNumber("project_number", mcp.Required(), mcp.Description("The project's number")),
			mcp.WithString("owner_type", mcp.Required(), mcp.Description("Owner type"), mcp.Enum("user", "org")),
			mcp.WithString("owner", mcp.Required(), mcp.Description("If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive.")),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

			projectNumber, err := RequiredInt(req, "project_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			ownerType, err := RequiredParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var url string
			if ownerType == "org" {
				url = fmt.Sprintf("orgs/%s/projectsV2/%d", owner, projectNumber)
			} else {
				url = fmt.Sprintf("users/%s/projectsV2/%d", owner, projectNumber)
			}

			project := github.ProjectV2{}

			httpRequest, err := client.NewRequest("GET", url, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}

			resp, err := client.Do(ctx, httpRequest, &project)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get project",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get project: %s", string(body))), nil
			}
			r, err := json.Marshal(project)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func ListProjectFields(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_project_fields",
			mcp.WithDescription(t("TOOL_LIST_PROJECT_FIELDS_DESCRIPTION", "List Project fields for a user or org")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_LIST_PROJECT_FIELDS_USER_TITLE", "List project fields"), ReadOnlyHint: ToBoolPtr(true)}),
			mcp.WithString("owner_type", mcp.Required(), mcp.Description("Owner type"), mcp.Enum("user", "org")),
			mcp.WithString("owner", mcp.Required(), mcp.Description("If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive.")),
			mcp.WithString("projectNumber", mcp.Required(), mcp.Description("The project's number.")),
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
			projectNumber, err := RequiredParam[string](req, "projectNumber")
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
			if ownerType == "org" {
				url = fmt.Sprintf("orgs/%s/projectsV2/%s/fields", owner, projectNumber)
			} else {
				url = fmt.Sprintf("users/%s/projectsV2/%s/fields", owner, projectNumber)
			}
			projectFields := []projectV2Field{}

			opts := listProjectsOptions{PerPage: perPage}

			if perPage > 0 {
				opts.PerPage = perPage
			}
			url, err = addOptions(url, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to add options to request: %w", err)
			}

			httpRequest, err := client.NewRequest("GET", url, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}

			resp, err := client.Do(ctx, httpRequest, &projectFields)
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
			r, err := json.Marshal(projectFields)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

type projectV2Field struct {
	ID        *int64            `json:"id,omitempty"`         // The unique identifier for this field.
	NodeID    string            `json:"node_id,omitempty"`    // The GraphQL node ID for this field.
	Name      string            `json:"name,omitempty"`       // The display name of the field.
	DataType  string            `json:"dataType,omitempty"`   // The data type of the field (e.g., "text", "number", "date", "single_select", "multi_select").
	URL       string            `json:"url,omitempty"`        // The API URL for this field.
	Options   []*any            `json:"options,omitempty"`    // Available options for single_select and multi_select fields.
	CreatedAt *github.Timestamp `json:"created_at,omitempty"` // The time when this field was created.
	UpdatedAt *github.Timestamp `json:"updated_at,omitempty"` // The time when this field was last updated.
}

type listProjectsOptions struct {
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
