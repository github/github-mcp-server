package github

// This file holds the legacy (non-multi-select) variants of the issue-fields
// write tools. They are served when FeatureFlagIssueFieldsMultiSelect is off
// and are intended to be deleted in their entirety once the flag graduates.
//
// Each legacy variant mirrors the structure of its multi-select-aware sibling
// (IssueWrite, GranularSetIssueFields, ListIssues, ListIssueFields) but with
// the multi-select schema slots and description text removed. The shared parser
// and resolver branch on a single bool flag — see optionalIssueWriteFields and
// resolveIssueRequestFieldValues. There is no "interleaved" feature-flag check
// in the handler bodies themselves: each tool variant is wired to one constant
// branch of those helpers at registration time.

import (
	"context"
	"fmt"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v87/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// IssueWriteLegacy is the FF-off variant of issue_write. It does not accept
// multi-select inputs and its schema does not mention multi-select. It is
// registered under the same tool name as IssueWrite with mutually exclusive
// feature-flag annotations.
func IssueWriteLegacy(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "issue_write",
			Description: t("TOOL_ISSUE_WRITE_DESCRIPTION", "Create a new or update an existing issue in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ISSUE_WRITE_USER_TITLE", "Create or update issue/pull request"),
				ReadOnlyHint: false,
			},
			Meta: mcp.Meta{
				"ui": map[string]any{
					"resourceUri": IssueWriteUIResourceURI,
					"visibility":  []string{"model", "app"},
				},
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: `Write operation to perform on a single issue.
Options are:
- 'create' - creates a new issue.
- 'update' - updates an existing issue.
`,
						Enum: []any{"create", "update"},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"issue_number": {
						Type:        "number",
						Description: "Issue number to update",
					},
					"title": {
						Type:        "string",
						Description: "Issue title",
					},
					"body": {
						Type:        "string",
						Description: "Issue body content",
					},
					"assignees": {
						Type:        "array",
						Description: "Usernames to assign to this issue",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"labels": {
						Type:        "array",
						Description: "Labels to apply to this issue",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"milestone": {
						Type:        "number",
						Description: "Milestone number",
					},
					"type": {
						Type:        "string",
						Description: "Type of this issue. Only use if issue types are enabled for this repository. Use list_issue_types tool to get valid type values for this repository or its owner organization. If the repository doesn't support issue types, omit this parameter.",
					},
					"state": {
						Type:        "string",
						Description: "New state",
						Enum:        []any{"open", "closed"},
					},
					"state_reason": {
						Type:        "string",
						Description: "Reason for the state change. Ignored unless state is changed.",
						Enum:        []any{"completed", "not_planned", "duplicate"},
					},
					"duplicate_of": {
						Type:        "number",
						Description: "Issue number that this issue is a duplicate of. Only used when state_reason is 'duplicate'.",
					},
					"issue_fields": {
						Type:        "array",
						Description: "Issue field values to set or clear. Each item requires 'field_name' and exactly one of 'value', 'field_option_name', or 'delete: true'.",
						Items: &jsonschema.Schema{
							Type:                 "object",
							AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
							Properties: map[string]*jsonschema.Schema{
								"field_name": {
									Type: "string",
									Description: "Issue field name (case-insensitive). Must match a field " +
										"returned by list_issue_fields for this repository or its organization.",
								},
								"value": {
									Types: []string{"string", "number", "boolean"},
									Description: "Value to set. Use for text, number, and date fields " +
										"(date as YYYY-MM-DD). For single-select fields, prefer " +
										"'field_option_name' so the option is validated before the API " +
										"call. Cannot be combined with 'field_option_name' or 'delete'.",
								},
								"field_option_name": {
									Type: "string",
									Description: "Option name for single-select fields. Validated against " +
										"the field's options before the API call. Cannot be combined with " +
										"'value' or 'delete'.",
								},
								"delete": {
									Type: "boolean",
									Enum: []any{true},
									Description: "Set to true to clear this field's current value on the " +
										"issue. Cannot be combined with 'value' or 'field_option_name'.",
								},
							},
							Required: []string{"field_name"},
						},
					},
				},
				Required: []string{"method", "owner", "repo"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Hand off to the interactive MCP App form unless this call must
			// execute now (see shouldDeferToForm).
			if shouldDeferToForm(ctx, deps, req, args, issueWriteFormParams) {
				issueNumber := 0
				if method == "update" {
					n, numErr := RequiredInt(args, "issue_number")
					if numErr != nil {
						return utils.NewToolResultError("issue_number is required for update method"), nil, nil
					}
					issueNumber = n
				}
				return issueWriteAwaitingFormResult(method, owner, repo, issueNumber), nil, nil
			}

			title, err := OptionalParam[string](args, "title")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			body, err := OptionalParam[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			assignees, err := OptionalStringArrayParam(args, "assignees")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			assigneesValue, assigneesProvided := args["assignees"]
			assigneesProvided = assigneesProvided && assigneesValue != nil

			labels, err := OptionalStringArrayParam(args, "labels")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			labelsValue, labelsProvided := args["labels"]
			labelsProvided = labelsProvided && labelsValue != nil

			milestone, err := OptionalIntParam(args, "milestone")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var milestoneNum int
			if milestone != 0 {
				milestoneNum = milestone
			}

			issueType, err := OptionalParam[string](args, "type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			stateReason, err := OptionalParam[string](args, "state_reason")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			duplicateOf, err := OptionalIntParam(args, "duplicate_of")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if duplicateOf != 0 && stateReason != "duplicate" {
				return utils.NewToolResultError("duplicate_of can only be used when state_reason is 'duplicate'"), nil, nil
			}

			var issueFields []issueWriteFieldInput
			issueFields, err = optionalIssueWriteFields(args, false)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GraphQL client", err), nil, nil
			}

			var issueFieldValues []*github.IssueRequestFieldValue
			var fieldIDsToDelete []int64
			if len(issueFields) > 0 {
				issueFieldValues, fieldIDsToDelete, err = resolveIssueRequestFieldValues(ctx, gqlClient, owner, repo, issueFields, false)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("failed to resolve issue_fields: %v", err)), nil, nil
				}
			}

			switch method {
			case "create":
				result, err := CreateIssue(ctx, client, owner, repo, title, body, assignees, labels, milestoneNum, issueType, issueFieldValues)
				return result, nil, err
			case "update":
				issueNumber, err := RequiredInt(args, "issue_number")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				result, err := UpdateIssue(ctx, client, gqlClient, owner, repo, issueNumber, title, body, assignees, labels, milestoneNum, issueType, issueFieldValues, fieldIDsToDelete, state, stateReason, duplicateOf, UpdateIssueOptions{
					AssigneesProvided: assigneesProvided,
					LabelsProvided:    labelsProvided,
				})
				return result, nil, err
			default:
				return utils.NewToolResultError("invalid method, must be either 'create' or 'update'"), nil, nil
			}
		})
	st.FeatureFlagDisable = []string{FeatureFlagIssuesGranular, FeatureFlagIssueFieldsMultiSelect}
	return st
}
