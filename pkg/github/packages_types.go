package github

import (
	"encoding/json"

	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
)

const (
	packagesMethodListOrgPackages        = "list_org_packages"
	packagesMethodGetOrgPackage          = "get_org_package"
	packagesMethodListOrgPackageVersions = "list_org_package_versions"
	packagesMethodGetOrgPackageVersion   = "get_org_package_version"
	packagesMethodListUserPackages       = "list_user_packages"
	packagesMethodGetUserPackage         = "get_user_package"
	packagesMethodListUserVersions       = "list_user_package_versions"
	packagesMethodGetUserPackageVersion  = "get_user_package_version"

	packagesMethodDeleteOrgPackage         = "delete_org_package"
	packagesMethodDeleteOrgPackageVersion  = "delete_org_package_version"
	packagesMethodDeleteUserPackage        = "delete_user_package"
	packagesMethodDeleteUserPackageVersion = "delete_user_package_version"
)

var packagesReadMethods = []any{
	packagesMethodListOrgPackages, packagesMethodGetOrgPackage, packagesMethodListOrgPackageVersions, packagesMethodGetOrgPackageVersion,
	packagesMethodListUserPackages, packagesMethodGetUserPackage, packagesMethodListUserVersions, packagesMethodGetUserPackageVersion,
}

var packagesWriteMethods = []any{
	packagesMethodDeleteOrgPackage, packagesMethodDeleteOrgPackageVersion, packagesMethodDeleteUserPackage, packagesMethodDeleteUserPackageVersion,
}

// PackageOutput is the trimmed output type for a package. The owner and
// repository objects are reduced to the owner login and repository full name.
type PackageOutput struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	PackageType  string `json:"package_type"`
	Visibility   string `json:"visibility,omitempty"`
	VersionCount *int64 `json:"version_count,omitempty"`
	Owner        string `json:"owner,omitempty"`
	Repository   string `json:"repository,omitempty"`
	HTMLURL      string `json:"html_url,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

// PackageVersionOutput is the trimmed output type for a package version.
type PackageVersionOutput struct {
	ID          int64                         `json:"id"`
	Name        string                        `json:"name"`
	Description string                        `json:"description,omitempty"`
	License     string                        `json:"license,omitempty"`
	HTMLURL     string                        `json:"html_url,omitempty"`
	CreatedAt   string                        `json:"created_at,omitempty"`
	UpdatedAt   string                        `json:"updated_at,omitempty"`
	Metadata    *PackageVersionMetadataOutput `json:"metadata,omitempty"`
}

type PackageVersionMetadataOutput struct {
	PackageType string                          `json:"package_type,omitempty"`
	Container   *PackageContainerMetadataOutput `json:"container,omitempty"`
}

type PackageContainerMetadataOutput struct {
	Tags []string `json:"tags"`
}

type PackagesReadOutput struct {
	Method   string                  `json:"method"`
	Packages *[]PackageOutput        `json:"packages,omitempty"`
	Package  *PackageOutput          `json:"package,omitempty"`
	Versions *[]PackageVersionOutput `json:"versions,omitempty"`
	Version  *PackageVersionOutput   `json:"version,omitempty"`
}

type PackagesWriteOutput struct {
	Method  string `json:"method"`
	Message string `json:"message"`
}

func convertToPackageOutput(pkg *github.Package) PackageOutput {
	out := PackageOutput{
		ID:           pkg.GetID(),
		Name:         pkg.GetName(),
		PackageType:  pkg.GetPackageType(),
		Visibility:   pkg.GetVisibility(),
		VersionCount: pkg.VersionCount,
		HTMLURL:      pkg.GetHTMLURL(),
		CreatedAt:    formatMinimalTimestamp(pkg.CreatedAt),
		UpdatedAt:    formatMinimalTimestamp(pkg.UpdatedAt),
	}
	if pkg.Owner != nil {
		out.Owner = pkg.Owner.GetLogin()
	}
	if pkg.Repository != nil {
		out.Repository = pkg.Repository.GetFullName()
	}
	return out
}

func convertToPackageOutputs(packages []*github.Package) []PackageOutput {
	out := make([]PackageOutput, 0, len(packages))
	for _, pkg := range packages {
		out = append(out, convertToPackageOutput(pkg))
	}
	return out
}

func convertToPackageVersionOutput(version *github.PackageVersion) PackageVersionOutput {
	out := PackageVersionOutput{
		ID:          version.GetID(),
		Name:        version.GetName(),
		Description: version.GetDescription(),
		License:     version.GetLicense(),
		HTMLURL:     version.GetHTMLURL(),
		CreatedAt:   formatMinimalTimestamp(version.CreatedAt),
		UpdatedAt:   formatMinimalTimestamp(version.UpdatedAt),
	}
	// REST responses carry metadata as an object. Anything else (such as the
	// webhook array form) is dropped rather than failing the whole call.
	var metadata github.PackageMetadata
	if len(version.Metadata) > 0 && json.Unmarshal(version.Metadata, &metadata) == nil {
		out.Metadata = &PackageVersionMetadataOutput{PackageType: metadata.GetPackageType()}
		if metadata.Container != nil {
			tags := metadata.Container.Tags
			if tags == nil {
				tags = []string{}
			}
			out.Metadata.Container = &PackageContainerMetadataOutput{Tags: tags}
		}
	}
	return out
}

func convertToPackageVersionOutputs(versions []*github.PackageVersion) []PackageVersionOutput {
	out := make([]PackageVersionOutput, 0, len(versions))
	for _, version := range versions {
		out = append(out, convertToPackageVersionOutput(version))
	}
	return out
}

func packagesReadOutputSchema() *jsonschema.Schema {
	schema := forbidOmittedNulls(repositoryOutputSchema[PackagesReadOutput]())
	schema.Properties["method"].Enum = packagesReadMethods
	schema.Properties["method"].Description = "The method that produced the corresponding typed output."
	for _, pkg := range []*jsonschema.Schema{schema.Properties["package"], schema.Properties["packages"].Items} {
		pkg.Properties["owner"].Description = "Login of the package owner."
		pkg.Properties["repository"].Description = "Full name of the repository linked to the package."
		pkg.Properties["version_count"].Description = "Number of versions. Omitted when the API does not report it, as in list results."
	}
	for _, version := range []*jsonschema.Schema{schema.Properties["version"], schema.Properties["versions"].Items} {
		tags := version.Properties["metadata"].Properties["container"].Properties["tags"]
		tags.Type, tags.Types = "array", nil
	}
	constrainMethodOutput(schema, map[string]string{
		packagesMethodListOrgPackages:        "packages",
		packagesMethodListUserPackages:       "packages",
		packagesMethodGetOrgPackage:          "package",
		packagesMethodGetUserPackage:         "package",
		packagesMethodListOrgPackageVersions: "versions",
		packagesMethodListUserVersions:       "versions",
		packagesMethodGetOrgPackageVersion:   "version",
		packagesMethodGetUserPackageVersion:  "version",
	})
	return schema
}

func packagesWriteOutputSchema() *jsonschema.Schema {
	schema := repositoryOutputSchema[PackagesWriteOutput]()
	schema.Properties["method"].Enum = packagesWriteMethods
	schema.Properties["method"].Description = "The method that produced the corresponding typed output."
	return schema
}
