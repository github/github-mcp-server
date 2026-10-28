package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NOTE: GitHub's REST API for packages does not expose download statistics.
// While download counts are visible on the GitHub web interface, they are not
// included in the API responses.

var packageTypes = []any{"npm", "maven", "rubygems", "docker", "nuget", "container"}

// packageCoordinates identifies a package (and optionally a version) owned by
// an organization or a user. An empty user refers to the authenticated user.
type packageCoordinates struct {
	owner       string
	packageType string
	packageName string
	versionID   int64
}

func packageOwnerSchema() map[string]*jsonschema.Schema {
	return map[string]*jsonschema.Schema{
		"org": {
			Type:        "string",
			Description: "Organization name. Required for org_* methods.",
		},
		"username": {
			Type:        "string",
			Description: "GitHub username for user_* methods. Omit to target the authenticated user.",
		},
		"package_type": {
			Type:        "string",
			Description: "Package type. Required for every method except list_org_packages and list_user_packages, where it is an optional filter.",
			Enum:        packageTypes,
		},
		"package_name": {
			Type:        "string",
			Description: "Package name. Required for every method except list_org_packages and list_user_packages.",
		},
		"package_version_id": {
			Type:        "number",
			Description: "Package version ID. Required for methods that target a single version.",
		},
	}
}

// requirePackageOwner reads the owner for an org_* or user_* method.
func requirePackageOwner(args map[string]any, isOrg bool) (string, error) {
	if isOrg {
		return RequiredParam[string](args, "org")
	}
	return OptionalParam[string](args, "username")
}

// requirePackage reads the owner, package type, and package name, and the
// version ID when withVersion is true.
func requirePackage(args map[string]any, isOrg, withVersion bool) (packageCoordinates, error) {
	owner, err := requirePackageOwner(args, isOrg)
	if err != nil {
		return packageCoordinates{}, err
	}
	packageType, err := RequiredParam[string](args, "package_type")
	if err != nil {
		return packageCoordinates{}, err
	}
	packageName, err := RequiredParam[string](args, "package_name")
	if err != nil {
		return packageCoordinates{}, err
	}
	c := packageCoordinates{owner: owner, packageType: packageType, packageName: packageName}
	if withVersion {
		c.versionID, err = RequiredBigInt(args, "package_version_id")
		if err != nil {
			return packageCoordinates{}, err
		}
	}
	return c, nil
}

// describeOwner returns a human-readable owner for error messages.
func describeOwner(owner string, isOrg bool) string {
	switch {
	case isOrg:
		return fmt.Sprintf("organization '%s'", owner)
	case owner == "":
		return "the authenticated user"
	default:
		return fmt.Sprintf("user '%s'", owner)
	}
}

// PackagesRead creates a consolidated tool to read GitHub Packages information.
func PackagesRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	properties := packageOwnerSchema()
	properties["method"] = &jsonschema.Schema{
		Type: "string",
		Description: `The read operation to perform.
Options are:
- 'list_org_packages' - list packages for an organization. Requires 'org'. Optional filters: 'package_type', 'visibility'.
- 'get_org_package' - get a package owned by an organization. Requires 'org', 'package_type', 'package_name'.
- 'list_org_package_versions' - list versions of an organization package. Requires 'org', 'package_type', 'package_name'. Optional filter: 'state'.
- 'get_org_package_version' - get a single version of an organization package. Requires 'org', 'package_type', 'package_name', 'package_version_id'.
- 'list_user_packages' - list packages for a user (or the authenticated user when 'username' is omitted). Optional filters: 'package_type', 'visibility'.
- 'get_user_package' - get a package owned by a user. Requires 'package_type', 'package_name'.
- 'list_user_package_versions' - list versions of a user package. Requires 'package_type', 'package_name'. Optional filter: 'state'.
- 'get_user_package_version' - get a single version of a user package. Requires 'package_type', 'package_name', 'package_version_id'.

Download statistics are not available through the GitHub REST API.`,
		Enum: []any{
			"list_org_packages", "get_org_package", "list_org_package_versions", "get_org_package_version",
			"list_user_packages", "get_user_package", "list_user_package_versions", "get_user_package_version",
		},
	}
	properties["visibility"] = &jsonschema.Schema{
		Type:        "string",
		Description: "Filter packages by visibility (list_org_packages and list_user_packages only).",
		Enum:        []any{"public", "private", "internal"},
	}
	properties["state"] = &jsonschema.Schema{
		Type:        "string",
		Description: "Filter package versions by state (list_*_package_versions only).",
		Enum:        []any{"active", "deleted"},
	}
	schema := &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"method"},
	}
	WithPagination(schema)

	return NewTool(
		ToolsetMetadataPackages,
		mcp.Tool{
			Name:        "packages_read",
			Description: t("TOOL_PACKAGES_READ_DESCRIPTION", "Get information about GitHub Packages owned by organizations and users. Supports listing packages, getting package details, and inspecting package versions."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_PACKAGES_READ_USER_TITLE", "Read package information"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		scopes.RequireAll(scopes.ReadPackages),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case "list_org_packages":
				return listPackages(ctx, client, args, true)
			case "list_user_packages":
				return listPackages(ctx, client, args, false)
			case "get_org_package":
				return getPackage(ctx, client, args, true)
			case "get_user_package":
				return getPackage(ctx, client, args, false)
			case "list_org_package_versions":
				return listPackageVersions(ctx, client, args, true)
			case "list_user_package_versions":
				return listPackageVersions(ctx, client, args, false)
			case "get_org_package_version":
				return getPackageVersion(ctx, client, args, true)
			case "get_user_package_version":
				return getPackageVersion(ctx, client, args, false)
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		},
	)
}

