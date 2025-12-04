package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
func handleDeletionResponse(resp *github.Response, successMessage string) (*mcp.CallToolResult, any, error) {
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return utils.NewToolResultError(fmt.Sprintf("deletion failed: %s", string(body))), nil, nil
	}

	result := map[string]interface{}{
		"success": true,
		"message": successMessage,
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

// PackagesRead creates a consolidated tool to read package information.
func PackagesRead(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"method": {
				Type: "string",
				Description: `Action to specify what package data needs to be retrieved from GitHub.
Possible options:
 1. list_org_packages - List packages for a GitHub organization. Requires 'org' parameter. Supports optional 'package_type' and 'visibility' filters.
 2. get_org_package - Get details of a specific package for an organization. Requires 'org', 'package_type', and 'package_name' parameters.
 3. list_package_versions - List versions of a package for an organization. Requires 'org', 'package_type', and 'package_name' parameters. Supports optional 'state' filter.
 4. get_package_version - Get details of a specific package version. Requires 'org', 'package_type', 'package_name', and 'package_version_id' parameters.
 5. list_user_packages - List packages for a GitHub user. Requires 'username' parameter. Supports optional 'package_type' and 'visibility' filters.

Note: Download statistics are not available via the GitHub REST API.`,
				Enum: []any{"list_org_packages", "get_org_package", "list_package_versions", "get_package_version", "list_user_packages"},
			},
			"org": {
				Type:        "string",
				Description: "Organization name (required for org-related methods)",
			},
			"username": {
				Type:        "string",
				Description: "GitHub username (required for list_user_packages method)",
			},
			"package_type": {
				Type:        "string",
				Description: "Package type",
				Enum:        []any{"npm", "maven", "rubygems", "docker", "nuget", "container"},
			},
			"package_name": {
				Type:        "string",
				Description: "Package name (required for get_org_package, list_package_versions, and get_package_version methods)",
			},
			"package_version_id": {
				Type:        "number",
				Description: "Package version ID (required for get_package_version method)",
			},
			"visibility": {
				Type:        "string",
				Description: "Filter by package visibility (optional for list methods)",
				Enum:        []any{"public", "private", "internal"},
			},
			"state": {
				Type:        "string",
				Description: "Filter by version state (optional for list_package_versions method)",
				Enum:        []any{"active", "deleted"},
			},
		},
		Required: []string{"method"},
	}
	WithPagination(schema)

	return mcp.Tool{
			Name:        "packages_read",
			Description: t("TOOL_PACKAGES_READ_DESCRIPTION", "Get information about GitHub packages for organizations and users. Supports listing packages, getting package details, and inspecting package versions."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_PACKAGES_READ_USER_TITLE", "Read package information"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			switch method {
			case "list_org_packages":
				result, err := listOrgPackagesImpl(ctx, client, args, pagination)
				return result, nil, err
			case "get_org_package":
				result, err := getOrgPackageImpl(ctx, client, args)
				return result, nil, err
			case "list_package_versions":
				result, err := listPackageVersionsImpl(ctx, client, args, pagination)
				return result, nil, err
			case "get_package_version":
				result, err := getPackageVersionImpl(ctx, client, args)
				return result, nil, err
			case "list_user_packages":
				result, err := listUserPackagesImpl(ctx, client, args, pagination)
				return result, nil, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		}
}

func listOrgPackagesImpl(ctx context.Context, client *github.Client, args map[string]any, pagination PaginationParams) (*mcp.CallToolResult, error) {
	org, err := RequiredParam[string](args, "org")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageType, err := OptionalParam[string](args, "package_type")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	visibility, err := OptionalParam[string](args, "visibility")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
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

	return utils.NewToolResultText(string(r)), nil
}

func getOrgPackageImpl(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, error) {
	org, err := RequiredParam[string](args, "org")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageType, err := RequiredParam[string](args, "package_type")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageName, err := RequiredParam[string](args, "package_name")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
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

	return utils.NewToolResultText(string(r)), nil
}

func listPackageVersionsImpl(ctx context.Context, client *github.Client, args map[string]any, pagination PaginationParams) (*mcp.CallToolResult, error) {
	org, err := RequiredParam[string](args, "org")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageType, err := RequiredParam[string](args, "package_type")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageName, err := RequiredParam[string](args, "package_name")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	state, err := OptionalParam[string](args, "state")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
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

	return utils.NewToolResultText(string(r)), nil
}

func getPackageVersionImpl(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, error) {
	org, err := RequiredParam[string](args, "org")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageType, err := RequiredParam[string](args, "package_type")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageName, err := RequiredParam[string](args, "package_name")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageVersionID, err := RequiredParam[float64](args, "package_version_id")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
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

	return utils.NewToolResultText(string(r)), nil
}

func listUserPackagesImpl(ctx context.Context, client *github.Client, args map[string]any, pagination PaginationParams) (*mcp.CallToolResult, error) {
	username, err := RequiredParam[string](args, "username")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageType, err := OptionalParam[string](args, "package_type")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	visibility, err := OptionalParam[string](args, "visibility")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
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

	return utils.NewToolResultText(string(r)), nil
}

