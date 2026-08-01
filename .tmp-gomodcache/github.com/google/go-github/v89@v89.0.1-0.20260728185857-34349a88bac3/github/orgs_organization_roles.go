// Copyright 2022 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// OrganizationCustomRoles represents custom organization roles available in specified organization.
type OrganizationCustomRoles struct {
	TotalCount      *int             `json:"total_count,omitempty"`
	CustomRepoRoles []*CustomOrgRole `json:"roles,omitempty"`
}

// CustomOrgRole represents custom organization role available in specified organization.
type CustomOrgRole struct {
	ID          *int64        `json:"id,omitempty"`
	Name        *string       `json:"name,omitempty"`
	Description *string       `json:"description,omitempty"`
	Permissions []string      `json:"permissions,omitempty"`
	Org         *Organization `json:"organization,omitempty"`
	CreatedAt   *Timestamp    `json:"created_at,omitempty"`
	UpdatedAt   *Timestamp    `json:"updated_at,omitempty"`
	Source      *string       `json:"source,omitempty"`
	BaseRole    *string       `json:"base_role,omitempty"`
}

// CreateCustomOrgRoleRequest represents body parameters required to create a custom organization role.
type CreateCustomOrgRoleRequest struct {
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Permissions []string `json:"permissions"`
	BaseRole    *string  `json:"base_role,omitempty"` // Can be one of: read, triage, write, maintain, admin
}

// UpdateCustomOrgRoleRequest represents body parameters to update a custom organization role.
type UpdateCustomOrgRoleRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	BaseRole    *string  `json:"base_role,omitempty"` // Can be one of: none, read, triage, write, maintain, admin
}