func listPackages(ctx context.Context, client *github.Client, args map[string]any, isOrg bool) (*mcp.CallToolResult, any, error) {
	owner, err := requirePackageOwner(args, isOrg)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	packageType, err := OptionalParam[string](args, "package_type")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	visibility, err := OptionalParam[string](args, "visibility")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	pagination, err := OptionalPaginationParams(args)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	opts := &github.PackageListOptions{
		PackageType: ToStringPtr(packageType),
		Visibility:  ToStringPtr(visibility),
		ListOptions: github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage},
	}

	var packages []*github.Package
	var resp *github.Response
	if isOrg {
		packages, resp, err = client.Organizations.ListPackages(ctx, owner, opts)
	} else {
		packages, resp, err = client.Users.ListPackages(ctx, owner, opts)
	}
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			fmt.Sprintf("failed to list packages for %s", describeOwner(owner, isOrg)),
			resp,
			err,
		), nil, nil
	}

	return MarshalledTextResult(packages), nil, nil
}

func getPackage(ctx context.Context, client *github.Client, args map[string]any, isOrg bool) (*mcp.CallToolResult, any, error) {
	c, err := requirePackage(args, isOrg, false)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	var pkg *github.Package
	var resp *github.Response
	if isOrg {
		pkg, resp, err = client.Organizations.GetPackage(ctx, c.owner, c.packageType, c.packageName)
	} else {
		pkg, resp, err = client.Users.GetPackage(ctx, c.owner, c.packageType, c.packageName)
	}
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			fmt.Sprintf("failed to get %s package '%s' for %s", c.packageType, c.packageName, describeOwner(c.owner, isOrg)),
			resp,
			err,
		), nil, nil
	}

	return MarshalledTextResult(pkg), nil, nil
}

func listPackageVersions(ctx context.Context, client *github.Client, args map[string]any, isOrg bool) (*mcp.CallToolResult, any, error) {
	c, err := requirePackage(args, isOrg, false)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	state, err := OptionalParam[string](args, "state")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	pagination, err := OptionalPaginationParams(args)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	listOptions := github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage}

	var versions []*github.PackageVersion
	var resp *github.Response
	switch {
	case isOrg:
		versions, resp, err = client.Organizations.PackageGetAllVersions(ctx, c.owner, c.packageType, c.packageName,
			&github.PackageListOptions{State: ToStringPtr(state), ListOptions: listOptions})
	case c.owner == "":
		versions, resp, err = client.Users.ListPackageVersions(ctx, c.packageType, c.packageName,
			&github.ListPackageVersionsOptions{State: state, ListOptions: listOptions})
	default:
		versions, resp, err = listOtherUserPackageVersions(ctx, client, c, state, listOptions)
	}
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			fmt.Sprintf("failed to list versions of %s package '%s' for %s", c.packageType, c.packageName, describeOwner(c.owner, isOrg)),
			resp,
			err,
		), nil, nil
	}

	return MarshalledTextResult(versions), nil, nil
}

func getPackageVersion(ctx context.Context, client *github.Client, args map[string]any, isOrg bool) (*mcp.CallToolResult, any, error) {
	c, err := requirePackage(args, isOrg, true)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	var version *github.PackageVersion
	var resp *github.Response
	if isOrg {
		version, resp, err = client.Organizations.PackageGetVersion(ctx, c.owner, c.packageType, c.packageName, c.versionID)
	} else {
		version, resp, err = client.Users.PackageGetVersion(ctx, c.owner, c.packageType, c.packageName, c.versionID)
	}
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			fmt.Sprintf("failed to get version %d of %s package '%s' for %s", c.versionID, c.packageType, c.packageName, describeOwner(c.owner, isOrg)),
			resp,
			err,
		), nil, nil
	}

	return MarshalledTextResult(version), nil, nil
}

