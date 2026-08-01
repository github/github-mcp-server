// Copyright 2025 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// PrivateRegistriesService handles communication with the private registries
// methods of the GitHub API.
//
// GitHub API docs: https://docs.github.com/rest/private-registries?apiVersion=2026-03-10
type PrivateRegistriesService service

// PrivateRegistryType represents the type of private registry.
type PrivateRegistryType string

const (
	PrivateRegistryTypeMavenRepository    PrivateRegistryType = "maven_repository"
	PrivateRegistryTypeNugetFeed          PrivateRegistryType = "nuget_feed"
	PrivateRegistryTypeGoProxyServer      PrivateRegistryType = "goproxy_server"
	PrivateRegistryTypeNpmRegistry        PrivateRegistryType = "npm_registry"
	PrivateRegistryTypeRubygemsServer     PrivateRegistryType = "rubygems_server"
	PrivateRegistryTypeCargoRegistry      PrivateRegistryType = "cargo_registry"
	PrivateRegistryTypeComposerRepository PrivateRegistryType = "composer_repository"
	PrivateRegistryTypeDockerRegistry     PrivateRegistryType = "docker_registry"
	PrivateRegistryTypeGitSource          PrivateRegistryType = "git_source"
	PrivateRegistryTypeHelmRegistry       PrivateRegistryType = "helm_registry"
	PrivateRegistryTypeHexOrganization    PrivateRegistryType = "hex_organization"
	PrivateRegistryTypeHexRepository      PrivateRegistryType = "hex_repository"
	PrivateRegistryTypePubRepository      PrivateRegistryType = "pub_repository"
	PrivateRegistryTypePythonIndex        PrivateRegistryType = "python_index"
	PrivateRegistryTypeTerraformRegistry  PrivateRegistryType = "terraform_registry"
)

// PrivateRegistryVisibility represents the visibility of a private registry.
type PrivateRegistryVisibility string

const (
	PrivateRegistryVisibilityPrivate  PrivateRegistryVisibility = "private"
	PrivateRegistryVisibilityAll      PrivateRegistryVisibility = "all"
	PrivateRegistryVisibilitySelected PrivateRegistryVisibility = "selected"
)

// PrivateRegistryAuthType represents the authentication type for a private registry.
type PrivateRegistryAuthType string

const (
	PrivateRegistryAuthTypeToken            PrivateRegistryAuthType = "token"
	PrivateRegistryAuthTypeUsernamePassword PrivateRegistryAuthType = "username_password"
	PrivateRegistryAuthTypeOIDCAzure        PrivateRegistryAuthType = "oidc_azure"
	PrivateRegistryAuthTypeOIDCAWS          PrivateRegistryAuthType = "oidc_aws"
	PrivateRegistryAuthTypeOIDCJFrog        PrivateRegistryAuthType = "oidc_jfrog"
)

// PrivateRegistry represents a private registry configuration.
type PrivateRegistry struct {
	// Name of the private registry.
	Name *string `json:"name,omitempty"`
	// RegistryType is the type of private registry. You can find the list of supported types in PrivateRegistryType.
	RegistryType *PrivateRegistryType `json:"registry_type,omitempty"`
	// AuthType is the authentication type for the private registry.
	AuthType *PrivateRegistryAuthType `json:"auth_type,omitempty"`
	// URL is the URL of the private registry.
	URL *string `json:"url,omitempty"`
	// Username to use when authenticating with the private registry.
	// This field is omitted if the private registry does not require a username for authentication.
	Username *string `json:"username,omitempty"`
	// ReplacesBase indicates whether this private registry should replace the base registry.
	ReplacesBase *bool `json:"replaces_base,omitempty"`
	// Visibility is the visibility of the private registry. Possible values are: "private", "all", and "selected".
	Visibility *PrivateRegistryVisibility `json:"visibility,omitempty"`
	// SelectedRepositoryIDs is an array of repository IDs that can access the organization private registry.
	SelectedRepositoryIDs []int64 `json:"selected_repository_ids,omitempty"`
	// TenantID is the tenant ID of the Azure AD application.
	TenantID *string `json:"tenant_id,omitempty"`
	// ClientID is the client ID of the Azure AD application.
	ClientID *string `json:"client_id,omitempty"`
	// AWSRegion is the AWS region.
	AWSRegion *string `json:"aws_region,omitempty"`
	// AccountID is the AWS account ID.
	AccountID *string `json:"account_id,omitempty"`
	// RoleName is the AWS IAM role name.
	RoleName *string `json:"role_name,omitempty"`
	// Domain is the CodeArtifact domain.
	Domain *string `json:"domain,omitempty"`
	// DomainOwner is the CodeArtifact domain owner.
	DomainOwner *string `json:"domain_owner,omitempty"`
	// JFrogOIDCProviderName is the JFrog OIDC provider name.
	JFrogOIDCProviderName *string `json:"jfrog_oidc_provider_name,omitempty"`
	// Audience is the OIDC audience.
	Audience *string `json:"audience,omitempty"`
	// IdentityMappingName is the JFrog identity mapping name.
	IdentityMappingName *string `json:"identity_mapping_name,omitempty"`
	// CreatedAt is the timestamp when the private registry was created.
	CreatedAt *Timestamp `json:"created_at,omitempty"`
	// UpdatedAt is the timestamp when the private registry was last updated.
	UpdatedAt *Timestamp `json:"updated_at,omitempty"`
}

