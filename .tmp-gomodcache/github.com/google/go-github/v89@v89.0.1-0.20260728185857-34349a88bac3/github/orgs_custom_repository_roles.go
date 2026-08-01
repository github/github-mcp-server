// Copyright 2024 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// OrganizationCustomRepoRoles represents custom repository roles available in specified organization.
type OrganizationCustomRepoRoles struct {
	TotalCount      *int               `json:"total_count,omitempty"`
	CustomRepoRoles []*CustomRepoRoles `json:"custom_roles,omitempty"`
}

// CustomRepoRoles represents custom repository roles for an organization.
// See https://docs.github.com/enterprise-cloud@latest/organizations/managing-peoples-access-to-your-organization-with-roles/managing-custom-repository-roles-for-an-organization
// for more information.
type CustomRepoRoles struct {
	ID          *int64        `json:"id,omitempty"`
	Name        *string       `json:"name,omitempty"`
	Description *string       `json:"description,omitempty"`
	BaseRole    *string       `json:"base_role,omitempty"`
	Permissions []string      `json:"permissions,omitempty"`
	Org         *Organization `json:"organization,omitempty"`
	CreatedAt   *Timestamp    `json:"created_at,omitempty"`
	UpdatedAt   *Timestamp    `json:"updated_at,omitempty"`
}

// CreateCustomRepoRoleRequest represents the parameters to create a custom repository role.
type CreateCustomRepoRoleRequest struct {
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	BaseRole    string   `json:"base_role"`
	Permissions []string `json:"permissions"`
}

// UpdateCustomRepoRoleRequest represents the parameters to update a custom repository role.
type UpdateCustomRepoRoleRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	BaseRole    *string  `json:"base_role,omitempty"`
	Permissions []string `json:"permissions,omitzero"`
}

// RepoFineGrainedPermission represents a fine-grained permission that can be used in a custom repository role.
type RepoFineGrainedPermission struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListCustomRepoRoles lists the custom repository roles available in this organization.
// In order to see custom repository roles in an organization, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/orgs/custom-roles?apiVersion=2022-11-28#list-custom-repository-roles-in-an-organization
//
//meta:operation GET /orgs/{org}/custom-repository-roles
func (s *OrganizationsService) ListCustomRepoRoles(ctx context.Context, org string) (*OrganizationCustomRepoRoles, *Response, error) {
	u := fmt.Sprintf("orgs/%v/custom-repository-roles", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var customRepoRoles *OrganizationCustomRepoRoles
	resp, err := s.client.Do(req, &customRepoRoles)
	if err != nil {
		return nil, resp, err
	}

	return customRepoRoles, resp, nil
}

// GetCustomRepoRole gets a custom repository roles available in this organization.
// In order to see custom repository roles in an organization, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/orgs/custom-roles?apiVersion=2022-11-28#get-a-custom-repository-role
//
//meta:operation GET /orgs/{org}/custom-repository-roles/{role_id}
func (s *OrganizationsService) GetCustomRepoRole(ctx context.Context, org string, roleID int64) (*CustomRepoRoles, *Response, error) {
	u := fmt.Sprintf("orgs/%v/custom-repository-roles/%v", org, roleID)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var resultingRole *CustomRepoRoles
	resp, err := s.client.Do(req, &resultingRole)
	if err != nil {
		return nil, resp, err
	}

	return resultingRole, resp, nil
}

// CreateCustomRepoRole creates a custom repository role in this organization.
// In order to create custom repository roles in an organization, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/orgs/custom-roles?apiVersion=2022-11-28#create-a-custom-repository-role
//
//meta:operation POST /orgs/{org}/custom-repository-roles
func (s *OrganizationsService) CreateCustomRepoRole(ctx context.Context, org string, body CreateCustomRepoRoleRequest) (*CustomRepoRoles, *Response, error) {
	u := fmt.Sprintf("orgs/%v/custom-repository-roles", org)

	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var resultingRole *CustomRepoRoles
	resp, err := s.client.Do(req, &resultingRole)
	if err != nil {
		return nil, resp, err
	}

	return resultingRole, resp, err
}

// UpdateCustomRepoRole updates a custom repository role in this organization.
// In order to update custom repository roles in an organization, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/orgs/custom-roles?apiVersion=2022-11-28#update-a-custom-repository-role
//
//meta:operation PATCH /orgs/{org}/custom-repository-roles/{role_id}
func (s *OrganizationsService) UpdateCustomRepoRole(ctx context.Context, org string, roleID int64, body UpdateCustomRepoRoleRequest) (*CustomRepoRoles, *Response, error) {
	u := fmt.Sprintf("orgs/%v/custom-repository-roles/%v", org, roleID)

	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var resultingRole *CustomRepoRoles
	resp, err := s.client.Do(req, &resultingRole)
	if err != nil {
		return nil, resp, err
	}

	return resultingRole, resp, err
}

// DeleteCustomRepoRole deletes an existing custom repository role in this organization.
// In order to delete custom repository roles in an organization, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/orgs/custom-roles?apiVersion=2022-11-28#delete-a-custom-repository-role
//
//meta:operation DELETE /orgs/{org}/custom-repository-roles/{role_id}
func (s *OrganizationsService) DeleteCustomRepoRole(ctx context.Context, org string, roleID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/custom-repository-roles/%v", org, roleID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	var resultingRole *CustomRepoRoles
	resp, err := s.client.Do(req, &resultingRole)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// ListRepositoryFineGrainedPermissions lists the fine-grained permissions that can be used in custom repository roles for an organization.
// The authenticated user must be an administrator of the organization or of a repository of the organization to use this endpoint.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/orgs/custom-roles?apiVersion=2022-11-28#list-repository-fine-grained-permissions-for-an-organization
//
//meta:operation GET /orgs/{org}/repository-fine-grained-permissions
func (s *OrganizationsService) ListRepositoryFineGrainedPermissions(ctx context.Context, org string) ([]*RepoFineGrainedPermission, *Response, error) {
	u := fmt.Sprintf("orgs/%v/repository-fine-grained-permissions", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var perms []*RepoFineGrainedPermission
	resp, err := s.client.Do(req, &perms)
	if err != nil {
		return nil, resp, err
	}

	return perms, resp, nil
}