// listOtherUserPackageVersions lists versions of a package owned by another
// user. go-github's Users.ListUserPackageVersions does not accept list
// options, although the endpoint supports state filtering and pagination.
func listOtherUserPackageVersions(ctx context.Context, client *github.Client, c packageCoordinates, state string, listOptions github.ListOptions) ([]*github.PackageVersion, *github.Response, error) {
	query := url.Values{}
	if state != "" {
		query.Set("state", state)
	}
	if listOptions.Page > 0 {
		query.Set("page", strconv.Itoa(listOptions.Page))
	}
	if listOptions.PerPage > 0 {
		query.Set("per_page", strconv.Itoa(listOptions.PerPage))
	}
	apiURL := fmt.Sprintf("users/%s/packages/%s/%s/versions?%s",
		url.PathEscape(c.owner), url.PathEscape(c.packageType), url.PathEscape(c.packageName), query.Encode())

	req, err := client.NewRequest(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, nil, err
	}
	var versions []*github.PackageVersion
	resp, err := client.Do(req, &versions)
	return versions, resp, err
}

// PackagesWrite creates a consolidated tool to delete GitHub Packages and package versions.
func PackagesWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	properties := packageOwnerSchema()
	properties["method"] = &jsonschema.Schema{
		Type: "string",
		Description: `The delete operation to perform.
Options are:
- 'delete_org_package' - delete an entire organization package, including all versions. Requires 'org', 'package_type', 'package_name'.
- 'delete_org_package_version' - delete a single version of an organization package. Requires 'org', 'package_type', 'package_name', 'package_version_id'.
- 'delete_user_package' - delete an entire user package, including all versions. Requires 'package_type', 'package_name'. Omit 'username' to target the authenticated user.
- 'delete_user_package_version' - delete a single version of a user package. Requires 'package_type', 'package_name', 'package_version_id'. Omit 'username' to target the authenticated user.

Deleted packages and versions can be restored from the GitHub web interface within 30 days.`,
		Enum: []any{"delete_org_package", "delete_org_package_version", "delete_user_package", "delete_user_package_version"},
	}
	properties["package_type"].Description = "Package type."
	properties["package_name"].Description = "Package name."

	return NewTool(
		ToolsetMetadataPackages,
		mcp.Tool{
			Name:        "packages_write",
			Description: t("TOOL_PACKAGES_WRITE_DESCRIPTION", "Delete GitHub Packages or individual package versions owned by organizations and users. Requires the delete:packages scope."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_PACKAGES_WRITE_USER_TITLE", "Delete packages"),
				ReadOnlyHint:    false,
				DestructiveHint: new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: properties,
				Required:   []string{"method", "package_type", "package_name"},
			},
		},
		scopes.RequireAll(scopes.ReadPackages, scopes.DeletePackages),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var isOrg, withVersion bool
			switch method {
			case "delete_org_package":
				isOrg = true
			case "delete_org_package_version":
				isOrg, withVersion = true, true
			case "delete_user_package":
			case "delete_user_package_version":
				withVersion = true
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}

			c, err := requirePackage(args, isOrg, withVersion)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			var resp *github.Response
			switch {
			case isOrg && withVersion:
				resp, err = client.Organizations.PackageDeleteVersion(ctx, c.owner, c.packageType, c.packageName, c.versionID)
			case isOrg:
				resp, err = client.Organizations.DeletePackage(ctx, c.owner, c.packageType, c.packageName)
			case withVersion:
				resp, err = client.Users.PackageDeleteVersion(ctx, c.owner, c.packageType, c.packageName, c.versionID)
			default:
				resp, err = client.Users.DeletePackage(ctx, c.owner, c.packageType, c.packageName)
			}

			target := fmt.Sprintf("%s package '%s'", c.packageType, c.packageName)
			if withVersion {
				target = fmt.Sprintf("version %d of %s", c.versionID, target)
			}
			target = fmt.Sprintf("%s for %s", target, describeOwner(c.owner, isOrg))

			return handleDeletionResponse(ctx, resp, err, target)
		},
	)
}

// handleDeletionResponse converts the result of a package deletion API call
// into a tool result.
func handleDeletionResponse(ctx context.Context, resp *github.Response, err error, target string) (*mcp.CallToolResult, any, error) {
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to delete %s", target), resp, err), nil, nil
	}
	if resp.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, fmt.Sprintf("failed to delete %s", target), resp, body), nil, nil
	}

	return MarshalledTextResult(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Deleted %s", target),
	}), nil, nil
}
