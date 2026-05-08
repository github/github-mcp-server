package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// IssueField represents an organization-level issue field definition.
type IssueField struct {
	ID          int64                          `json:"id"`
	NodeID      string                         `json:"node_id"`
	Name        string                         `json:"name"`
	Description string                         `json:"description,omitempty"`
	DataType    string                         `json:"data_type"`
	Visibility  string                         `json:"visibility"`
	Options     []IssueSingleSelectFieldOption `json:"options,omitempty"`
}

// IssueSingleSelectFieldOption represents an option for a single_select issue field.
type IssueSingleSelectFieldOption struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color"`
	Priority    int64  `json:"priority"`
}

// ListOrgIssueFields creates a tool to list issue field definitions for an organization.
func ListOrgIssueFields(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "list_org_issue_fields",
			Description: t("TOOL_LIST_ORG_ISSUE_FIELDS_DESCRIPTION", "List issue fields for an organization. Returns field definitions including name, type (text, number, date, single_select), and for single_select fields the list of valid option names."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_ORG_ISSUE_FIELDS_USER_TITLE", "List organization issue fields"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"org": {
						Type:        "string",
						Description: "The organization name. The name is not case sensitive.",
					},
				},
				Required: []string{"org"},
			},
		},
		[]scopes.Scope{scopes.ReadOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			org, err := RequiredParam[string](args, "org")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			reqURL := fmt.Sprintf("orgs/%s/issue-fields", org)
			req, err := client.NewRequest(http.MethodGet, reqURL, nil)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
			}

			var fields []*IssueField
			resp, err := client.Do(ctx, req, &fields)
			if err != nil {
				if resp != nil && resp.StatusCode == http.StatusNotFound {
					// Org doesn't have issue fields enabled — return empty list
					result, marshalErr := json.Marshal([]*IssueField{})
					if marshalErr != nil {
						return utils.NewToolResultErrorFromErr("failed to marshal response", marshalErr), nil, nil
					}
					return utils.NewToolResultText(string(result)), nil, nil
				}
				return utils.NewToolResultErrorFromErr("failed to list issue fields", err), nil, nil
			}

			r, err := json.Marshal(fields)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal issue fields", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
}
