// Copyright 2025 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// EnterpriseTeam represent a team in a GitHub Enterprise.
type EnterpriseTeam struct {
	ID                        int64     `json:"id"`
	URL                       string    `json:"url"`
	MemberURL                 string    `json:"member_url"`
	Name                      string    `json:"name"`
	Description               *string   `json:"description,omitempty"`
	HTMLURL                   string    `json:"html_url"`
	Slug                      string    `json:"slug"`
	CreatedAt                 Timestamp `json:"created_at"`
	UpdatedAt                 Timestamp `json:"updated_at"`
	GroupID                   string    `json:"group_id"`
	OrganizationSelectionType *string   `json:"organization_selection_type,omitempty"`
}

// EnterpriseTeamCreateOrUpdateRequest is used to create or update an enterprise team.
type EnterpriseTeamCreateOrUpdateRequest struct {
	// The name of the team.
	Name string `json:"name"`
	// A description of the team.
	Description *string `json:"description,omitempty"`
	// Specifies which organizations in the enterprise should have access to this team.
	// Possible values are "disabled" , "all" and "selected". If not specified, the default is "disabled".
	OrganizationSelectionType *string `json:"organization_selection_type,omitempty"`
	// The ID of the IdP group to assign team membership with.
	GroupID *string `json:"group_id,omitempty"`
}

// ListTeams lists all teams in an enterprise.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-teams?apiVersion=2022-11-28#list-enterprise-teams
//
//meta:operation GET /enterprises/{enterprise}/teams
func (s *EnterpriseService) ListTeams(ctx context.Context, enterprise string, opts *ListOptions) ([]*EnterpriseTeam, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams", enterprise)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var teams []*EnterpriseTeam
	resp, err := s.client.Do(req, &teams)
	if err != nil {
		return nil, resp, err
	}

	return teams, resp, nil
}

// CreateTeam creates a new team in an enterprise.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-teams?apiVersion=2022-11-28#create-an-enterprise-team
//
//meta:operation POST /enterprises/{enterprise}/teams
func (s *EnterpriseService) CreateTeam(ctx context.Context, enterprise string, body EnterpriseTeamCreateOrUpdateRequest) (*EnterpriseTeam, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams", enterprise)

	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var createdTeam *EnterpriseTeam
	resp, err := s.client.Do(req, &createdTeam)
	if err != nil {
		return nil, resp, err
	}

	return createdTeam, resp, nil
}

// GetTeam retrieves a team in an enterprise.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-teams?apiVersion=2022-11-28#get-an-enterprise-team
//
//meta:operation GET /enterprises/{enterprise}/teams/{team_slug}
func (s *EnterpriseService) GetTeam(ctx context.Context, enterprise, teamSlug string) (*EnterpriseTeam, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v", enterprise, teamSlug)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var team *EnterpriseTeam
	resp, err := s.client.Do(req, &team)
	if err != nil {
		return nil, resp, err
	}

	return team, resp, nil
}

// UpdateTeam updates a team in an enterprise.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-teams?apiVersion=2022-11-28#update-an-enterprise-team
//
//meta:operation PATCH /enterprises/{enterprise}/teams/{team_slug}
func (s *EnterpriseService) UpdateTeam(ctx context.Context, enterprise, teamSlug string, body EnterpriseTeamCreateOrUpdateRequest) (*EnterpriseTeam, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v", enterprise, teamSlug)

	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var updatedTeam *EnterpriseTeam
	resp, err := s.client.Do(req, &updatedTeam)
	if err != nil {
		return nil, resp, err
	}

	return updatedTeam, resp, nil
}

// DeleteTeam deletes a team in an enterprise.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-teams?apiVersion=2022-11-28#delete-an-enterprise-team
//
//meta:operation DELETE /enterprises/{enterprise}/teams/{team_slug}
func (s *EnterpriseService) DeleteTeam(ctx context.Context, enterprise, teamSlug string) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v", enterprise, teamSlug)

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

