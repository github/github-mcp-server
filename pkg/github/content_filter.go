package github

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/shurcooL/githubv4"
)

// contextKey is a private type used for context keys
type contextKey int

const (
	// ContentFilterKey is the key used to access content filter settings from context
	contentFilterKey contextKey = iota
)

// ContentFilterSettings holds the configuration for content filtering
type ContentFilterSettings struct {
	// Enabled indicates if content filtering is enabled
	Enabled bool
	// TrustedRepo is the repository in format "owner/repo" that is used to check permissions
	TrustedRepo string
	// OwnerRepo is the parsed owner and repo from TrustedRepo
	OwnerRepo OwnerRepo
	// IsPrivate indicates if the trusted repo is private
	IsPrivate bool
	// TrustedUsers is a map of users who have been verified to have push access
	TrustedUsers map[string]bool
	// AuthenticatedUser is the login name of the authenticated user
	AuthenticatedUser string
	// mu protects the TrustedUsers map
	mu sync.RWMutex
}

// OwnerRepo holds the parsed owner and repo from a string in the format "owner/repo"
type OwnerRepo struct {
	Owner string
	Repo  string
}

// ParseOwnerRepo parses a string in the format "owner/repo" into an OwnerRepo struct
func ParseOwnerRepo(s string) (OwnerRepo, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return OwnerRepo{}, fmt.Errorf("invalid format for owner/repo: %s", s)
	}
	return OwnerRepo{Owner: parts[0], Repo: parts[1]}, nil
}

// GetContentFilterFromContext retrieves the content filter settings from the context
func GetContentFilterFromContext(ctx context.Context) (*ContentFilterSettings, bool) {
	if ctx == nil {
		return nil, false
	}
	settings, ok := ctx.Value(contentFilterKey).(*ContentFilterSettings)
	return settings, ok
}

// InitContentFilter initializes the content filter in the context
func InitContentFilter(ctx context.Context, trustedRepo string, getGQLClient GetGQLClientFn) (context.Context, error) {
	if trustedRepo == "" {
		// Content filtering is not enabled
		return ctx, nil
	}

	ownerRepo, err := ParseOwnerRepo(trustedRepo)
	if err != nil {
		return ctx, err
	}

	settings := &ContentFilterSettings{
		Enabled:      true,
		TrustedRepo:  trustedRepo,
		OwnerRepo:    ownerRepo,
		TrustedUsers: map[string]bool{},
	}

	// Check if the repository is private, if so, disable content filtering
	isPrivate, err := IsRepoPrivate(ctx, settings.OwnerRepo, getGQLClient)
	if err != nil {
		return ctx, fmt.Errorf("failed to check repository visibility: %w", err)
	}
	settings.IsPrivate = isPrivate

	// Get the authenticated user's login name
	authUserLogin, err := GetAuthenticatedUser(ctx, getGQLClient)
	if err != nil {
		// Non-fatal error - we can continue without knowing the authenticated user
		// Just log the error and continue
		fmt.Printf("warning: failed to get authenticated user: %v\n", err)
	} else {
		settings.AuthenticatedUser = authUserLogin
		// The authenticated user is always trusted
		settings.TrustedUsers[authUserLogin] = true
	}

	return context.WithValue(ctx, contentFilterKey, settings), nil
}

// IsRepoPrivate checks if a repository is private using GraphQL
func IsRepoPrivate(ctx context.Context, ownerRepo OwnerRepo, getGQLClient GetGQLClientFn) (bool, error) {
	client, err := getGQLClient(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get GraphQL client: %w", err)
	}

	var query struct {
		Repository struct {
			IsPrivate githubv4.Boolean
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner": githubv4.String(ownerRepo.Owner),
		"name":  githubv4.String(ownerRepo.Repo),
	}

	err = client.Query(ctx, &query, variables)
	if err != nil {
		return false, fmt.Errorf("failed to query repository visibility: %w", err)
	}

	return bool(query.Repository.IsPrivate), nil
}

// HasPushAccess checks if a user has push access to the trusted repository
func HasPushAccess(ctx context.Context, username string, getGQLClient GetGQLClientFn) (bool, error) {
	settings, ok := GetContentFilterFromContext(ctx)
	if !ok || !settings.Enabled || settings.IsPrivate {
		// If filtering is not enabled or repo is private, all users are trusted
		return true, nil
	}

	// Check cache first
	settings.mu.RLock()
	trusted, found := settings.TrustedUsers[username]
	settings.mu.RUnlock()
	if found {
		return trusted, nil
	}

	// Query GitHub API for permission
	client, err := getGQLClient(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get GraphQL client: %w", err)
	}

	var query struct {
		Repository struct {
			Collaborators struct {
				Edges []struct {
					Permission githubv4.String
					Node       struct {
						Login githubv4.String
					}
				}
			} `graphql:"collaborators(query: $username, first: 1)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner":    githubv4.String(settings.OwnerRepo.Owner),
		"name":     githubv4.String(settings.OwnerRepo.Repo),
		"username": githubv4.String(username),
	}

	err = client.Query(ctx, &query, variables)
	if err != nil {
		return false, fmt.Errorf("failed to query user permissions: %w", err)
	}

	// Check if the user has push access
	hasPush := false
	for _, edge := range query.Repository.Collaborators.Edges {
		login := string(edge.Node.Login)
		if strings.EqualFold(login, username) {
			permission := string(edge.Permission)
			// WRITE, ADMIN, and MAINTAIN permissions have push access
			hasPush = permission == "WRITE" || permission == "ADMIN" || permission == "MAINTAIN"
			break
		}
	}

	// Cache the result
	settings.mu.Lock()
	settings.TrustedUsers[username] = hasPush
	settings.mu.Unlock()

	return hasPush, nil
}

// GetAuthenticatedUser gets the login name of the authenticated user using GraphQL
func GetAuthenticatedUser(ctx context.Context, getGQLClient GetGQLClientFn) (string, error) {
	client, err := getGQLClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get GraphQL client: %w", err)
	}

	var query struct {
		Viewer struct {
			Login githubv4.String
		}
	}

	err = client.Query(ctx, &query, nil)
	if err != nil {
		return "", fmt.Errorf("failed to query authenticated user: %w", err)
	}

	return string(query.Viewer.Login), nil
}

// ShouldIncludeContent checks if content from a user should be included
func ShouldIncludeContent(ctx context.Context, username string, getGQLClient GetGQLClientFn) bool {
	settings, ok := GetContentFilterFromContext(ctx)
	if !ok || !settings.Enabled || settings.IsPrivate {
		// If filtering is not enabled or repo is private, include all content
		return true
	}

	// Always include content from the authenticated user
	if settings.AuthenticatedUser != "" && strings.EqualFold(username, settings.AuthenticatedUser) {
		return true
	}

	// Check if user has push access
	hasPush, err := HasPushAccess(ctx, username, getGQLClient)
	if err != nil {
		// If there's an error checking permissions, default to not including the content for safety
		return false
	}
	return hasPush
}
