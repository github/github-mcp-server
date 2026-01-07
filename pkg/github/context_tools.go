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
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// GetMeUIResourceURI is the URI for the get_me tool's MCP App UI resource.
const GetMeUIResourceURI = "ui://github-mcp-server/get-me"

// GetMeUIHTML is the HTML content for the get_me tool's MCP App UI.
// This UI dynamically displays user information from the tool result.
//
// How MCP Apps work:
// 1. Server registers this HTML as a resource at ui://github-mcp-server/get-me
// 2. Server links the get_me tool to this resource via _meta.ui.resourceUri
// 3. When host calls get_me, it sees the resourceUri and fetches this HTML
// 4. Host renders HTML in a sandboxed iframe and communicates via postMessage
// 5. After ui/initialize, host sends ui/notifications/tool-result with the data
// 6. This UI parses the tool result and renders the user profile dynamically
const GetMeUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GitHub MCP Server - Get Me</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: var(--font-sans, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif);
            padding: 16px;
            margin: 0;
            background: var(--color-background-primary, #fff);
            color: var(--color-text-primary, #24292f);
            font-size: var(--font-text-md-size, 14px);
            line-height: var(--font-text-md-line-height, 1.5);
        }
        .loading {
            color: var(--color-text-secondary, #656d76);
            font-style: italic;
        }
        .error {
            color: var(--color-text-danger, #cf222e);
        }
        .user-card {
            border: 1px solid var(--color-border-primary, #d0d7de);
            border-radius: var(--border-radius-lg, 6px);
            padding: 16px;
            max-width: 400px;
            background: var(--color-background-secondary, #f6f8fa);
        }
        .user-header {
            display: flex;
            align-items: center;
            gap: 12px;
            margin-bottom: 12px;
            padding-bottom: 12px;
            border-bottom: 1px solid var(--color-border-primary, #d0d7de);
        }
        .avatar {
            width: 48px;
            height: 48px;
            border-radius: 50%;
            border: 1px solid var(--color-border-primary, #d0d7de);
        }
        .user-names {
            flex: 1;
        }
        .display-name {
            font-weight: 600;
            font-size: var(--font-heading-xs-size, 16px);
            color: var(--color-text-primary, #24292f);
            margin: 0;
        }
        .username {
            color: var(--color-text-secondary, #656d76);
            margin: 0;
        }
        .bio {
            font-style: italic;
            color: var(--color-text-secondary, #656d76);
            margin: 0 0 12px 0;
            padding: 8px;
            background: var(--color-background-primary, #fff);
            border-radius: var(--border-radius-sm, 4px);
        }
        .info-grid {
            display: grid;
            grid-template-columns: auto 1fr;
            gap: 4px 12px;
        }
        .info-label {
            color: var(--color-text-secondary, #656d76);
            font-weight: 500;
        }
        .info-value {
            color: var(--color-text-primary, #24292f);
        }
        .info-value a {
            color: var(--color-text-info, #0969da);
            text-decoration: none;
        }
        .info-value a:hover {
            text-decoration: underline;
        }
        .stats {
            display: flex;
            gap: 16px;
            margin-top: 12px;
            padding-top: 12px;
            border-top: 1px solid var(--color-border-primary, #d0d7de);
        }
        .stat {
            text-align: center;
        }
        .stat-value {
            font-weight: 600;
            font-size: var(--font-heading-xs-size, 16px);
            color: var(--color-text-primary, #24292f);
        }
        .stat-label {
            font-size: var(--font-text-sm-size, 12px);
            color: var(--color-text-secondary, #656d76);
        }
    </style>
</head>
<body>
    <div id="content">
        <p class="loading">Loading user data...</p>
    </div>
    <script>
        // ============================================================
        // MCP Apps Protocol Implementation
        // ============================================================
        // MCP Apps communicate with the host via postMessage using JSON-RPC.
        // The host (e.g., VS Code) renders this HTML in a sandboxed iframe.
        // After initialization, the host pushes the tool result to us.
        
        const pendingRequests = new Map();
        let requestId = 0;

        // Send a JSON-RPC request to the host and wait for response
        function sendRequest(method, params) {
            const id = ++requestId;
            return new Promise((resolve) => {
                pendingRequests.set(id, resolve);
                window.parent.postMessage({ jsonrpc: '2.0', id, method, params }, '*');
            });
        }

        // Handle all messages from the host
        function handleMessage(event) {
            const message = event.data;
            if (!message || typeof message !== 'object') return;

            // Handle responses to our requests (e.g., ui/initialize response)
            if (message.id !== undefined && pendingRequests.has(message.id)) {
                const resolve = pendingRequests.get(message.id);
                pendingRequests.delete(message.id);
                resolve(message.result);
                return;
            }

            // Handle notifications pushed by the host
            // The host sends ui/notifications/tool-result after the tool completes
            if (message.method === 'ui/notifications/tool-result') {
                handleToolResult(message.params);
            }
        }

        // Process the tool result and render the UI
        function handleToolResult(result) {
            if (!result || !result.content) {
                showError('No content in tool result');
                return;
            }
            
            // The tool result contains content blocks; find the text one
            const textContent = result.content.find(c => c.type === 'text');
            if (!textContent || !textContent.text) {
                showError('No text content in tool result');
                return;
            }

            try {
                // Parse the JSON user data from the tool's text output
                const userData = JSON.parse(textContent.text);
                renderUserCard(userData);
            } catch (e) {
                showError('Error parsing user data: ' + e.message);
            }
        }

        function showError(msg) {
            document.getElementById('content').innerHTML = 
                '<p class="error">' + escapeHtml(msg) + '</p>';
            notifySize();
        }

        function renderUserCard(user) {
            const d = user.details || {};
            const html = ` + "`" + `
                <div class="user-card">
                    <div class="user-header">
                        ${user.avatar_url ? ` + "`" + `<img class="avatar" src="${escapeHtml(user.avatar_url)}" alt="Avatar">` + "`" + ` : ''}
                        <div class="user-names">
                            <p class="display-name">${escapeHtml(d.name || user.login || 'Unknown')}</p>
                            <p class="username">@${escapeHtml(user.login || 'unknown')}</p>
                        </div>
                    </div>
                    <div class="info-grid">
                        ${d.company ? ` + "`" + `
                            <span class="info-label">🏢 Company</span>
                            <span class="info-value">${escapeHtml(d.company)}</span>
                        ` + "`" + ` : ''}
                        ${d.location ? ` + "`" + `
                            <span class="info-label">📍 Location</span>
                            <span class="info-value">${escapeHtml(d.location)}</span>
                        ` + "`" + ` : ''}
                        ${d.blog ? ` + "`" + `
                            <span class="info-label">🔗 Website</span>
                            <span class="info-value"><a href="${escapeHtml(d.blog)}" target="_blank">${escapeHtml(d.blog)}</a></span>
                        ` + "`" + ` : ''}
                        ${d.twitter_username ? ` + "`" + `
                            <span class="info-label">🐦 Twitter</span>
                            <span class="info-value"><a href="https://twitter.com/${escapeHtml(d.twitter_username)}" target="_blank">@${escapeHtml(d.twitter_username)}</a></span>
                        ` + "`" + ` : ''}
                        ${d.email ? ` + "`" + `
                            <span class="info-label">✉️ Email</span>
                            <span class="info-value"><a href="mailto:${escapeHtml(d.email)}">${escapeHtml(d.email)}</a></span>
                        ` + "`" + ` : ''}
                    </div>
                    <div class="stats">
                        <div class="stat">
                            <div class="stat-value">${d.public_repos ?? 0}</div>
                            <div class="stat-label">Repos</div>
                        </div>
                        <div class="stat">
                            <div class="stat-value">${d.followers ?? 0}</div>
                            <div class="stat-label">Followers</div>
                        </div>
                        <div class="stat">
                            <div class="stat-value">${d.following ?? 0}</div>
                            <div class="stat-label">Following</div>
                        </div>
                        <div class="stat">
                            <div class="stat-value">${d.public_gists ?? 0}</div>
                            <div class="stat-label">Gists</div>
                        </div>
                    </div>
                </div>
            ` + "`" + `;
            
            document.getElementById('content').innerHTML = html;
            notifySize();
        }

        // Tell the host our rendered size so it can adjust the iframe
        function notifySize() {
            window.parent.postMessage({
                jsonrpc: '2.0',
                method: 'ui/notifications/size-changed',
                params: { height: document.body.scrollHeight + 20 }
            }, '*');
        }

        function escapeHtml(text) {
            if (text == null) return '';
            const div = document.createElement('div');
            div.textContent = String(text);
            return div.innerHTML;
        }

        // Listen for messages from the host
        window.addEventListener('message', handleMessage);

        // Initialize the MCP App connection
        // After this completes, the host will send us:
        // 1. ui/notifications/tool-input (the arguments passed to the tool)
        // 2. ui/notifications/tool-result (the tool's output - what we render)
        sendRequest('ui/initialize', {
            appInfo: { name: 'github-mcp-server-get-me', version: '1.0.0' },
            appCapabilities: {},
            protocolVersion: '2025-11-21'
        });
    </script>
</body>
</html>`

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
			// MCP Apps UI metadata - links this tool to its UI resource
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