// ListTeamMembers lists all members of an enterprise team.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-team-members?apiVersion=2022-11-28#list-members-in-an-enterprise-team
//
//meta:operation GET /enterprises/{enterprise}/teams/{enterprise-team}/memberships
func (s *EnterpriseService) ListTeamMembers(ctx context.Context, enterprise, enterpriseTeam string, opts *ListOptions) ([]*User, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v/memberships", enterprise, enterpriseTeam)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var members []*User
	resp, err := s.client.Do(req, &members)
	if err != nil {
		return nil, resp, err
	}

	return members, resp, nil
}

// BulkAddTeamMembers adds multiple members to an enterprise team.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-team-members?apiVersion=2022-11-28#bulk-add-team-members
//
//meta:operation POST /enterprises/{enterprise}/teams/{enterprise-team}/memberships/add
func (s *EnterpriseService) BulkAddTeamMembers(ctx context.Context, enterprise, enterpriseTeam string, username []string) ([]*User, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v/memberships/add", enterprise, enterpriseTeam)
	req, err := s.client.NewRequest(ctx, "POST", u, map[string][]string{"usernames": username})
	if err != nil {
		return nil, nil, err
	}

	var members []*User
	resp, err := s.client.Do(req, &members)
	if err != nil {
		return nil, resp, err
	}

	return members, resp, nil
}

// BulkRemoveTeamMembers removes multiple members from an enterprise team.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-team-members?apiVersion=2022-11-28#bulk-remove-team-members
//
//meta:operation POST /enterprises/{enterprise}/teams/{enterprise-team}/memberships/remove
func (s *EnterpriseService) BulkRemoveTeamMembers(ctx context.Context, enterprise, enterpriseTeam string, username []string) ([]*User, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v/memberships/remove", enterprise, enterpriseTeam)
	req, err := s.client.NewRequest(ctx, "POST", u, map[string][]string{"usernames": username})
	if err != nil {
		return nil, nil, err
	}

	var members []*User
	resp, err := s.client.Do(req, &members)
	if err != nil {
		return nil, resp, err
	}

	return members, resp, nil
}

// GetTeamMembership retrieves a team membership for a user in an enterprise team.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-team-members?apiVersion=2022-11-28#get-enterprise-team-membership
//
//meta:operation GET /enterprises/{enterprise}/teams/{enterprise-team}/memberships/{username}
func (s *EnterpriseService) GetTeamMembership(ctx context.Context, enterprise, enterpriseTeam, username string) (*User, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v/memberships/%v", enterprise, enterpriseTeam, username)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var membership *User
	resp, err := s.client.Do(req, &membership)
	if err != nil {
		return nil, resp, err
	}

	return membership, resp, nil
}

