package github

import (
	"context"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
)

// GetUser creates a tool to get a user by username.
func GetUser(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataUsers,
		mcp.Tool{
			Name:        "get_user",
			Description: t("TOOL_GET_USER_DESCRIPTION", "Get a user by username. Use this when you need information about a specific GitHub user."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_USER_TITLE", "Get a user by username"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"username": {
						Type:        "string",
						Description: t("TOOL_GET_USER_USERNAME_DESCRIPTION", "Username of the user"),
					},
				},
				Required: []string{"username"},
			},
		},
		nil,
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			return getUserHandler(ctx, deps, args)
		},
	)
}

func getUserHandler(ctx context.Context, deps ToolDependencies, args map[string]any) (*mcp.CallToolResult, any, error) {
	username, err := RequiredParam[string](args, "username")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	client, err := deps.GetClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
	}

	user, resp, err := client.Users.Get(ctx, username)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get user",
			resp,
			err,
		), nil, nil
	}

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
}
