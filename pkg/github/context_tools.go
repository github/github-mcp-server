package github

import (
	"context"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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
func GetMe(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("get_me",
		mcp.WithDescription(t("TOOL_GET_ME_DESCRIPTION", "Get details of the authenticated GitHub user. Use this when a request includes \"me\", \"my\". The output will not change unless the user changes their profile, so only call this once.")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        t("TOOL_GET_ME_USER_TITLE", "Get my user profile"),
			ReadOnlyHint: ToBoolPtr(true),
		}),
		mcp.WithString("reason",
			mcp.Description("Optional: the reason for requesting the user information"),
		),
	)

	type args struct{}
	handler := mcp.NewTypedToolHandler(func(ctx context.Context, _ mcp.CallToolRequest, _ args) (*mcp.CallToolResult, error) {
		client, err := getClient(ctx)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get GitHub client", err), nil
		}

		user, res, err := client.Users.Get(ctx, "")
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx,
				"failed to get user",
				res,
				err,
			), nil
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

		return MarshalledTextResult(minimalUser), nil
	})

	return tool, handler
}