// PackagesWrite creates a consolidated tool for package deletion operations.
// Requires delete:packages scope in addition to read:packages.
func PackagesWrite(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"method": {
				Type: "string",
				Description: `The write operation to perform on packages.

Available methods:
 1. delete_org_package - Delete an entire package from an organization. This will delete all versions of the package. Requires 'org', 'package_type', and 'package_name' parameters.
 2. delete_org_package_version - Delete a specific version of a package from an organization. Requires 'org', 'package_type', 'package_name', and 'package_version_id' parameters.
 3. delete_user_package - Delete an entire package from the authenticated user's account. This will delete all versions of the package. Requires 'package_type' and 'package_name' parameters.
 4. delete_user_package_version - Delete a specific version of a package from the authenticated user's account. Requires 'package_type', 'package_name', and 'package_version_id' parameters.

All operations require delete:packages scope.`,
				Enum: []any{"delete_org_package", "delete_org_package_version", "delete_user_package", "delete_user_package_version"},
			},
			"org": {
				Type:        "string",
				Description: "Organization name (required for delete_org_package and delete_org_package_version methods)",
			},
			"package_type": {
				Type:        "string",
				Description: "Package type (required for all methods)",
				Enum:        []any{"npm", "maven", "rubygems", "docker", "nuget", "container"},
			},
			"package_name": {
				Type:        "string",
				Description: "Package name (required for all methods)",
			},
			"package_version_id": {
				Type:        "number",
				Description: "Package version ID (required for delete_org_package_version and delete_user_package_version methods)",
			},
		},
		Required: []string{"method", "package_type", "package_name"},
	}

	return mcp.Tool{
			Name:        "packages_write",
			Description: t("TOOL_PACKAGES_WRITE_DESCRIPTION", "Delete packages and package versions for organizations and users. All operations require delete:packages scope."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_PACKAGES_WRITE_USER_TITLE", "Delete operations on packages"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case "delete_org_package":
				result, err := deleteOrgPackageImpl(ctx, client, args)
				return result, nil, err
			case "delete_org_package_version":
				result, err := deleteOrgPackageVersionImpl(ctx, client, args)
				return result, nil, err
			case "delete_user_package":
				result, err := deleteUserPackageImpl(ctx, client, args)
				return result, nil, err
			case "delete_user_package_version":
				result, err := deleteUserPackageVersionImpl(ctx, client, args)
				return result, nil, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		}
}

func deleteOrgPackageImpl(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, error) {
	org, err := RequiredParam[string](args, "org")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageType, err := RequiredParam[string](args, "package_type")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageName, err := RequiredParam[string](args, "package_name")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	resp, err := client.Organizations.DeletePackage(ctx, org, packageType, packageName)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			fmt.Sprintf("failed to delete package '%s' of type '%s' for organization '%s'", packageName, packageType, org),
			resp,
			err,
		), nil
	}

	result, _, err := handleDeletionResponse(resp, fmt.Sprintf("Package '%s' deleted successfully from organization '%s'", packageName, org))
	return result, err
}

func deleteOrgPackageVersionImpl(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, error) {
	org, err := RequiredParam[string](args, "org")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageType, err := RequiredParam[string](args, "package_type")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageName, err := RequiredParam[string](args, "package_name")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageVersionID, err := RequiredParam[float64](args, "package_version_id")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	resp, err := client.Organizations.PackageDeleteVersion(ctx, org, packageType, packageName, int64(packageVersionID))
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			fmt.Sprintf("failed to delete package version %d of package '%s'", int64(packageVersionID), packageName),
			resp,
			err,
		), nil
	}

	result, _, err := handleDeletionResponse(resp, fmt.Sprintf("Package version %d deleted successfully from package '%s'", int64(packageVersionID), packageName))
	return result, err
}

func deleteUserPackageImpl(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, error) {
	packageType, err := RequiredParam[string](args, "package_type")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageName, err := RequiredParam[string](args, "package_name")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	resp, err := client.Users.DeletePackage(ctx, authenticatedUser, packageType, packageName)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			fmt.Sprintf("failed to delete package '%s' of type '%s'", packageName, packageType),
			resp,
			err,
		), nil
	}

	result, _, err := handleDeletionResponse(resp, fmt.Sprintf("Package '%s' deleted successfully", packageName))
	return result, err
}

func deleteUserPackageVersionImpl(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, error) {
	packageType, err := RequiredParam[string](args, "package_type")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageName, err := RequiredParam[string](args, "package_name")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}
	packageVersionID, err := RequiredParam[float64](args, "package_version_id")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	resp, err := client.Users.PackageDeleteVersion(ctx, authenticatedUser, packageType, packageName, int64(packageVersionID))
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			fmt.Sprintf("failed to delete version %d of package '%s'", int64(packageVersionID), packageName),
			resp,
			err,
		), nil
	}

	result, _, err := handleDeletionResponse(resp, fmt.Sprintf("Package version %d deleted successfully from package '%s'", int64(packageVersionID), packageName))
	return result, err
}
