package github

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// UserDetails contains additional fields about a GitHub user not already
// present in MinimalUser. Used by get_me context tool but omitted from search_users.
type UserDetails struct {
	Name              string    `json:"name,omitempty"`
	Company           string    `json:"company,omitempty"`
	Blog              string    `json:"blog,omitempty"`
	Location          string    `json:"location,omitempty"`
	Email             string    `json:"email,omitempty"`
	Hireable          bool      `json:"hireable,omitempty"`
	Bio               string    `json:"bio,omitempty"`
	TwitterUsername   string    `json:"twitter_username,omitempty"`
	PublicRepos       int       `json:"public_repos"`
	PublicGists       int       `json:"public_gists"`
	Followers         int       `json:"followers"`
	Following         int       `json:"following"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	PrivateGists      int       `json:"private_gists,omitempty"`
	TotalPrivateRepos int64     `json:"total_private_repos,omitempty"`
	OwnedPrivateRepos int64     `json:"owned_private_repos,omitempty"`
}

// GetMe creates a tool to get details of the authenticated user.
func GetMe(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataContext,
		mcp.Tool{
			Name:        "get_me",
			Description: t("TOOL_GET_ME_DESCRIPTION", "Get details of the authenticated GitHub user. Use this when a request is about the user's own profile for GitHub. Or when information is missing to build other tool calls."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_ME_USER_TITLE", "Get my user profile"),
				ReadOnlyHint: true,
			},
			// Use json.RawMessage to ensure "properties" is included even when empty.
			// OpenAI strict mode requires the properties field to be present.
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		nil,
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, _ map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			user, res, err := client.Users.Get(ctx, "")
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get user",
					res,
					err,
				), nil, nil
			}

			// Create minimal user representation instead of returning full user object
			minimalUser := MinimalUser{
				Login:      user.GetLogin(),
				ID:         user.GetID(),
				ProfileURL: user.GetHTMLURL(),
				AvatarURL:  user.GetAvatarURL(),
				Details: &UserDetails{
					Name:              user.GetName(),
					Company:           user.GetCompany(),
					Blog:              user.GetBlog(),
					Location:          user.GetLocation(),
					Email:             user.GetEmail(),
					Hireable:          user.GetHireable(),
					Bio:               user.GetBio(),
					TwitterUsername:   user.GetTwitterUsername(),
					PublicRepos:       user.GetPublicRepos(),
					PublicGists:       user.GetPublicGists(),
					Followers:         user.GetFollowers(),
					Following:         user.GetFollowing(),
					CreatedAt:         user.GetCreatedAt().Time,
					UpdatedAt:         user.GetUpdatedAt().Time,
					PrivateGists:      user.GetPrivateGists(),
					TotalPrivateRepos: user.GetTotalPrivateRepos(),
					OwnedPrivateRepos: user.GetOwnedPrivateRepos(),
				},
			}

			return MarshalledTextResult(minimalUser), nil, nil
		},
	)
}

type TeamInfo struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type OrganizationTeams struct {
	Org   string     `json:"org"`
	Teams []TeamInfo `json:"teams"`
}

type OutUser struct {
	Login     string `json:"login"`
	ID        string `json:"id"`
	AvatarURL string `json:"avatar_url"`
	Type      string `json:"type"`
	SiteAdmin bool   `json:"site_admin"`
}

func GetTeams(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataContext,
		mcp.Tool{
			Name:        "get_teams",
			Description: t("TOOL_GET_TEAMS_DESCRIPTION", "Get details of the teams the user is a member of. Limited to organizations accessible with current credentials"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_TEAMS_TITLE", "Get teams"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"user": {
						Type:        "string",
						Description: t("TOOL_GET_TEAMS_USER_DESCRIPTION", "Username to get teams for. If not provided, uses the authenticated user."),
					},
				},
			},
		},
		[]scopes.Scope{scopes.ReadOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			user, err := OptionalParam[string](args, "user")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var username string
			if user != "" {
				username = user
			} else {
				client, err := deps.GetClient(ctx)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
				}

				userResp, res, err := client.Users.Get(ctx, "")
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to get user",
						res,
						err,
					), nil, nil
				}
				username = userResp.GetLogin()
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GQL client", err), nil, nil
			}

			var q struct {
				User struct {
					Organizations struct {
						Nodes []struct {
							Login githubv4.String
							Teams struct {
								Nodes []struct {
									Name        githubv4.String
									Slug        githubv4.String
									Description githubv4.String
								}
							} `graphql:"teams(first: 100, userLogins: [$login])"`
						}
					} `graphql:"organizations(first: 100)"`
				} `graphql:"user(login: $login)"`
			}
			vars := map[string]interface{}{
				"login": githubv4.String(username),
			}
			if err := gqlClient.Query(ctx, &q, vars); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to find teams", err), nil, nil
			}

			var organizations []OrganizationTeams
			for _, org := range q.User.Organizations.Nodes {
				orgTeams := OrganizationTeams{
					Org:   string(org.Login),
					Teams: make([]TeamInfo, 0, len(org.Teams.Nodes)),
				}

				for _, team := range org.Teams.Nodes {
					orgTeams.Teams = append(orgTeams.Teams, TeamInfo{
						Name:        string(team.Name),
						Slug:        string(team.Slug),
						Description: string(team.Description),
					})
				}

				organizations = append(organizations, orgTeams)
			}

			return MarshalledTextResult(organizations), nil, nil
		},
	)
}

func GetTeamMembers(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataContext,
		mcp.Tool{
			Name:        "get_team_members",
			Description: t("TOOL_GET_TEAM_MEMBERS_DESCRIPTION", "Get member usernames of a specific team in an organization. Limited to organizations accessible with current credentials"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_TEAM_MEMBERS_TITLE", "Get team members"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"org": {
						Type:        "string",
						Description: t("TOOL_GET_TEAM_MEMBERS_ORG_DESCRIPTION", "Organization login (owner) that contains the team."),
					},
					"team_slug": {
						Type:        "string",
						Description: t("TOOL_GET_TEAM_MEMBERS_TEAM_SLUG_DESCRIPTION", "Team slug"),
					},
				},
				Required: []string{"org", "team_slug"},
			},
		},
		[]scopes.Scope{scopes.ReadOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			org, err := RequiredParam[string](args, "org")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			teamSlug, err := RequiredParam[string](args, "team_slug")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GQL client", err), nil, nil
			}

			var q struct {
				Organization struct {
					Team struct {
						Members struct {
							Nodes []struct {
								Login githubv4.String
							}
						} `graphql:"members(first: 100)"`
					} `graphql:"team(slug: $teamSlug)"`
				} `graphql:"organization(login: $org)"`
			}
			vars := map[string]interface{}{
				"org":      githubv4.String(org),
				"teamSlug": githubv4.String(teamSlug),
			}
			if err := gqlClient.Query(ctx, &q, vars); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to get team members", err), nil, nil
			}

			var members []string
			for _, member := range q.Organization.Team.Members.Nodes {
				members = append(members, string(member.Login))
			}

			return MarshalledTextResult(members), nil, nil
		},
	)
}

func GetOrgMembers(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataContext,
		mcp.Tool{
			Name:        "get_org_members",
			Description: t("TOOL_GET_ORG_MEMBERS_DESCRIPTION", "Get member users of a specific organization. Returns a list of user objects with fields: login, id, avatar_url, type. Limited to organizations accessible with current credentials"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_ORG_MEMBERS_TITLE", "Get organization members"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"org": {
						Type:        "string",
						Description: t("TOOL_GET_ORG_MEMBERS_ORG_DESCRIPTION", "Organization login (owner) to get members for."),
					},
					"role": {
						Type:        "string",
						Description: "Filter by role: all, admin, member",
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

			role, err := OptionalParam[string](args, "role")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GQL client", err), nil, nil
			}

			roleFilter := strings.ToLower(strings.TrimSpace(role))
			if roleFilter == "all" {
				roleFilter = ""
			}

			var q struct {
				Organization struct {
					MembersWithRole struct {
						Edges []struct {
							Role githubv4.String
							Node struct {
								Login githubv4.String
							}
						}
					} `graphql:"membersWithRole(first: 100)"`
				} `graphql:"organization(login: $org)"`
			}
			vars := map[string]any{
				"org": githubv4.String(org),
			}

			if err := gqlClient.Query(ctx, &q, vars); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to get organization members", err), nil, nil
			}

			members := make([]struct {
				Login string `json:"login"`
				Role  string `json:"role"`
			}, 0, len(q.Organization.MembersWithRole.Edges))
			for _, member := range q.Organization.MembersWithRole.Edges {
				if roleFilter != "" && strings.ToLower(string(member.Role)) != roleFilter {
					continue
				}
				members = append(members, struct {
					Login string `json:"login"`
					Role  string `json:"role"`
				}{Login: string(member.Node.Login), Role: string(member.Role)})
			}

			return MarshalledTextResult(members), nil, nil
		},
	)
}

func ListOutsideCollaborators(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataContext,
		mcp.Tool{
			Name:        "list_outside_collaborators",
			Description: t("TOOL_LIST_OUTSIDE_COLLABORATORS_DESCRIPTION", "List all outside collaborators of an organization (users with access to organization repositories but not members)."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_OUTSIDE_COLLABORATORS_TITLE", "List outside collaborators"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"org": {
						Type:        "string",
						Description: t("TOOL_LIST_OUTSIDE_COLLABORATORS_ORG_DESCRIPTION", "The organization name"),
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

			users, resp, err := client.Organizations.ListOutsideCollaborators(ctx, org, nil)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "Failed to list outside collaborators", resp, err), nil, nil
			}

			collaborators := make([]string, 0, len(users))
			for _, user := range users {
				collaborators = append(collaborators, user.GetLogin())
			}

			return MarshalledTextResult(collaborators), nil, nil
		},
	)
}
