package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// authenticatedUser represents the empty string used to indicate operations
// should be performed on the authenticated user's account rather than a specific username.
// This is used by GitHub's API when the username parameter is empty.
const authenticatedUser = ""

// NOTE: GitHub's REST API for packages does not currently expose download statistics.
// While download counts are visible on the GitHub web interface (e.g., github.com/orgs/{org}/packages),
// they are not included in the API responses.

// handleDeletionResponse handles the common response logic for package deletion operations.
// It checks the status code, reads error messages if any, and returns a standardized success response.
func handleDeletionResponse(resp *github.Response, successMessage string) (*mcp.CallToolResult, error) {
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return mcp.NewToolResultError(fmt.Sprintf("deletion failed: %s", string(body))), nil
	}

	result := map[string]interface{}{
		"success": true,
		"message": successMessage,
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

// ListOrgPackages creates a tool to list packages for an organization
func ListOrgPackages(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_org_packages",
			mcp.WithDescription(t("TOOL_LIST_ORG_PACKAGES_DESCRIPTION", "List packages for a GitHub organization. Returns package metadata including name, type, visibility, and version count.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_ORG_PACKAGES_USER_TITLE", "List organization packages"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("org",
				mcp.Required(),
				mcp.Description("Organization name"),
			),
			mcp.WithString("package_type",
				mcp.Description("Filter by package type"),
				mcp.Enum("npm", "maven", "rubygems", "docker", "nuget", "container"),
			),
			mcp.WithString("visibility",
				mcp.Description("Filter by package visibility"),
				mcp.Enum("public", "private", "internal"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			org, err := RequiredParam[string](request, "org")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageType, err := OptionalParam[string](request, "package_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			visibility, err := OptionalParam[string](request, "visibility")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.PackageListOptions{
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			// Only set optional parameters if they have values
			if packageType != "" {
				opts.PackageType = github.Ptr(packageType)
			}
			if visibility != "" {
				opts.Visibility = github.Ptr(visibility)
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			packages, resp, err := client.Organizations.ListPackages(ctx, org, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to list packages for organization '%s'", org),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(packages)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetOrgPackage creates a tool to get a specific package for an organization with download statistics
func GetOrgPackage(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_org_package",
			mcp.WithDescription(t("TOOL_GET_ORG_PACKAGE_DESCRIPTION", "Get details of a specific package for an organization.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_ORG_PACKAGE_USER_TITLE", "Get organization package details"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("org",
				mcp.Required(),
				mcp.Description("Organization name"),
			),
			mcp.WithString("package_type",
				mcp.Required(),
				mcp.Description("Package type"),
				mcp.Enum("npm", "maven", "rubygems", "docker", "nuget", "container"),
			),
			mcp.WithString("package_name",
				mcp.Required(),
				mcp.Description("Package name"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			org, err := RequiredParam[string](request, "org")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageType, err := RequiredParam[string](request, "package_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageName, err := RequiredParam[string](request, "package_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			pkg, resp, err := client.Organizations.GetPackage(ctx, org, packageType, packageName)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get package '%s' of type '%s' for organization '%s'", packageName, packageType, org),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(pkg)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListPackageVersions creates a tool to list versions of a package with download statistics
func ListPackageVersions(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_package_versions",
			mcp.WithDescription(t("TOOL_LIST_PACKAGE_VERSIONS_DESCRIPTION", "List versions of a package for an organization. Each version includes metadata.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_PACKAGE_VERSIONS_USER_TITLE", "List package versions"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("org",
				mcp.Required(),
				mcp.Description("Organization name"),
			),
			mcp.WithString("package_type",
				mcp.Required(),
				mcp.Description("Package type"),
				mcp.Enum("npm", "maven", "rubygems", "docker", "nuget", "container"),
			),
			mcp.WithString("package_name",
				mcp.Required(),
				mcp.Description("Package name"),
			),
			mcp.WithString("state",
				mcp.Description("Filter by version state"),
				mcp.Enum("active", "deleted"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			org, err := RequiredParam[string](request, "org")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageType, err := RequiredParam[string](request, "package_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageName, err := RequiredParam[string](request, "package_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			state, err := OptionalParam[string](request, "state")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.PackageListOptions{
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			// Only set state parameter if it has a value
			if state != "" {
				opts.State = github.Ptr(state)
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			versions, resp, err := client.Organizations.PackageGetAllVersions(ctx, org, packageType, packageName, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to list package versions for package '%s' of type '%s'", packageName, packageType),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(versions)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetPackageVersion creates a tool to get a specific package version with download statistics
func GetPackageVersion(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_package_version",
			mcp.WithDescription(t("TOOL_GET_PACKAGE_VERSION_DESCRIPTION", "Get details of a specific package version, including metadata.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PACKAGE_VERSION_USER_TITLE", "Get package version details"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("org",
				mcp.Required(),
				mcp.Description("Organization name"),
			),
			mcp.WithString("package_type",
				mcp.Required(),
				mcp.Description("Package type"),
				mcp.Enum("npm", "maven", "rubygems", "docker", "nuget", "container"),
			),
			mcp.WithString("package_name",
				mcp.Required(),
				mcp.Description("Package name"),
			),
			mcp.WithNumber("package_version_id",
				mcp.Required(),
				mcp.Description("Package version ID"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			org, err := RequiredParam[string](request, "org")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageType, err := RequiredParam[string](request, "package_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageName, err := RequiredParam[string](request, "package_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageVersionID, err := RequiredParam[float64](request, "package_version_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			version, resp, err := client.Organizations.PackageGetVersion(ctx, org, packageType, packageName, int64(packageVersionID))
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get package version %d for package '%s'", int64(packageVersionID), packageName),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(version)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListUserPackages creates a tool to list packages for a user
func ListUserPackages(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_user_packages",
			mcp.WithDescription(t("TOOL_LIST_USER_PACKAGES_DESCRIPTION", "List packages for a GitHub user. Note: Download statistics are not available via the GitHub REST API.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_USER_PACKAGES_USER_TITLE", "List user packages"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("username",
				mcp.Required(),
				mcp.Description("GitHub username"),
			),
			mcp.WithString("package_type",
				mcp.Description("Filter by package type"),
				mcp.Enum("npm", "maven", "rubygems", "docker", "nuget", "container"),
			),
			mcp.WithString("visibility",
				mcp.Description("Filter by package visibility"),
				mcp.Enum("public", "private", "internal"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			username, err := RequiredParam[string](request, "username")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageType, err := OptionalParam[string](request, "package_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			visibility, err := OptionalParam[string](request, "visibility")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.PackageListOptions{
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			// Only set optional parameters if they have values
			if packageType != "" {
				opts.PackageType = github.Ptr(packageType)
			}
			if visibility != "" {
				opts.Visibility = github.Ptr(visibility)
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			packages, resp, err := client.Users.ListPackages(ctx, username, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to list packages for user '%s'", username),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(packages)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// DeleteOrgPackage creates a tool to delete an entire package from an organization
// Requires delete:packages scope in addition to read:packages
func DeleteOrgPackage(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("delete_org_package",
			mcp.WithDescription(t("TOOL_DELETE_ORG_PACKAGE_DESCRIPTION", "Delete an entire package from a GitHub organization. This will delete all versions of the package. Requires delete:packages scope.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_DELETE_ORG_PACKAGE_USER_TITLE", "Delete organization package"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("org",
				mcp.Required(),
				mcp.Description("Organization name"),
			),
			mcp.WithString("package_type",
				mcp.Required(),
				mcp.Description("Package type"),
				mcp.Enum("npm", "maven", "rubygems", "docker", "nuget", "container"),
			),
			mcp.WithString("package_name",
				mcp.Required(),
				mcp.Description("Package name"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			org, err := RequiredParam[string](request, "org")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageType, err := RequiredParam[string](request, "package_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageName, err := RequiredParam[string](request, "package_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Organizations.DeletePackage(ctx, org, packageType, packageName)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to delete package '%s' of type '%s' for organization '%s'", packageName, packageType, org),
					resp,
					err,
				), nil
			}

			return handleDeletionResponse(resp, fmt.Sprintf("Package '%s' deleted successfully from organization '%s'", packageName, org))
		}
}

// DeleteOrgPackageVersion creates a tool to delete a specific version of a package from an organization
// Requires delete:packages scope in addition to read:packages
func DeleteOrgPackageVersion(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("delete_org_package_version",
			mcp.WithDescription(t("TOOL_DELETE_ORG_PACKAGE_VERSION_DESCRIPTION", "Delete a specific version of a package from a GitHub organization. Requires delete:packages scope.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_DELETE_ORG_PACKAGE_VERSION_USER_TITLE", "Delete organization package version"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("org",
				mcp.Required(),
				mcp.Description("Organization name"),
			),
			mcp.WithString("package_type",
				mcp.Required(),
				mcp.Description("Package type"),
				mcp.Enum("npm", "maven", "rubygems", "docker", "nuget", "container"),
			),
			mcp.WithString("package_name",
				mcp.Required(),
				mcp.Description("Package name"),
			),
			mcp.WithNumber("package_version_id",
				mcp.Required(),
				mcp.Description("Package version ID"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			org, err := RequiredParam[string](request, "org")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageType, err := RequiredParam[string](request, "package_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageName, err := RequiredParam[string](request, "package_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageVersionID, err := RequiredParam[float64](request, "package_version_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Organizations.PackageDeleteVersion(ctx, org, packageType, packageName, int64(packageVersionID))
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to delete package version %d of package '%s'", int64(packageVersionID), packageName),
					resp,
					err,
				), nil
			}

			return handleDeletionResponse(resp, fmt.Sprintf("Package version %d deleted successfully from package '%s'", int64(packageVersionID), packageName))
		}
}

// DeleteUserPackage creates a tool to delete an entire package from the authenticated user's account
// Requires delete:packages scope in addition to read:packages
func DeleteUserPackage(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("delete_user_package",
			mcp.WithDescription(t("TOOL_DELETE_USER_PACKAGE_DESCRIPTION", "Delete an entire package from the authenticated user's account. This will delete all versions of the package. Requires delete:packages scope.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_DELETE_USER_PACKAGE_USER_TITLE", "Delete user package"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("package_type",
				mcp.Required(),
				mcp.Description("Package type"),
				mcp.Enum("npm", "maven", "rubygems", "docker", "nuget", "container"),
			),
			mcp.WithString("package_name",
				mcp.Required(),
				mcp.Description("Package name"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			packageType, err := RequiredParam[string](request, "package_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageName, err := RequiredParam[string](request, "package_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Users.DeletePackage(ctx, authenticatedUser, packageType, packageName)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to delete package '%s' of type '%s'", packageName, packageType),
					resp,
					err,
				), nil
			}

			return handleDeletionResponse(resp, fmt.Sprintf("Package '%s' deleted successfully", packageName))
		}
}

// DeleteUserPackageVersion creates a tool to delete a specific version of a package from the authenticated user's account
// Requires delete:packages scope in addition to read:packages
func DeleteUserPackageVersion(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("delete_user_package_version",
			mcp.WithDescription(t("TOOL_DELETE_USER_PACKAGE_VERSION_DESCRIPTION", "Delete a specific version of a package from the authenticated user's account. Requires delete:packages scope.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_DELETE_USER_PACKAGE_VERSION_USER_TITLE", "Delete user package version"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("package_type",
				mcp.Required(),
				mcp.Description("Package type"),
				mcp.Enum("npm", "maven", "rubygems", "docker", "nuget", "container"),
			),
			mcp.WithString("package_name",
				mcp.Required(),
				mcp.Description("Package name"),
			),
			mcp.WithNumber("package_version_id",
				mcp.Required(),
				mcp.Description("Package version ID"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			packageType, err := RequiredParam[string](request, "package_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageName, err := RequiredParam[string](request, "package_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			packageVersionID, err := RequiredParam[float64](request, "package_version_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Users.PackageDeleteVersion(ctx, authenticatedUser, packageType, packageName, int64(packageVersionID))
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to delete version %d of package '%s'", int64(packageVersionID), packageName),
					resp,
					err,
				), nil
			}

			return handleDeletionResponse(resp, fmt.Sprintf("Package version %d deleted successfully from package '%s'", int64(packageVersionID), packageName))
		}
}
