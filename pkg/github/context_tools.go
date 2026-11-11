package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// GetMeUIResourceURI is the URI for the get_me tool's MCP App UI resource.
const GetMeUIResourceURI = "ui://github-mcp-server/get-me"

type GetMeInput struct{}

type GetTeamsInput struct {
	User string `json:"user,omitempty" jsonschema:"Username to get teams for. If not provided, uses the authenticated user."`
}

type GetTeamMembersInput struct {
	Org      string `json:"org" jsonschema:"Organization login (owner) that contains the team."`
	TeamSlug string `json:"team_slug" jsonschema:"Team slug"`
}

func contextToolInputSchema[In any](descriptions map[string]string) *jsonschema.Schema {
	schema, err := jsonschema.For[In](nil)
	if err != nil {
		panic(fmt.Sprintf("failed to generate context tool input schema: %v", err))
	}
	if schema.Properties == nil {
		schema.Properties = make(map[string]*jsonschema.Schema)
	}
	schema.AdditionalProperties = nil
	for name, description := range descriptions {
		schema.Properties[name].Description = description
	}
	return schema
}

func contextToolValidationSchema(schema *jsonschema.Schema) *jsonschema.Schema {
	validationSchema := schema.CloneSchemas()
	validationSchema.AdditionalProperties = &jsonschema.Schema{}
	return validationSchema
}

func normalizeGetTeamsInput(arguments json.RawMessage) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(arguments, &fields); err != nil {
		return nil, err
	}
	if user, exists := fields["user"]; exists && bytes.Equal(bytes.TrimSpace(user), []byte("null")) {
		return nil, &inventory.ToolInputError{Message: "parameter user is not of type string, is <nil>"}
	}
	return normalizeContextRoutingKeys(arguments, fields, "user")
}

func normalizeGetTeamMembersInput(arguments json.RawMessage) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(arguments, &fields); err != nil {
		return nil, err
	}
	return normalizeContextRoutingKeys(arguments, fields, "org", "team_slug")
}

func normalizeContextRoutingKeys(arguments json.RawMessage, fields map[string]json.RawMessage, routingKeys ...string) (json.RawMessage, error) {
	changed := false
	for name := range fields {
		for _, key := range routingKeys {
			if name != key && strings.EqualFold(name, key) {
				delete(fields, name)
				changed = true
				break
			}
		}
	}
	if !changed {
		return arguments, nil
	}
	return json.Marshal(fields)
}

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
	CreatedAt         time.Time `json:"created_at" jsonschema:"Account creation time in RFC3339 format."`
	UpdatedAt         time.Time `json:"updated_at" jsonschema:"Last profile update time in RFC3339 format."`
	PrivateGists      int       `json:"private_gists,omitempty"`
	TotalPrivateRepos int64     `json:"total_private_repos,omitempty"`
	OwnedPrivateRepos int64     `json:"owned_private_repos,omitempty"`
}

