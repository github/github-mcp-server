package github

import (
	"context"
	"encoding/json"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// GetMeUIResourceURI is the URI for the get_me tool's MCP App UI resource.
const GetMeUIResourceURI = "ui://github-mcp-server/get-me"

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
			Meta: mcp.Meta{
				"ui": map[string]any{
					"resourceUri": GetMeUIResourceURI,
				},
			},
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
			vars := map[string]any{
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

// UpdateUserProfile creates a tool to update the authenticated user's profile.
func UpdateUserProfile(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataContext,
		mcp.Tool{
			Name:        "update_user_profile",
			Description: t("TOOL_UPDATE_USER_PROFILE_DESCRIPTION", "Update the authenticated GitHub user's profile information. At least one field to update must be provided."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UPDATE_USER_PROFILE_USER_TITLE", "Update my user profile"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {
						Type:        "string",
						Description: t("TOOL_UPDATE_USER_PROFILE_NAME_DESCRIPTION", "The new name of the user"),
					},
					"email": {
						Type:        "string",
						Description: t("TOOL_UPDATE_USER_PROFILE_EMAIL_DESCRIPTION", "The publicly visible email address of the user"),
					},
					"blog": {
						Type:        "string",
						Description: t("TOOL_UPDATE_USER_PROFILE_BLOG_DESCRIPTION", "The new blog URL of the user"),
					},
					"twitter_username": {
						Type:        "string",
						Description: t("TOOL_UPDATE_USER_PROFILE_TWITTER_USERNAME_DESCRIPTION", "The new Twitter username of the user"),
					},
					"company": {
						Type:        "string",
						Description: t("TOOL_UPDATE_USER_PROFILE_COMPANY_DESCRIPTION", "The new company of the user"),
					},
					"location": {
						Type:        "string",
						Description: t("TOOL_UPDATE_USER_PROFILE_LOCATION_DESCRIPTION", "The new location of the user"),
					},
					"hireable": {
						Type:        "boolean",
						Description: t("TOOL_UPDATE_USER_PROFILE_HIREABLE_DESCRIPTION", "The new hireable value of the user"),
					},
					"bio": {
						Type:        "string",
						Description: t("TOOL_UPDATE_USER_PROFILE_BIO_DESCRIPTION", "The new short biography of the user"),
					},
				},
			},
		},
		[]scopes.Scope{scopes.User},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			name, err := OptionalParam[string](args, "name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			email, err := OptionalParam[string](args, "email")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			blog, err := OptionalParam[string](args, "blog")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			twitterUsername, err := OptionalParam[string](args, "twitter_username")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			company, err := OptionalParam[string](args, "company")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			location, err := OptionalParam[string](args, "location")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			hireable, err := OptionalParam[bool](args, "hireable")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			bio, err := OptionalParam[string](args, "bio")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Require at least one field to be set
			_, hasHireable := args["hireable"]
			if name == "" && email == "" && blog == "" && twitterUsername == "" &&
				company == "" && location == "" && !hasHireable && bio == "" {
				return utils.NewToolResultError("at least one field to update must be provided"), nil, nil
			}

			userReq := &gogithub.User{}
			if name != "" {
				userReq.Name = gogithub.Ptr(name)
			}
			if email != "" {
				userReq.Email = gogithub.Ptr(email)
			}
			if blog != "" {
				userReq.Blog = gogithub.Ptr(blog)
			}
			if twitterUsername != "" {
				userReq.TwitterUsername = gogithub.Ptr(twitterUsername)
			}
			if company != "" {
				userReq.Company = gogithub.Ptr(company)
			}
			if location != "" {
				userReq.Location = gogithub.Ptr(location)
			}
			if hasHireable {
				userReq.Hireable = gogithub.Ptr(hireable)
			}
			if bio != "" {
				userReq.Bio = gogithub.Ptr(bio)
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			user, res, err := client.Users.Edit(ctx, userReq)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to update user profile",
					res,
					err,
				), nil, nil
			}

			minimalUser := MinimalUser{
				Login:      user.GetLogin(),
				ID:         user.GetID(),
				ProfileURL: user.GetHTMLURL(),
				AvatarURL:  user.GetAvatarURL(),
				Details: &UserDetails{
					Name:            user.GetName(),
					Company:         user.GetCompany(),
					Blog:            user.GetBlog(),
					Location:        user.GetLocation(),
					Email:           user.GetEmail(),
					Hireable:        user.GetHireable(),
					Bio:             user.GetBio(),
					TwitterUsername: user.GetTwitterUsername(),
				},
			}

			return MarshalledTextResult(minimalUser), nil, nil
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
			vars := map[string]any{
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
