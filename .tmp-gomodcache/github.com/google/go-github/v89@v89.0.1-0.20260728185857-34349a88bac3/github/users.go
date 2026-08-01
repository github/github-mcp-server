// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// UsersService handles communication with the user related
// methods of the GitHub API.
//
// GitHub API docs: https://docs.github.com/rest/users?apiVersion=2022-11-28
type UsersService service

// User represents a GitHub user.
type User struct {
	Login                   *string    `json:"login,omitempty"`
	ID                      *int64     `json:"id,omitempty"`
	UserViewType            *string    `json:"user_view_type,omitempty"`
	NodeID                  *string    `json:"node_id,omitempty"`
	AvatarURL               *string    `json:"avatar_url,omitempty"`
	HTMLURL                 *string    `json:"html_url,omitempty"`
	GravatarID              *string    `json:"gravatar_id,omitempty"`
	Name                    *string    `json:"name,omitempty"`
	Company                 *string    `json:"company,omitempty"`
	Blog                    *string    `json:"blog,omitempty"`
	Location                *string    `json:"location,omitempty"`
	Email                   *string    `json:"email,omitempty"`
	NotificationEmail       *string    `json:"notification_email,omitempty"`
	Hireable                *bool      `json:"hireable,omitempty"`
	Bio                     *string    `json:"bio,omitempty"`
	TwitterUsername         *string    `json:"twitter_username,omitempty"`
	PublicRepos             *int       `json:"public_repos,omitempty"`
	PublicGists             *int       `json:"public_gists,omitempty"`
	Followers               *int       `json:"followers,omitempty"`
	Following               *int       `json:"following,omitempty"`
	CreatedAt               *Timestamp `json:"created_at,omitempty"`
	UpdatedAt               *Timestamp `json:"updated_at,omitempty"`
	SuspendedAt             *Timestamp `json:"suspended_at,omitempty"`
	Type                    *string    `json:"type,omitempty"`
	SiteAdmin               *bool      `json:"site_admin,omitempty"`
	TotalPrivateRepos       *int64     `json:"total_private_repos,omitempty"`
	OwnedPrivateRepos       *int64     `json:"owned_private_repos,omitempty"`
	PrivateGists            *int       `json:"private_gists,omitempty"`
	DiskUsage               *int       `json:"disk_usage,omitempty"`
	Collaborators           *int       `json:"collaborators,omitempty"`
	TwoFactorAuthentication *bool      `json:"two_factor_authentication,omitempty"`
	Plan                    *Plan      `json:"plan,omitempty"`
	BusinessPlus            *bool      `json:"business_plus,omitempty"`
	LdapDn                  *string    `json:"ldap_dn,omitempty"`

	// API URLs
	URL               *string `json:"url,omitempty"`
	EventsURL         *string `json:"events_url,omitempty"`
	FollowingURL      *string `json:"following_url,omitempty"`
	FollowersURL      *string `json:"followers_url,omitempty"`
	GistsURL          *string `json:"gists_url,omitempty"`
	OrganizationsURL  *string `json:"organizations_url,omitempty"`
	ReceivedEventsURL *string `json:"received_events_url,omitempty"`
	ReposURL          *string `json:"repos_url,omitempty"`
	StarredURL        *string `json:"starred_url,omitempty"`
	SubscriptionsURL  *string `json:"subscriptions_url,omitempty"`

	// TextMatches is only populated from search results that request text matches
	// See: search.go and https://docs.github.com/rest/search?apiVersion=2022-11-28#text-match-metadata
	TextMatches []*TextMatch `json:"text_matches,omitempty"`

	// Permissions identify the permissions that a user has on a given
	// repository.
	// This is only populated when calling [RepositoriesService.ListCollaborators].
	Permissions *RepositoryPermissions `json:"permissions,omitempty"`

	// RoleName identifies the role that a user has on a given repository.
	// This is only populated when calling [RepositoriesService.ListCollaborators].
	RoleName *string `json:"role_name,omitempty"`

	// Assignment identifies how a user was assigned to an organization role. Its
	// possible values are: "direct", "indirect", "mixed". This is only populated when
	// calling the ListUsersAssignedToOrgRole method.
	Assignment *string `json:"assignment,omitempty"`
	// InheritedFrom identifies the team that a user inherited their organization role
	// from. This is only populated when calling the ListUsersAssignedToOrgRole method.
	InheritedFrom []*Team `json:"inherited_from,omitempty"`

	// Role identifies the role that a user has on a given team.
	// Its possible values are: "member", "maintainer".
	// This is only populated when calling the
	// [TeamsService.ListTeamMembersBySlug] or [TeamsService.ListTeamMembersByID]
	// methods.
	Role *string `json:"role,omitempty"`

	// Inherited identifies whether a user inherited their team role from a child
	// team.
	// This is only populated when calling the
	// [TeamsService.ListTeamMembersBySlug] or [TeamsService.ListTeamMembersByID]
	// methods.
	Inherited *bool `json:"inherited,omitempty"`
}

func (u User) String() string {
	return Stringify(u)
}