// GetMe creates a tool to get details of the authenticated user.
func GetMe(t translations.TranslationHelperFunc) inventory.ServerTool {
	inputSchema := contextToolInputSchema[GetMeInput](nil)
	return NewToolWithSchemaOptions[GetMeInput, MinimalUser](
		ToolsetMetadataContext,
		mcp.Tool{
			Name:        "get_me",
			Description: t("TOOL_GET_ME_DESCRIPTION", "Get details of the authenticated GitHub user. Use this when a request is about the user's own profile for GitHub. Or when information is missing to build other tool calls."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_ME_USER_TITLE", "Get my user profile"),
				ReadOnlyHint: true,
			},
			InputSchema: inputSchema,
			Meta: mcp.Meta{
				"ui": map[string]any{
					"resourceUri": GetMeUIResourceURI,
					"visibility":  []string{"model", "app"},
				},
			},
		},
		scopes.NoScopes(),
		inventory.TypedSchemaOptions{
			ValidationInputSchema: contextToolValidationSchema(inputSchema),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, _ GetMeInput) (*mcp.CallToolResult, MinimalUser, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), MinimalUser{}, nil
			}

			user, res, err := client.Users.Get(ctx, "")
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get user",
					res,
					err,
				), MinimalUser{}, nil
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

			result := MarshalledTextResult(minimalUser)
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelGetMe())
			return result, minimalUser, nil
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
	inputSchema := contextToolInputSchema[GetTeamsInput](map[string]string{
		"user": t("TOOL_GET_TEAMS_USER_DESCRIPTION", "Username to get teams for. If not provided, uses the authenticated user."),
	})
	return NewToolWithSchemaOptions[GetTeamsInput, []OrganizationTeams](
		ToolsetMetadataContext,
		mcp.Tool{
			Name:        "get_teams",
			Description: t("TOOL_GET_TEAMS_DESCRIPTION", "Get details of the teams the user is a member of. Limited to organizations accessible with current credentials"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_TEAMS_TITLE", "Get teams"),
				ReadOnlyHint: true,
			},
			InputSchema: inputSchema,
		},
		scopes.RequireAll(scopes.ReadOrg),
		inventory.TypedSchemaOptions{
			ValidationInputSchema: contextToolValidationSchema(inputSchema),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GetTeamsInput) (*mcp.CallToolResult, []OrganizationTeams, error) {
			var username string
			if args.User != "" {
				username = args.User
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

			result := MarshalledTextResult(organizations)
			// Team membership is maintained by GitHub and cannot be forged by
			// outside contributors (trusted). Org team rosters are visible only
			// to org members, so confidentiality is private.
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelTeam())
			if organizations == nil {
				organizations = []OrganizationTeams{}
			}
			return result, organizations, nil
		},
		normalizeGetTeamsInput,
	)
}

func GetTeamMembers(t translations.TranslationHelperFunc) inventory.ServerTool {
	inputSchema := contextToolInputSchema[GetTeamMembersInput](map[string]string{
		"org":       t("TOOL_GET_TEAM_MEMBERS_ORG_DESCRIPTION", "Organization login (owner) that contains the team."),
		"team_slug": t("TOOL_GET_TEAM_MEMBERS_TEAM_SLUG_DESCRIPTION", "Team slug"),
	})

	return NewToolWithSchemaOptions[GetTeamMembersInput, []string](
		ToolsetMetadataContext,
		mcp.Tool{
			Name:        "get_team_members",
			Description: t("TOOL_GET_TEAM_MEMBERS_DESCRIPTION", "Get member usernames of a specific team in an organization. Limited to organizations accessible with current credentials"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_TEAM_MEMBERS_TITLE", "Get team members"),
				ReadOnlyHint: true,
			},
			InputSchema: inputSchema,
		},
		scopes.RequireAll(scopes.ReadOrg),
		inventory.TypedSchemaOptions{
			ValidationInputSchema: contextToolValidationSchema(inputSchema),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GetTeamMembersInput) (*mcp.CallToolResult, []string, error) {
			if args.Org == "" {
				return utils.NewToolResultError("missing required parameter: org"), nil, nil
			}
			if args.TeamSlug == "" {
				return utils.NewToolResultError("missing required parameter: team_slug"), nil, nil
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
				"org":      githubv4.String(args.Org),
				"teamSlug": githubv4.String(args.TeamSlug),
			}
			if err := gqlClient.Query(ctx, &q, vars); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to get team members", err), nil, nil
			}

			var members []string
			for _, member := range q.Organization.Team.Members.Nodes {
				members = append(members, string(member.Login))
			}

			result := MarshalledTextResult(members)
			// Team membership is maintained by GitHub and cannot be forged by
			// outside contributors (trusted). A team's member roster is visible
			// only to org members, so confidentiality is private.
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelTeam())
			if members == nil {
				members = []string{}
			}
			return result, members, nil
		},
		normalizeGetTeamMembersInput,
	)
}