// PrivateRegistries represents a list of private registries.
type PrivateRegistries struct {
	// TotalCount is the total number of private registries.
	TotalCount *int `json:"total_count,omitempty"`
	// Configurations is the list of private registry configurations.
	Configurations []*PrivateRegistry `json:"configurations,omitempty"`
}

// CreateOrganizationPrivateRegistry represents the payload to create a private registry.
type CreateOrganizationPrivateRegistry struct {
	// RegistryType is the type of private registry.
	// You can find the list of supported types in PrivateRegistryType.
	RegistryType PrivateRegistryType `json:"registry_type"`

	// URL is the URL of the private registry.
	URL string `json:"url"`

	// The username to use when authenticating with the private registry.
	// This field should be omitted if the private registry does not require a username for authentication.
	Username *string `json:"username,omitempty"`

	// ReplacesBase indicates whether this private registry should replace the base registry
	// (e.g., npmjs.org for npm, rubygems.org for rubygems).
	ReplacesBase *bool `json:"replaces_base,omitempty"`

	// The value for your secret, encrypted with [LibSodium](https://libsodium.gitbook.io/doc/bindings_for_other_languages)
	// using the public key retrieved from the PrivateRegistriesService.GetOrganizationPrivateRegistriesPublicKey.
	// Required when AuthType is "token" or "username_password". Should be omitted for OIDC auth types.
	EncryptedValue *string `json:"encrypted_value,omitempty"`
	// KeyID is the ID of the public key used to encrypt the secret.
	// Required when AuthType is "token" or "username_password". Should be omitted for OIDC auth types.
	KeyID *string `json:"key_id,omitempty"`
	// Visibility is the visibility of the private registry.
	// Possible values are: "private", "all", and "selected".
	Visibility PrivateRegistryVisibility `json:"visibility"`

	// An array of repository IDs that can access the organization private registry.
	// You can only provide a list of repository IDs when CreateOrganizationPrivateRegistry.Visibility is set to PrivateRegistryVisibilitySelected.
	// This field should be omitted if visibility is set to PrivateRegistryVisibilityAll or PrivateRegistryVisibilityPrivate.
	SelectedRepositoryIDs []int64 `json:"selected_repository_ids,omitempty"`

	// AuthType is the authentication type for the private registry.
	// Defaults to "token" if not specified. Use "oidc_azure", "oidc_aws", or "oidc_jfrog" for OIDC authentication.
	AuthType *string `json:"auth_type,omitempty"`

	// TenantID is the tenant ID of the Azure AD application. Required when AuthType is "oidc_azure".
	TenantID *string `json:"tenant_id,omitempty"`
	// ClientID is the client ID of the Azure AD application. Required when AuthType is "oidc_azure".
	ClientID *string `json:"client_id,omitempty"`

	// AWSRegion is the AWS region. Required when AuthType is "oidc_aws".
	AWSRegion *string `json:"aws_region,omitempty"`
	// AccountID is the AWS account ID. Required when AuthType is "oidc_aws".
	AccountID *string `json:"account_id,omitempty"`
	// RoleName is the AWS IAM role name. Required when AuthType is "oidc_aws".
	RoleName *string `json:"role_name,omitempty"`
	// Domain is the CodeArtifact domain. Required when AuthType is "oidc_aws".
	Domain *string `json:"domain,omitempty"`
	// DomainOwner is the CodeArtifact domain owner (AWS account ID). Required when AuthType is "oidc_aws".
	DomainOwner *string `json:"domain_owner,omitempty"`

	// JFrogOIDCProviderName is the JFrog OIDC provider name. Required when AuthType is "oidc_jfrog".
	JFrogOIDCProviderName *string `json:"jfrog_oidc_provider_name,omitempty"`

	// Audience is the OIDC audience. Optional for "oidc_aws" and "oidc_jfrog" auth types.
	Audience *string `json:"audience,omitempty"`
	// IdentityMappingName is the JFrog identity mapping name. Optional for "oidc_jfrog" auth type.
	IdentityMappingName *string `json:"identity_mapping_name,omitempty"`
}

