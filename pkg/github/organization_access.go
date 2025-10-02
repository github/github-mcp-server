package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// OrganizationAccessValidator provides methods to validate organization access and permissions
type OrganizationAccessValidator struct {
	client *github.Client
}

// NewOrganizationAccessValidator creates a new validator instance
func NewOrganizationAccessValidator(client *github.Client) *OrganizationAccessValidator {
	return &OrganizationAccessValidator{
		client: client,
	}
}

// AccessValidationResult contains the result of organization access validation
type AccessValidationResult struct {
	HasAccess     bool
	IsPublicMember bool
	IsPrivateMember bool
	ErrorMessage  string
	Suggestions   []string
}

// ValidateOrganizationAccess checks if the current user has access to the specified organization
func (v *OrganizationAccessValidator) ValidateOrganizationAccess(ctx context.Context, orgName string) (*AccessValidationResult, error) {
	result := &AccessValidationResult{
		HasAccess: false,
		Suggestions: []string{},
	}

	// First, check if the user is a public member of the organization
	isMember, resp, err := v.client.Organizations.IsMember(ctx, orgName, "")
	if err == nil && resp.StatusCode == http.StatusOK && isMember {
		result.HasAccess = true
		result.IsPublicMember = true
		return result, nil
	}

	// Check if the user is a private member by trying to get organization membership
	membership, resp, err := v.client.Organizations.GetOrgMembership(ctx, "", orgName)
	if err == nil && resp.StatusCode == http.StatusOK && membership != nil {
		if membership.GetState() == "active" {
			result.HasAccess = true
			result.IsPrivateMember = true
			return result, nil
		}
	}

	// If we reach here, the user doesn't have access
	result.HasAccess = false
	
	// Check if it's a permissions issue
	if resp != nil && resp.StatusCode == http.StatusForbidden {
		result.ErrorMessage = "Access denied to organization. This may be due to insufficient token permissions or organization restrictions."
		result.Suggestions = append(result.Suggestions, 
			"Ensure your token has 'repo' and 'read:org' scopes",
			"Verify that the organization allows classic personal access tokens",
			"Check if the organization requires SSO authentication",
			"Confirm you are a member of the organization",
		)
	} else if resp != nil && resp.StatusCode == http.StatusNotFound {
		result.ErrorMessage = "Organization not found or you don't have access to it."
		result.Suggestions = append(result.Suggestions,
			"Verify the organization name is correct",
			"Ensure you are a member of the organization",
			"Check if your token has appropriate permissions",
		)
	} else {
		result.ErrorMessage = "Unable to verify organization access."
		result.Suggestions = append(result.Suggestions,
			"Check your network connection",
			"Verify your token is valid and not expired",
			"Ensure the organization name is correct",
		)
	}

	return result, nil
}

// ValidateRepositoryAccess checks if the current user has access to a specific repository
func (v *OrganizationAccessValidator) ValidateRepositoryAccess(ctx context.Context, owner, repo string) (*AccessValidationResult, error) {
	result := &AccessValidationResult{
		HasAccess: false,
		Suggestions: []string{},
	}

	// Try to get repository information
	repository, resp, err := v.client.Repositories.Get(ctx, owner, repo)
	
	if err == nil && resp.StatusCode == http.StatusOK && repository != nil {
		result.HasAccess = true
		return result, nil
	}

	// Handle different error scenarios
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusNotFound:
			result.ErrorMessage = fmt.Sprintf("Repository %s/%s not found or you don't have access to it.", owner, repo)
			result.Suggestions = append(result.Suggestions,
				"Verify the repository name and owner are correct",
				"Ensure you have access to the repository",
				"If this is a private repository, check your token permissions",
				"For organization repositories, verify you are a member of the organization",
			)
		case http.StatusForbidden:
			result.ErrorMessage = fmt.Sprintf("Access denied to repository %s/%s.", owner, repo)
			result.Suggestions = append(result.Suggestions,
				"Ensure your token has 'repo' scope for private repositories",
				"Check if the organization allows access via personal access tokens",
				"Verify you have the required permissions for this repository",
				"If SSO is enabled, ensure your token is authorized",
			)
		default:
			result.ErrorMessage = fmt.Sprintf("Unable to access repository %s/%s (HTTP %d).", owner, repo, resp.StatusCode)
			result.Suggestions = append(result.Suggestions,
				"Check your token permissions",
				"Verify the repository exists and you have access",
				"Check your network connection",
			)
		}
	} else if err != nil {
		result.ErrorMessage = fmt.Sprintf("Error accessing repository %s/%s: %v", owner, repo, err)
		result.Suggestions = append(result.Suggestions,
			"Check your network connection",
			"Verify your token is valid",
			"Ensure the repository name is correct",
		)
	}

	return result, nil
}

