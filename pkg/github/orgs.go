package github

import (
	"context"
	"encoding/json"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const orgMembershipVisibilityNote = "result reflects caller visibility; private members of orgs you can't see appear as non-members."

type CheckOrgMembershipInput struct {
	Org      string `json:"org"`
	Username string `json:"username"`
}

type CheckOrgMembershipOutput struct {
	Org        string `json:"org"`
	Username   string `json:"username"`
	IsMember   bool   `json:"isMember"`
	Visibility string `json:"visibility"`
	Note       string `json:"note,omitempty"`
}

// CheckOrgMembership creates a tool to check whether a GitHub user is a member of an organization.
func CheckOrgMembership(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[CheckOrgMembershipInput, CheckOrgMembershipOutput](
		ToolsetMetadataOrgs,
		mcp.Tool{
			Name:        "check_org_membership",
			Description: t("TOOL_CHECK_ORG_MEMBERSHIP_DESCRIPTION", "Check whether a GitHub user is a member of an organization, and report whether that membership is public, private, or not visible."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CHECK_ORG_MEMBERSHIP_USER_TITLE", "Check organization membership"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"org": {
						Type:        "string",
						Description: "GitHub organization login",
					},
					"username": {
						Type:        "string",
						Description: "GitHub username to check",
					},
				},
				Required: []string{"org", "username"},
			},
		},
		[]scopes.Scope{scopes.ReadOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args CheckOrgMembershipInput) (*mcp.CallToolResult, CheckOrgMembershipOutput, error) {
			if args.Org == "" {
				return utils.NewToolResultError("missing required parameter: org"), CheckOrgMembershipOutput{}, nil
			}
			if args.Username == "" {
				return utils.NewToolResultError("missing required parameter: username"), CheckOrgMembershipOutput{}, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), CheckOrgMembershipOutput{}, nil
			}

			isMember, res, err := client.Organizations.IsMember(ctx, args.Org, args.Username)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to check organization membership",
					res,
					err,
				), CheckOrgMembershipOutput{}, nil
			}

			isPublicMember, res, err := client.Organizations.IsPublicMember(ctx, args.Org, args.Username)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to check public organization membership",
					res,
					err,
				), CheckOrgMembershipOutput{}, nil
			}

			output := CheckOrgMembershipOutput{
				Org:      args.Org,
				Username: args.Username,
			}
			switch {
			case isPublicMember:
				output.IsMember = true
				output.Visibility = "public"
			case isMember:
				output.IsMember = true
				output.Visibility = "private"
			default:
				if errResult := verifyOrganizationExists(ctx, args.Org, deps); errResult != nil {
					return errResult, CheckOrgMembershipOutput{}, nil
				}
				output.Visibility = "none"
				output.Note = orgMembershipVisibilityNote
			}

			r, err := json.Marshal(output)
			if err != nil {
				return nil, CheckOrgMembershipOutput{}, err
			}
			return utils.NewToolResultText(string(r)), output, nil
		},
	)
}

func verifyOrganizationExists(ctx context.Context, org string, deps ToolDependencies) *mcp.CallToolResult {
	client, err := deps.GetClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub client", err)
	}

	_, res, err := client.Organizations.Get(ctx, org)
	if err == nil {
		return nil
	}
	if res != nil && res.Response != nil && res.Response.StatusCode == http.StatusNotFound {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get organization",
			res,
			err,
		)
	}

	return ghErrors.NewGitHubAPIErrorResponse(ctx,
		"failed to verify organization exists",
		res,
		err,
	)
}