// UpdateOrganizationPrivateRegistry represents the payload to update a private registry.
type UpdateOrganizationPrivateRegistry struct {
	// RegistryType is the type of private registry.
	// You can find the list of supported types in PrivateRegistryType.
	RegistryType *PrivateRegistryType `json:"registry_type,omitempty"`

	// URL is the URL of the private registry.
	URL *string `json:"url,omitempty"`

	// The username to use when authenticating with the private registry.
	// This field should be omitted if the private registry does not require a username for authentication.
	Username *string `json:"username,omitempty"`

	// ReplacesBase indicates whether this private registry should replace the base registry
	// (e.g., npmjs.org for npm, rubygems.org for rubygems).
	ReplacesBase *bool `json:"replaces_base,omitempty"`

	// The value for your secret, encrypted with [LibSodium](https://libsodium.gitbook.io/doc/bindings_for_other_languages)
	// using the public key retrieved from the PrivateRegistriesService.GetOrganizationPrivateRegistriesPublicKey.
	EncryptedValue *string `json:"encrypted_value,omitempty"`
	// KeyID is the ID of the public key used to encrypt the secret.
	KeyID *string `json:"key_id,omitempty"`
	// Visibility is the visibility of the private registry.
	// Possible values are: "private", "all", and "selected".
	Visibility *PrivateRegistryVisibility `json:"visibility,omitempty"`

	// An array of repository IDs that can access the organization private registry.
	// You can only provide a list of repository IDs when UpdateOrganizationPrivateRegistry.Visibility is set to PrivateRegistryVisibilitySelected.
	// This field should be omitted if visibility is set to PrivateRegistryVisibilityAll or PrivateRegistryVisibilityPrivate.
	SelectedRepositoryIDs []int64 `json:"selected_repository_ids,omitempty"`

	// AuthType is the authentication type for the private registry.
	// This field cannot be changed after creation. If provided, it must match the existing auth_type.
	AuthType *string `json:"auth_type,omitempty"`

	// TenantID is the tenant ID of the Azure AD application. Required when AuthType is "oidc_azure".
	TenantID *string `json:"tenant_id,omitempty"`
	// ClientID is the client ID of the Azure AD application. Required when AuthType is "oidc_azure".
	ClientID *string `json:"client_id,omitempty"`

	// AWSRegion is the AWS region. Required when AuthType is "oidc_aws".
	AWSRegion *string `json:"aws_region,omitempty"`
	// AccountID is the AWS account ID. Required when AuthType is "oidc_aws".
	AccountID *string `json:"account_id,omitempty"`
	// RoleName is the AWS IAM role name. Required when AuthType is "oidc_aws".
	RoleName *string `json:"role_name,omitempty"`
	// Domain is the CodeArtifact domain. Required when AuthType is "oidc_aws".
	Domain *string `json:"domain,omitempty"`
	// DomainOwner is the CodeArtifact domain owner (AWS account ID). Required when AuthType is "oidc_aws".
	DomainOwner *string `json:"domain_owner,omitempty"`

	// JFrogOIDCProviderName is the JFrog OIDC provider name. Required when AuthType is "oidc_jfrog".
	JFrogOIDCProviderName *string `json:"jfrog_oidc_provider_name,omitempty"`

	// Audience is the OIDC audience. Optional for "oidc_aws" and "oidc_jfrog" auth types.
	Audience *string `json:"audience,omitempty"`
	// IdentityMappingName is the JFrog identity mapping name. Optional for "oidc_jfrog" auth type.
	IdentityMappingName *string `json:"identity_mapping_name,omitempty"`
}