// CheckTokenPermissions validates that the token has the required permissions
func (v *OrganizationAccessValidator) CheckTokenPermissions(ctx context.Context) (*AccessValidationResult, error) {
	result := &AccessValidationResult{
		HasAccess: false,
		Suggestions: []string{},
	}

	// Try to get the authenticated user to verify token validity
	user, resp, err := v.client.Users.Get(ctx, "")
	if err != nil || resp.StatusCode != http.StatusOK {
		result.ErrorMessage = "Invalid or expired token."
		result.Suggestions = append(result.Suggestions,
			"Verify your GITHUB_PERSONAL_ACCESS_TOKEN is correct",
			"Check if your token has expired",
			"Ensure you're using a valid GitHub personal access token",
		)
		return result, nil
	}

	result.HasAccess = true

	// Check token scopes (available in response headers)
	if resp.Header != nil {
		scopes := resp.Header.Get("X-OAuth-Scopes")
		if scopes != "" {
			scopeList := strings.Split(scopes, ", ")
			hasRepo := false
			hasReadOrg := false
			
			for _, scope := range scopeList {
				scope = strings.TrimSpace(scope)
				if scope == "repo" {
					hasRepo = true
				}
				if scope == "read:org" {
					hasReadOrg = true
				}
			}

			if !hasRepo {
				result.Suggestions = append(result.Suggestions,
					"Consider adding 'repo' scope to access private repositories",
				)
			}
			if !hasReadOrg {
				result.Suggestions = append(result.Suggestions,
					"Consider adding 'read:org' scope to access organization information",
				)
			}
		}
	}

	if user != nil && user.Login != nil {
		result.ErrorMessage = fmt.Sprintf("Token is valid for user: %s", *user.Login)
	}

	return result, nil
}

// GetEnhancedErrorMessage creates a user-friendly error message with suggestions
func GetEnhancedErrorMessage(originalError error, owner, repo string, validator *OrganizationAccessValidator, ctx context.Context) string {
	if originalError == nil {
		return ""
	}

	errorMsg := originalError.Error()
	
	// Check if this looks like an organization repository
	if owner != "" && !isLikelyUserAccount(owner) {
		// Validate organization access
		orgResult, err := validator.ValidateOrganizationAccess(ctx, owner)
		if err == nil && !orgResult.HasAccess {
			return buildEnhancedMessage(orgResult.ErrorMessage, orgResult.Suggestions)
		}
	}

	// Check repository access if repo is specified
	if repo != "" {
		repoResult, err := validator.ValidateRepositoryAccess(ctx, owner, repo)
		if err == nil && !repoResult.HasAccess {
			return buildEnhancedMessage(repoResult.ErrorMessage, repoResult.Suggestions)
		}
	}

	return errorMsg
}

// isLikelyUserAccount checks if the owner looks like a user account vs organization
func isLikelyUserAccount(owner string) bool {
	// Simple heuristic: user accounts are typically lowercase and may contain hyphens
	// Organizations often have more structured names
	// This is just a heuristic and not 100% accurate
	return strings.ToLower(owner) == owner && !strings.Contains(owner, "_")
}

// buildEnhancedMessage creates a formatted error message with suggestions
func buildEnhancedMessage(errorMsg string, suggestions []string) string {
	if len(suggestions) == 0 {
		return errorMsg
	}

	message := errorMsg + "\n\nSuggestions to resolve this issue:\n"
	for i, suggestion := range suggestions {
		message += fmt.Sprintf("  %d. %s\n", i+1, suggestion)
	}

	message += "\nFor more information about token permissions, see: https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens"

	return message
}

// CheckOrganizationAccess creates a tool to check organization access and membership
func CheckOrganizationAccess(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("check_organization_access",
			mcp.WithDescription(t("TOOL_CHECK_ORGANIZATION_ACCESS_DESCRIPTION", "Check if you have access to a GitHub organization and validate your token permissions for private organization repositories")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CHECK_ORGANIZATION_ACCESS_USER_TITLE", "Check organization access"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("organization",
				mcp.Required(),
				mcp.Description("Organization name to check access for"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgName, err := RequiredParam[string](request, "organization")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			validator := NewOrganizationAccessValidator(client)

			// Check token permissions first
			tokenResult, err := validator.CheckTokenPermissions(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Error checking token permissions: %v", err)), nil
			}

			// Check organization access
			orgResult, err := validator.ValidateOrganizationAccess(ctx, orgName)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Error checking organization access: %v", err)), nil
			}

			// Prepare response
			response := map[string]interface{}{
				"organization":     orgName,
				"has_access":      orgResult.HasAccess,
				"is_public_member": orgResult.IsPublicMember,
				"is_private_member": orgResult.IsPrivateMember,
				"token_valid":     tokenResult.HasAccess,
			}

			if !orgResult.HasAccess {
				response["error"] = orgResult.ErrorMessage
				response["suggestions"] = orgResult.Suggestions
			}

			if len(tokenResult.Suggestions) > 0 {
				response["token_suggestions"] = tokenResult.Suggestions
			}

			if tokenResult.ErrorMessage != "" {
				response["token_info"] = tokenResult.ErrorMessage
			}

			r, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// CheckTokenPermissions creates a tool to validate GitHub token permissions
func CheckTokenPermissions(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("check_token_permissions",
			mcp.WithDescription(t("TOOL_CHECK_TOKEN_PERMISSIONS_DESCRIPTION", "Validate your GitHub personal access token and check its permissions for organization access")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CHECK_TOKEN_PERMISSIONS_USER_TITLE", "Check token permissions"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			validator := NewOrganizationAccessValidator(client)
			result, err := validator.CheckTokenPermissions(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Error checking token permissions: %v", err)), nil
			}

			// Prepare response
			response := map[string]interface{}{
				"token_valid": result.HasAccess,
			}

			if result.ErrorMessage != "" {
				response["message"] = result.ErrorMessage
			}

			if len(result.Suggestions) > 0 {
				response["suggestions"] = result.Suggestions
				response["recommendations"] = []string{
					"For organization access, ensure your token has 'repo' and 'read:org' scopes",
					"Check if the organization allows classic personal access tokens",
					"Verify that SSO is properly configured if required",
				}
			}

			r, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}