// AddTeamMember adds a member to an enterprise team.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-team-members?apiVersion=2022-11-28#add-team-member
//
//meta:operation PUT /enterprises/{enterprise}/teams/{enterprise-team}/memberships/{username}
func (s *EnterpriseService) AddTeamMember(ctx context.Context, enterprise, enterpriseTeam, username string) (*User, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v/memberships/%v", enterprise, enterpriseTeam, username)

	req, err := s.client.NewRequest(ctx, "PUT", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var member *User
	resp, err := s.client.Do(req, &member)
	if err != nil {
		return nil, resp, err
	}

	return member, resp, nil
}

// RemoveTeamMember removes a member from an enterprise team.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-team-members?apiVersion=2022-11-28#remove-team-membership
//
//meta:operation DELETE /enterprises/{enterprise}/teams/{enterprise-team}/memberships/{username}
func (s *EnterpriseService) RemoveTeamMember(ctx context.Context, enterprise, enterpriseTeam, username string) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v/memberships/%v", enterprise, enterpriseTeam, username)

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

// ListAssignments gets all organizations assigned to an enterprise team.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-team-organizations?apiVersion=2022-11-28#get-organization-assignments
//
//meta:operation GET /enterprises/{enterprise}/teams/{enterprise-team}/organizations
func (s *EnterpriseService) ListAssignments(ctx context.Context, enterprise, enterpriseTeam string, opts *ListOptions) ([]*Organization, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v/organizations", enterprise, enterpriseTeam)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var orgs []*Organization
	resp, err := s.client.Do(req, &orgs)
	if err != nil {
		return nil, resp, err
	}

	return orgs, resp, nil
}

// AddMultipleAssignments assigns an enterprise team to multiple organizations.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-team-organizations?apiVersion=2022-11-28#add-organization-assignments
//
//meta:operation POST /enterprises/{enterprise}/teams/{enterprise-team}/organizations/add
func (s *EnterpriseService) AddMultipleAssignments(ctx context.Context, enterprise, enterpriseTeam string, organizationSlugs []string) ([]*Organization, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v/organizations/add", enterprise, enterpriseTeam)

	req, err := s.client.NewRequest(ctx, "POST", u, map[string][]string{"organization_slugs": organizationSlugs})
	if err != nil {
		return nil, nil, err
	}

	var orgs []*Organization
	resp, err := s.client.Do(req, &orgs)
	if err != nil {
		return nil, resp, err
	}

	return orgs, resp, nil
}

// RemoveMultipleAssignments unassigns an enterprise team from multiple organizations.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-team-organizations?apiVersion=2022-11-28#remove-organization-assignments
//
//meta:operation POST /enterprises/{enterprise}/teams/{enterprise-team}/organizations/remove
func (s *EnterpriseService) RemoveMultipleAssignments(ctx context.Context, enterprise, enterpriseTeam string, organizationSlugs []string) ([]*Organization, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v/organizations/remove", enterprise, enterpriseTeam)

	req, err := s.client.NewRequest(ctx, "POST", u, map[string][]string{"organization_slugs": organizationSlugs})
	if err != nil {
		return nil, nil, err
	}

	var orgs []*Organization
	resp, err := s.client.Do(req, &orgs)
	if err != nil {
		return nil, resp, err
	}

	return orgs, resp, nil
}

// GetAssignment checks if an enterprise team is assigned to an organization.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-team-organizations?apiVersion=2022-11-28#get-organization-assignment
//
//meta:operation GET /enterprises/{enterprise}/teams/{enterprise-team}/organizations/{org}
func (s *EnterpriseService) GetAssignment(ctx context.Context, enterprise, enterpriseTeam, org string) (*Organization, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v/organizations/%v", enterprise, enterpriseTeam, org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var organization *Organization
	resp, err := s.client.Do(req, &organization)
	if err != nil {
		return nil, resp, err
	}

	return organization, resp, nil
}

// AddAssignment assigns an enterprise team to an organizations.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-team-organizations?apiVersion=2022-11-28#add-an-organization-assignment
//
//meta:operation PUT /enterprises/{enterprise}/teams/{enterprise-team}/organizations/{org}
func (s *EnterpriseService) AddAssignment(ctx context.Context, enterprise, enterpriseTeam, org string) (*Organization, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v/organizations/%v", enterprise, enterpriseTeam, org)

	req, err := s.client.NewRequest(ctx, "PUT", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var organization *Organization
	resp, err := s.client.Do(req, &organization)
	if err != nil {
		return nil, resp, err
	}

	return organization, resp, nil
}

// RemoveAssignment unassigns an enterprise team from an organizations.
//
// GitHub API docs: https://docs.github.com/rest/enterprise-teams/enterprise-team-organizations?apiVersion=2022-11-28#delete-an-organization-assignment
//
//meta:operation DELETE /enterprises/{enterprise}/teams/{enterprise-team}/organizations/{org}
func (s *EnterpriseService) RemoveAssignment(ctx context.Context, enterprise, enterpriseTeam, org string) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/teams/%v/organizations/%v", enterprise, enterpriseTeam, org)

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