// ListOrganizationPrivateRegistries lists private registries for an organization.
//
// GitHub API docs: https://docs.github.com/rest/private-registries/organization-configurations?apiVersion=2026-03-10#list-private-registries-for-an-organization
//
//meta:operation GET /orgs/{org}/private-registries
func (s *PrivateRegistriesService) ListOrganizationPrivateRegistries(ctx context.Context, org string, opts *ListOptions) (*PrivateRegistries, *Response, error) {
	u := fmt.Sprintf("orgs/%v/private-registries", org)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil, WithVersion(api20260310))
	if err != nil {
		return nil, nil, err
	}

	var privateRegistries *PrivateRegistries
	resp, err := s.client.Do(req, &privateRegistries)
	if err != nil {
		return nil, resp, err
	}
	return privateRegistries, resp, nil
}

// CreateOrganizationPrivateRegistry creates a private registry configuration with an encrypted value for an organization.
//
// GitHub API docs: https://docs.github.com/rest/private-registries/organization-configurations?apiVersion=2026-03-10#create-a-private-registry-for-an-organization
//
//meta:operation POST /orgs/{org}/private-registries
func (s *PrivateRegistriesService) CreateOrganizationPrivateRegistry(ctx context.Context, org string, body CreateOrganizationPrivateRegistry) (*PrivateRegistry, *Response, error) {
	u := fmt.Sprintf("orgs/%v/private-registries", org)

	req, err := s.client.NewRequest(ctx, "POST", u, body, WithVersion(api20260310))
	if err != nil {
		return nil, nil, err
	}

	var result *PrivateRegistry
	resp, err := s.client.Do(req, &result)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}

// GetOrganizationPrivateRegistriesPublicKey retrieves the public key for encrypting secrets for an organization's private registries.
//
// GitHub API docs: https://docs.github.com/rest/private-registries/organization-configurations?apiVersion=2026-03-10#get-private-registries-public-key-for-an-organization
//
//meta:operation GET /orgs/{org}/private-registries/public-key
func (s *PrivateRegistriesService) GetOrganizationPrivateRegistriesPublicKey(ctx context.Context, org string) (*PublicKey, *Response, error) {
	u := fmt.Sprintf("orgs/%v/private-registries/public-key", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil, WithVersion(api20260310))
	if err != nil {
		return nil, nil, err
	}

	var publicKey *PublicKey
	resp, err := s.client.Do(req, &publicKey)
	if err != nil {
		return nil, resp, err
	}
	return publicKey, resp, nil
}

// GetOrganizationPrivateRegistry gets a specific private registry for an organization.
// The `name` parameter is the name of the private registry to retrieve. It is the same as PrivateRegistry.Name.
//
// GitHub API docs: https://docs.github.com/rest/private-registries/organization-configurations?apiVersion=2026-03-10#get-a-private-registry-for-an-organization
//
//meta:operation GET /orgs/{org}/private-registries/{secret_name}
func (s *PrivateRegistriesService) GetOrganizationPrivateRegistry(ctx context.Context, org, secretName string) (*PrivateRegistry, *Response, error) {
	u := fmt.Sprintf("orgs/%v/private-registries/%v", org, secretName)

	req, err := s.client.NewRequest(ctx, "GET", u, nil, WithVersion(api20260310))
	if err != nil {
		return nil, nil, err
	}

	var privateRegistry *PrivateRegistry
	resp, err := s.client.Do(req, &privateRegistry)
	if err != nil {
		return nil, resp, err
	}

	return privateRegistry, resp, nil
}

// UpdateOrganizationPrivateRegistry updates a specific private registry for an organization.
// The `name` parameter is the name of the private registry to update. It is the same as PrivateRegistry.Name.
//
// GitHub API docs: https://docs.github.com/rest/private-registries/organization-configurations?apiVersion=2026-03-10#update-a-private-registry-for-an-organization
//
//meta:operation PATCH /orgs/{org}/private-registries/{secret_name}
func (s *PrivateRegistriesService) UpdateOrganizationPrivateRegistry(ctx context.Context, org, secretName string, body UpdateOrganizationPrivateRegistry) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/private-registries/%v", org, secretName)

	req, err := s.client.NewRequest(ctx, "PATCH", u, body, WithVersion(api20260310))
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteOrganizationPrivateRegistry deletes a specific private registry for an organization.
// The `name` parameter is the name of the private registry to delete. It is the same as PrivateRegistry.Name.
//
// GitHub API docs: https://docs.github.com/rest/private-registries/organization-configurations?apiVersion=2026-03-10#delete-a-private-registry-for-an-organization
//
//meta:operation DELETE /orgs/{org}/private-registries/{secret_name}
func (s *PrivateRegistriesService) DeleteOrganizationPrivateRegistry(ctx context.Context, org, secretName string) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/private-registries/%v", org, secretName)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil, WithVersion(api20260310))
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}
