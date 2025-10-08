package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/github/github-mcp-server/pkg/translations"
)

// CreateOrgInvitation creates a new invitation for a user to join an organization
func CreateOrgInvitation(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("create_org_invitation",
			mcp.WithDescription(t("TOOL_CREATE_ORG_INVITATION_DESCRIPTION", "Invite a user to join an organization by GitHub user ID or email address. Requires organization owner permissions. This endpoint triggers notifications and may be subject to rate limiting.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CREATE_ORG_INVITATION", "Create Organization Invitation"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("org",
				mcp.Required(),
				mcp.Description("The organization name (not case sensitive)"),
			),
			mcp.WithNumber("invitee_id",
				mcp.Description("GitHub user ID for the person you are inviting. Required unless email is provided."),
			),
			mcp.WithString("email",
				mcp.Description("Email address of the person you are inviting. Required unless invitee_id is provided."),
			),
			mcp.WithString("role",
				mcp.Description("The role for the new member"),
				mcp.Enum("admin", "direct_member", "billing_manager", "reinstate"),
				mcp.DefaultString("direct_member"),
			),
			mcp.WithArray("team_ids",
				mcp.Description("Team IDs to invite new members to"),
				mcp.Items(map[string]any{
					"type": "number",
				}),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			org, err := RequiredParam[string](request, "org")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			inviteeID, err := OptionalParam[float64](request, "invitee_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			email, err := OptionalParam[string](request, "email")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Validate that at least one of invitee_id or email is provided
			if inviteeID == 0 && email == "" {
				return mcp.NewToolResultError("either invitee_id or email must be provided"), nil
			}

			role, err := OptionalParam[string](request, "role")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if role == "" {
				role = "direct_member"
			}

			var teamIDs []int64
			if rawTeamIDs, ok := request.GetArguments()["team_ids"]; ok {
				switch v := rawTeamIDs.(type) {
				case nil:
					// nothing to do
				case []any:
					for _, item := range v {
						id, parseErr := parseTeamID(item)
						if parseErr != nil {
							return mcp.NewToolResultError(parseErr.Error()), nil
						}
						teamIDs = append(teamIDs, id)
					}
				case []float64:
					for _, item := range v {
						teamIDs = append(teamIDs, int64(item))
					}
				default:
					return mcp.NewToolResultError("team_ids must be an array of numbers"), nil
				}
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Create the invitation request
			invitation := &github.CreateOrgInvitationOptions{
				Role:   github.Ptr(role),
				TeamID: teamIDs,
			}

			if inviteeID != 0 {
				invitation.InviteeID = github.Ptr(int64(inviteeID))
			}

			if email != "" {
				invitation.Email = github.Ptr(email)
			}

			createdInvitation, resp, err := client.Organizations.CreateOrgInvitation(ctx, org, invitation)
			if err != nil {
				return nil, fmt.Errorf("failed to create organization invitation: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to create organization invitation: %s", string(body))), nil
			}

			// Return a minimal response with relevant information
			type InvitationResponse struct {
				ID                 int64  `json:"id"`
				Login              string `json:"login,omitempty"`
				Email              string `json:"email,omitempty"`
				Role               string `json:"role"`
				InvitationTeamsURL string `json:"invitation_teams_url"`
				CreatedAt          string `json:"created_at"`
				InviterLogin       string `json:"inviter_login,omitempty"`
			}

			response := InvitationResponse{
				ID:                 createdInvitation.GetID(),
				Login:              createdInvitation.GetLogin(),
				Email:              createdInvitation.GetEmail(),
				Role:               createdInvitation.GetRole(),
				InvitationTeamsURL: createdInvitation.GetInvitationTeamURL(),
				CreatedAt:          createdInvitation.GetCreatedAt().Format("2006-01-02T15:04:05Z07:00"),
			}

			if createdInvitation.Inviter != nil {
				response.InviterLogin = createdInvitation.Inviter.GetLogin()
			}

			r, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func parseTeamID(value any) (int64, error) {
	switch v := value.(type) {
	case float64:
		// JSON numbers decode to float64; ensure they are whole numbers
		if v != float64(int64(v)) {
			return 0, fmt.Errorf("team_id must be an integer value")
		}
		return int64(v), nil
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case string:
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid team_id")
		}
		return id, nil
	default:
		return 0, fmt.Errorf("invalid team_id")
	}
}