// Get fetches a user. Passing the empty string will fetch the authenticated
// user.
//
// GitHub API docs: https://docs.github.com/rest/users/users?apiVersion=2022-11-28#get-a-user
//
// GitHub API docs: https://docs.github.com/rest/users/users?apiVersion=2022-11-28#get-the-authenticated-user
//
//meta:operation GET /user
//meta:operation GET /users/{username}
func (s *UsersService) Get(ctx context.Context, user string) (*User, *Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v", user)
	} else {
		u = "user"
	}
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var uResp *User
	resp, err := s.client.Do(req, &uResp)
	if err != nil {
		return nil, resp, err
	}

	return uResp, resp, nil
}

// GetByID fetches a user.
//
// GitHub API docs: https://docs.github.com/rest/users/users?apiVersion=2022-11-28#get-a-user-using-their-id
//
//meta:operation GET /user/{account_id}
func (s *UsersService) GetByID(ctx context.Context, id int64) (*User, *Response, error) {
	u := fmt.Sprintf("user/%v", id)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var user *User
	resp, err := s.client.Do(req, &user)
	if err != nil {
		return nil, resp, err
	}

	return user, resp, nil
}

// Edit the authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/users/users?apiVersion=2022-11-28#update-the-authenticated-user
//
//meta:operation PATCH /user
func (s *UsersService) Edit(ctx context.Context, body *User) (*User, *Response, error) {
	u := "user"
	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var uResp *User
	resp, err := s.client.Do(req, &uResp)
	if err != nil {
		return nil, resp, err
	}

	return uResp, resp, nil
}

// HovercardOptions specifies optional parameters to the UsersService.GetHovercard
// method.
type HovercardOptions struct {
	// SubjectType specifies the additional information to be received about the hovercard.
	// Possible values are: organization, repository, issue, pull_request. (Required when using subject_id.)
	SubjectType string `url:"subject_type"`

	// SubjectID specifies the ID for the SubjectType. (Required when using subject_type.)
	SubjectID string `url:"subject_id"`
}

// Hovercard represents hovercard information about a user.
type Hovercard struct {
	Contexts []*UserContext `json:"contexts,omitempty"`
}

// UserContext represents the contextual information about user.
type UserContext struct {
	Message *string `json:"message,omitempty"`
	Octicon *string `json:"octicon,omitempty"`
}

// GetHovercard fetches contextual information about user. It requires authentication
// via Basic Auth or via OAuth with the repo scope.
//
// GitHub API docs: https://docs.github.com/rest/users/users?apiVersion=2022-11-28#get-contextual-information-for-a-user
//
//meta:operation GET /users/{username}/hovercard
func (s *UsersService) GetHovercard(ctx context.Context, user string, opts *HovercardOptions) (*Hovercard, *Response, error) {
	u := fmt.Sprintf("users/%v/hovercard", user)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var hc *Hovercard
	resp, err := s.client.Do(req, &hc)
	if err != nil {
		return nil, resp, err
	}

	return hc, resp, nil
}

// UserListOptions specifies optional parameters to the UsersService.ListAll
// method.
type UserListOptions struct {
	// A user ID. Only return users with an ID greater than this ID.
	Since   int64 `url:"since,omitempty"`
	PerPage int   `url:"per_page,omitempty"`
}

// ListAll lists all GitHub users.
//
// To paginate through all users, populate 'Since' with the ID of the last user.
//
// GitHub API docs: https://docs.github.com/rest/users/users?apiVersion=2022-11-28#list-users
//
//meta:operation GET /users
func (s *UsersService) ListAll(ctx context.Context, opts *UserListOptions) ([]*User, *Response, error) {
	u, err := addOptions("users", opts)
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

// ListInvitations lists all currently-open repository invitations for the
// authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/collaborators/invitations?apiVersion=2022-11-28#list-repository-invitations-for-the-authenticated-user
//
//meta:operation GET /user/repository_invitations
func (s *UsersService) ListInvitations(ctx context.Context, opts *ListOptions) ([]*RepositoryInvitation, *Response, error) {
	u, err := addOptions("user/repository_invitations", opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	invites := []*RepositoryInvitation{}
	resp, err := s.client.Do(req, &invites)
	if err != nil {
		return nil, resp, err
	}

	return invites, resp, nil
}

// AcceptInvitation accepts the currently-open repository invitation for the
// authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/collaborators/invitations?apiVersion=2022-11-28#accept-a-repository-invitation
//
//meta:operation PATCH /user/repository_invitations/{invitation_id}
func (s *UsersService) AcceptInvitation(ctx context.Context, invitationID int64) (*Response, error) {
	u := fmt.Sprintf("user/repository_invitations/%v", invitationID)
	req, err := s.client.NewRequest(ctx, "PATCH", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeclineInvitation declines the currently-open repository invitation for the
// authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/collaborators/invitations?apiVersion=2022-11-28#decline-a-repository-invitation
//
//meta:operation DELETE /user/repository_invitations/{invitation_id}
func (s *UsersService) DeclineInvitation(ctx context.Context, invitationID int64) (*Response, error) {
	u := fmt.Sprintf("user/repository_invitations/%v", invitationID)
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