// OrganizationFineGrainedPermission represents a fine-grained permission that protects organization resources.
type OrganizationFineGrainedPermission struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListRoles lists the custom roles available in this organization.
// In order to see custom roles in an organization, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/rest/orgs/organization-roles?apiVersion=2022-11-28#get-all-organization-roles-for-an-organization
//
//meta:operation GET /orgs/{org}/organization-roles
func (s *OrganizationsService) ListRoles(ctx context.Context, org string) (*OrganizationCustomRoles, *Response, error) {
	u := fmt.Sprintf("orgs/%v/organization-roles", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var customRepoRoles *OrganizationCustomRoles
	resp, err := s.client.Do(req, &customRepoRoles)
	if err != nil {
		return nil, resp, err
	}

	return customRepoRoles, resp, nil
}

// GetOrgRole gets an organization role in this organization.
// In order to get organization roles in an organization, the authenticated user must be an organization owner, or have access via an organization role.
//
// GitHub API docs: https://docs.github.com/rest/orgs/organization-roles?apiVersion=2022-11-28#get-an-organization-role
//
//meta:operation GET /orgs/{org}/organization-roles/{role_id}
func (s *OrganizationsService) GetOrgRole(ctx context.Context, org string, roleID int64) (*CustomOrgRole, *Response, error) {
	u := fmt.Sprintf("orgs/%v/organization-roles/%v", org, roleID)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var resultingRole *CustomOrgRole
	resp, err := s.client.Do(req, &resultingRole)
	if err != nil {
		return nil, resp, err
	}

	return resultingRole, resp, err
}

// CreateCustomOrgRole creates a custom role in this organization.
// In order to create custom roles in an organization, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/orgs/organization-roles?apiVersion=2022-11-28#create-a-custom-organization-role
//
//meta:operation POST /orgs/{org}/organization-roles
func (s *OrganizationsService) CreateCustomOrgRole(ctx context.Context, org string, body CreateCustomOrgRoleRequest) (*CustomOrgRole, *Response, error) {
	u := fmt.Sprintf("orgs/%v/organization-roles", org)

	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var resultingRole *CustomOrgRole
	resp, err := s.client.Do(req, &resultingRole)
	if err != nil {
		return nil, resp, err
	}

	return resultingRole, resp, err
}

// UpdateCustomOrgRole updates a custom role in this organization.
// In order to update custom roles in an organization, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/orgs/organization-roles?apiVersion=2022-11-28#update-a-custom-organization-role
//
//meta:operation PATCH /orgs/{org}/organization-roles/{role_id}
func (s *OrganizationsService) UpdateCustomOrgRole(ctx context.Context, org string, roleID int64, body UpdateCustomOrgRoleRequest) (*CustomOrgRole, *Response, error) {
	u := fmt.Sprintf("orgs/%v/organization-roles/%v", org, roleID)

	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var resultingRole *CustomOrgRole
	resp, err := s.client.Do(req, &resultingRole)
	if err != nil {
		return nil, resp, err
	}

	return resultingRole, resp, err
}

// DeleteCustomOrgRole deletes an existing custom role in this organization.
// In order to delete custom roles in an organization, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/orgs/organization-roles?apiVersion=2022-11-28#delete-a-custom-organization-role
//
//meta:operation DELETE /orgs/{org}/organization-roles/{role_id}
func (s *OrganizationsService) DeleteCustomOrgRole(ctx context.Context, org string, roleID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/organization-roles/%v", org, roleID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	var resultingRole *CustomOrgRole
	resp, err := s.client.Do(req, &resultingRole)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// AssignOrgRoleToTeam assigns an existing organization role to a team in this organization.
// In order to assign organization roles in an organization, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/rest/orgs/organization-roles?apiVersion=2022-11-28#assign-an-organization-role-to-a-team
//
//meta:operation PUT /orgs/{org}/organization-roles/teams/{team_slug}/{role_id}
func (s *OrganizationsService) AssignOrgRoleToTeam(ctx context.Context, org, teamSlug string, roleID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/organization-roles/teams/%v/%v", org, teamSlug, roleID)

	req, err := s.client.NewRequest(ctx, "PUT", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// RemoveOrgRoleFromTeam removes an existing organization role assignment from a team in this organization.
// In order to remove organization role assignments in an organization, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/rest/orgs/organization-roles?apiVersion=2022-11-28#remove-an-organization-role-from-a-team
//
//meta:operation DELETE /orgs/{org}/organization-roles/teams/{team_slug}/{role_id}
func (s *OrganizationsService) RemoveOrgRoleFromTeam(ctx context.Context, org, teamSlug string, roleID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/organization-roles/teams/%v/%v", org, teamSlug, roleID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// AssignOrgRoleToUser assigns an existing organization role to a user in this organization.
// In order to assign organization roles in an organization, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/rest/orgs/organization-roles?apiVersion=2022-11-28#assign-an-organization-role-to-a-user
//
//meta:operation PUT /orgs/{org}/organization-roles/users/{username}/{role_id}
func (s *OrganizationsService) AssignOrgRoleToUser(ctx context.Context, org, username string, roleID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/organization-roles/users/%v/%v", org, username, roleID)

	req, err := s.client.NewRequest(ctx, "PUT", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// RemoveOrgRoleFromUser removes an existing organization role assignment from a user in this organization.
// In order to remove organization role assignments in an organization, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/rest/orgs/organization-roles?apiVersion=2022-11-28#remove-an-organization-role-from-a-user
//
//meta:operation DELETE /orgs/{org}/organization-roles/users/{username}/{role_id}
func (s *OrganizationsService) RemoveOrgRoleFromUser(ctx context.Context, org, username string, roleID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/organization-roles/users/%v/%v", org, username, roleID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// ListTeamsAssignedToOrgRole returns all teams assigned to a specific organization role.
// In order to list teams assigned to an organization role, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/rest/orgs/organization-roles?apiVersion=2022-11-28#list-teams-that-are-assigned-to-an-organization-role
//
//meta:operation GET /orgs/{org}/organization-roles/{role_id}/teams
func (s *OrganizationsService) ListTeamsAssignedToOrgRole(ctx context.Context, org string, roleID int64, opts *ListOptions) ([]*Team, *Response, error) {
	u := fmt.Sprintf("orgs/%v/organization-roles/%v/teams", org, roleID)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var teams []*Team
	resp, err := s.client.Do(req, &teams)
	if err != nil {
		return nil, resp, err
	}

	return teams, resp, nil
}

// ListUsersAssignedToOrgRole returns all users assigned to a specific organization role.
// In order to list users assigned to an organization role, the authenticated user must be an organization owner.
//
// GitHub API docs: https://docs.github.com/rest/orgs/organization-roles?apiVersion=2022-11-28#list-users-that-are-assigned-to-an-organization-role
//
//meta:operation GET /orgs/{org}/organization-roles/{role_id}/users
func (s *OrganizationsService) ListUsersAssignedToOrgRole(ctx context.Context, org string, roleID int64, opts *ListOptions) ([]*User, *Response, error) {
	u := fmt.Sprintf("orgs/%v/organization-roles/%v/users", org, roleID)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var users []*User
	resp, err := s.client.Do(req, &users)
	if err != nil {
		return nil, resp, err
	}

	return users, resp, nil
}

// ListFineGrainedPermissions lists the fine-grained permissions that can be used in custom organization roles for an organization.
//
// To use this endpoint, the authenticated user must be one of:
//   - An administrator for the organization.
//   - A user, or a user on a team, with the fine-grained permissions of `read_organization_custom_org_role` in the organization.
//
// OAuth app tokens and personal access tokens (classic) need the `admin:org` scope to use this endpoint.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/orgs/organization-roles?apiVersion=2022-11-28#list-organization-fine-grained-permissions-for-an-organization
//
//meta:operation GET /orgs/{org}/organization-fine-grained-permissions
func (s *OrganizationsService) ListFineGrainedPermissions(ctx context.Context, org string) ([]*OrganizationFineGrainedPermission, *Response, error) {
	u := fmt.Sprintf("orgs/%v/organization-fine-grained-permissions", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var permissions []*OrganizationFineGrainedPermission
	resp, err := s.client.Do(req, &permissions)
	if err != nil {
		return nil, resp, err
	}

	return permissions, resp, nil
}
