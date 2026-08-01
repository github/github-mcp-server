// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestOrganizationsService_ListMembers(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/members", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"filter": "2fa_disabled",
			"role":   "admin",
			"page":   "2",
		})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &ListMembersOptions{
		PublicOnly:  false,
		Filter:      "2fa_disabled",
		Role:        "admin",
		ListOptions: ListOptions{Page: 2},
	}
	ctx := t.Context()
	members, _, err := client.Organizations.ListMembers(ctx, "o", opt)
	if err != nil {
		t.Errorf("Organizations.ListMembers returned error: %v", err)
	}

	want := []*User{{ID: Ptr(int64(1))}}
	if !cmp.Equal(members, want) {
		t.Errorf("Organizations.ListMembers returned %+v, want %+v", members, want)
	}

	const methodName = "ListMembers"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListMembers(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListMembers(ctx, "o", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListMembers_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Organizations.ListMembers(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestOrganizationsService_ListMembers_public(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/public_members", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &ListMembersOptions{PublicOnly: true}
	ctx := t.Context()
	members, _, err := client.Organizations.ListMembers(ctx, "o", opt)
	if err != nil {
		t.Errorf("Organizations.ListMembers returned error: %v", err)
	}

	want := []*User{{ID: Ptr(int64(1))}}
	if !cmp.Equal(members, want) {
		t.Errorf("Organizations.ListMembers returned %+v, want %+v", members, want)
	}
}

func TestOrganizationsService_IsMember(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/members/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	member, _, err := client.Organizations.IsMember(ctx, "o", "u")
	if err != nil {
		t.Errorf("Organizations.IsMember returned error: %v", err)
	}
	if want := true; member != want {
		t.Errorf("Organizations.IsMember returned %+v, want %+v", member, want)
	}

	const methodName = "IsMember"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.IsMember(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.IsMember(ctx, "o", "u")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

// Ensure that a 404 response is interpreted as "false" and not an error.
func TestOrganizationsService_IsMember_notMember(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/members/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	member, _, err := client.Organizations.IsMember(ctx, "o", "u")
	if err != nil {
		t.Errorf("Organizations.IsMember returned error: %+v", err)
	}
	if want := false; member != want {
		t.Errorf("Organizations.IsMember returned %+v, want %+v", member, want)
	}
}

// Ensure that a 400 response is interpreted as an actual error, and not simply
// as "false" like the above case of a 404.
func TestOrganizationsService_IsMember_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/members/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		http.Error(w, "BadRequest", http.StatusBadRequest)
	})

	ctx := t.Context()
	member, _, err := client.Organizations.IsMember(ctx, "o", "u")
	if err == nil {
		t.Error("Expected HTTP 400 response")
	}
	if want := false; member != want {
		t.Errorf("Organizations.IsMember returned %+v, want %+v", member, want)
	}
}

func TestOrganizationsService_IsMember_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Organizations.IsMember(ctx, "%", "u")
	testURLParseError(t, err)
}

func TestOrganizationsService_IsPublicMember(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/public_members/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	member, _, err := client.Organizations.IsPublicMember(ctx, "o", "u")
	if err != nil {
		t.Errorf("Organizations.IsPublicMember returned error: %v", err)
	}
	if want := true; member != want {
		t.Errorf("Organizations.IsPublicMember returned %+v, want %+v", member, want)
	}

	const methodName = "IsPublicMember"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.IsPublicMember(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.IsPublicMember(ctx, "o", "u")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

// Ensure that a 404 response is interpreted as "false" and not an error.
func TestOrganizationsService_IsPublicMember_notMember(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/public_members/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	member, _, err := client.Organizations.IsPublicMember(ctx, "o", "u")
	if err != nil {
		t.Errorf("Organizations.IsPublicMember returned error: %v", err)
	}
	if want := false; member != want {
		t.Errorf("Organizations.IsPublicMember returned %+v, want %+v", member, want)
	}
}

// Ensure that a 400 response is interpreted as an actual error, and not simply
// as "false" like the above case of a 404.
func TestOrganizationsService_IsPublicMember_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/public_members/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		http.Error(w, "BadRequest", http.StatusBadRequest)
	})

	ctx := t.Context()
	member, _, err := client.Organizations.IsPublicMember(ctx, "o", "u")
	if err == nil {
		t.Error("Expected HTTP 400 response")
	}
	if want := false; member != want {
		t.Errorf("Organizations.IsPublicMember returned %+v, want %+v", member, want)
	}
}

func TestOrganizationsService_IsPublicMember_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Organizations.IsPublicMember(ctx, "%", "u")
	testURLParseError(t, err)
}

func TestOrganizationsService_RemoveMember(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/members/u", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Organizations.RemoveMember(ctx, "o", "u")
	if err != nil {
		t.Errorf("Organizations.RemoveMember returned error: %v", err)
	}

	const methodName = "RemoveMember"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.RemoveMember(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.RemoveMember(ctx, "o", "u")
	})
}

func TestOrganizationsService_CancelInvite(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/invitations/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Organizations.CancelInvite(ctx, "o", 1)
	if err != nil {
		t.Errorf("Organizations.CancelInvite returned error: %v", err)
	}

	const methodName = "CancelInvite"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.CancelInvite(ctx, "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.CancelInvite(ctx, "o", 1)
	})
}

func TestOrganizationsService_RemoveMember_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Organizations.RemoveMember(ctx, "%", "u")
	testURLParseError(t, err)
}

func TestOrganizationsService_PublicizeMembership(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/public_members/u", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
	})

	ctx := t.Context()
	_, err := client.Organizations.PublicizeMembership(ctx, "o", "u")
	if err != nil {
		t.Errorf("Organizations.PublicizeMembership returned error: %v", err)
	}

	const methodName = "PublicizeMembership"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.PublicizeMembership(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.PublicizeMembership(ctx, "o", "u")
	})
}

func TestOrganizationsService_ConcealMembership(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/public_members/u", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Organizations.ConcealMembership(ctx, "o", "u")
	if err != nil {
		t.Errorf("Organizations.ConcealMembership returned error: %v", err)
	}

	const methodName = "ConcealMembership"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.ConcealMembership(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.ConcealMembership(ctx, "o", "u")
	})
}

func TestOrganizationsService_ListOrgMemberships(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/memberships/orgs", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"state": "active",
			"page":  "2",
		})
		fmt.Fprint(w, `[{"url":"u"}]`)
	})

	opt := &ListOrgMembershipsOptions{
		State:       "active",
		ListOptions: ListOptions{Page: 2},
	}
	ctx := t.Context()
	memberships, _, err := client.Organizations.ListOrgMemberships(ctx, opt)
	if err != nil {
		t.Errorf("Organizations.ListOrgMemberships returned error: %v", err)
	}

	want := []*Membership{{URL: Ptr("u")}}
	if !cmp.Equal(memberships, want) {
		t.Errorf("Organizations.ListOrgMemberships returned %+v, want %+v", memberships, want)
	}

	const methodName = "ListOrgMemberships"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListOrgMemberships(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_GetOrgMembership_AuthenticatedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/memberships/orgs/o", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"url":"u"}`)
	})

	ctx := t.Context()
	membership, _, err := client.Organizations.GetOrgMembership(ctx, "", "o")
	if err != nil {
		t.Errorf("Organizations.GetOrgMembership returned error: %v", err)
	}

	want := &Membership{URL: Ptr("u")}
	if !cmp.Equal(membership, want) {
		t.Errorf("Organizations.GetOrgMembership returned %+v, want %+v", membership, want)
	}

	const methodName = "GetOrgMembership"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetOrgMembership(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetOrgMembership(ctx, "", "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_GetOrgMembership_SpecifiedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/memberships/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"url":"u"}`)
	})

	ctx := t.Context()
	membership, _, err := client.Organizations.GetOrgMembership(ctx, "u", "o")
	if err != nil {
		t.Errorf("Organizations.GetOrgMembership returned error: %v", err)
	}

	want := &Membership{URL: Ptr("u")}
	if !cmp.Equal(membership, want) {
		t.Errorf("Organizations.GetOrgMembership returned %+v, want %+v", membership, want)
	}
}

func TestOrganizationsService_EditOrgMembership_AuthenticatedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &Membership{State: Ptr("active")}

	mux.HandleFunc("/user/memberships/orgs/o", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"url":"u"}`)
	})

	ctx := t.Context()
	membership, _, err := client.Organizations.EditOrgMembership(ctx, "", "o", input)
	if err != nil {
		t.Errorf("Organizations.EditOrgMembership returned error: %v", err)
	}

	want := &Membership{URL: Ptr("u")}
	if !cmp.Equal(membership, want) {
		t.Errorf("Organizations.EditOrgMembership returned %+v, want %+v", membership, want)
	}

	const methodName = "EditOrgMembership"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.EditOrgMembership(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.EditOrgMembership(ctx, "", "o", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_EditOrgMembership_SpecifiedUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &Membership{State: Ptr("active")}

	mux.HandleFunc("/orgs/o/memberships/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"url":"u"}`)
	})

	ctx := t.Context()
	membership, _, err := client.Organizations.EditOrgMembership(ctx, "u", "o", input)
	if err != nil {
		t.Errorf("Organizations.EditOrgMembership returned error: %v", err)
	}

	want := &Membership{URL: Ptr("u")}
	if !cmp.Equal(membership, want) {
		t.Errorf("Organizations.EditOrgMembership returned %+v, want %+v", membership, want)
	}
}

func TestOrganizationsService_RemoveOrgMembership(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/memberships/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Organizations.RemoveOrgMembership(ctx, "u", "o")
	if err != nil {
		t.Errorf("Organizations.RemoveOrgMembership returned error: %v", err)
	}

	const methodName = "RemoveOrgMembership"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.RemoveOrgMembership(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.RemoveOrgMembership(ctx, "u", "o")
	})
}

func TestOrganizationsService_ListPendingOrgInvitations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/invitations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "1"})
		fmt.Fprint(w, `[
				{
    					"id": 1,
    					"login": "monalisa",
    					"email": "octocat@github.com",
    					"role": "direct_member",
					"created_at": `+referenceTimeStr+`,
    					"inviter": {
      						"login": "other_user",
      						"id": 1,
      						"avatar_url": "https://github.com/images/error/other_user_happy.gif",
      						"gravatar_id": "",
      						"url": "https://api.github.com/users/other_user",
      						"html_url": "https://github.com/other_user",
      						"followers_url": "https://api.github.com/users/other_user/followers",
      						"following_url": "https://api.github.com/users/other_user/following/other_user",
      						"gists_url": "https://api.github.com/users/other_user/gists/gist_id",
      						"starred_url": "https://api.github.com/users/other_user/starred/owner/repo",
      						"subscriptions_url": "https://api.github.com/users/other_user/subscriptions",
      						"organizations_url": "https://api.github.com/users/other_user/orgs",
      						"repos_url": "https://api.github.com/users/other_user/repos",
      						"events_url": "https://api.github.com/users/other_user/events/privacy",
      						"received_events_url": "https://api.github.com/users/other_user/received_events/privacy",
      						"type": "User",
      						"site_admin": false
						},
						"team_count": 2,
						"invitation_team_url": "https://api.github.com/organizations/2/invitations/1/teams"
  				}
			]`)
	})

	opt := &ListOptions{Page: 1}
	ctx := t.Context()
	invitations, _, err := client.Organizations.ListPendingOrgInvitations(ctx, "o", opt)
	if err != nil {
		t.Errorf("Organizations.ListPendingOrgInvitations returned error: %v", err)
	}

	want := []*Invitation{
		{
			ID:        Ptr(int64(1)),
			Login:     Ptr("monalisa"),
			Email:     Ptr("octocat@github.com"),
			Role:      Ptr("direct_member"),
			CreatedAt: &referenceTimestamp,
			Inviter: &User{
				Login:             Ptr("other_user"),
				ID:                Ptr(int64(1)),
				AvatarURL:         Ptr("https://github.com/images/error/other_user_happy.gif"),
				GravatarID:        Ptr(""),
				URL:               Ptr("https://api.github.com/users/other_user"),
				HTMLURL:           Ptr("https://github.com/other_user"),
				FollowersURL:      Ptr("https://api.github.com/users/other_user/followers"),
				FollowingURL:      Ptr("https://api.github.com/users/other_user/following/other_user"),
				GistsURL:          Ptr("https://api.github.com/users/other_user/gists/gist_id"),
				StarredURL:        Ptr("https://api.github.com/users/other_user/starred/owner/repo"),
				SubscriptionsURL:  Ptr("https://api.github.com/users/other_user/subscriptions"),
				OrganizationsURL:  Ptr("https://api.github.com/users/other_user/orgs"),
				ReposURL:          Ptr("https://api.github.com/users/other_user/repos"),
				EventsURL:         Ptr("https://api.github.com/users/other_user/events/privacy"),
				ReceivedEventsURL: Ptr("https://api.github.com/users/other_user/received_events/privacy"),
				Type:              Ptr("User"),
				SiteAdmin:         Ptr(false),
			},
			TeamCount:         Ptr(2),
			InvitationTeamURL: Ptr("https://api.github.com/organizations/2/invitations/1/teams"),
		},
	}

	if !cmp.Equal(invitations, want) {
		t.Errorf("Organizations.ListPendingOrgInvitations returned %+v, want %+v", invitations, want)
	}

	const methodName = "ListPendingOrgInvitations"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListPendingOrgInvitations(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListPendingOrgInvitations(ctx, "o", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_CreateOrgInvitation(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CreateOrgInvitationOptions{
		Email: Ptr("octocat@github.com"),
		Role:  Ptr("direct_member"),
		TeamID: []int64{
			12,
			26,
		},
	}

	mux.HandleFunc("/orgs/o/invitations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprintln(w, `{"email": "octocat@github.com"}`)
	})

	ctx := t.Context()
	invitations, _, err := client.Organizations.CreateOrgInvitation(ctx, "o", input)
	if err != nil {
		t.Errorf("Organizations.CreateOrgInvitation returned error: %v", err)
	}

	want := &Invitation{Email: Ptr("octocat@github.com")}
	if !cmp.Equal(invitations, want) {
		t.Errorf("Organizations.CreateOrgInvitation returned %+v, want %+v", invitations, want)
	}

	const methodName = "CreateOrgInvitation"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.CreateOrgInvitation(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.CreateOrgInvitation(ctx, "o", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListOrgInvitationTeams(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/invitations/22/teams", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "1"})
		fmt.Fprint(w, `[
			{
				"id": 1,
				"url": "https://api.github.com/teams/1",
				"name": "Justice League",
				"slug": "justice-league",
				"description": "A great team.",
				"privacy": "closed",
				"permission": "admin",
				"members_url": "https://api.github.com/teams/1/members{/member}",
				"repositories_url": "https://api.github.com/teams/1/repos"
			  }
			]`)
	})

	opt := &ListOptions{Page: 1}
	ctx := t.Context()
	invitations, _, err := client.Organizations.ListOrgInvitationTeams(ctx, "o", "22", opt)
	if err != nil {
		t.Errorf("Organizations.ListOrgInvitationTeams returned error: %v", err)
	}

	want := []*Team{
		{
			ID:              Ptr(int64(1)),
			URL:             Ptr("https://api.github.com/teams/1"),
			Name:            Ptr("Justice League"),
			Slug:            Ptr("justice-league"),
			Description:     Ptr("A great team."),
			Privacy:         Ptr("closed"),
			Permission:      Ptr("admin"),
			MembersURL:      Ptr("https://api.github.com/teams/1/members{/member}"),
			RepositoriesURL: Ptr("https://api.github.com/teams/1/repos"),
		},
	}

	if !cmp.Equal(invitations, want) {
		t.Errorf("Organizations.ListOrgInvitationTeams returned %+v, want %+v", invitations, want)
	}

	const methodName = "ListOrgInvitationTeams"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListOrgInvitationTeams(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListOrgInvitationTeams(ctx, "o", "22", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListFailedOrgInvitations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/failed_invitations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2", "per_page": "1"})
		fmt.Fprint(w, `[
			{
			   "id":1,
			   "login":"monalisa",
			   "node_id":"MDQ6VXNlcjE=",
			   "email":"octocat@github.com",
			   "role":"direct_member",
			   "created_at":`+refTimeStr(1136178000)+`,
			   "failed_at":`+refTimeStr(1136178001)+`,
			   "failed_reason":"the reason",
			   "inviter":{
				  "login":"other_user",
				  "id":1,
				  "node_id":"MDQ6VXNlcjE=",
				  "avatar_url":"https://github.com/images/error/other_user_happy.gif",
				  "gravatar_id":"",
				  "url":"https://api.github.com/users/other_user",
				  "html_url":"https://github.com/other_user",
				  "followers_url":"https://api.github.com/users/other_user/followers",
				  "following_url":"https://api.github.com/users/other_user/following{/other_user}",
				  "gists_url":"https://api.github.com/users/other_user/gists{/gist_id}",
				  "starred_url":"https://api.github.com/users/other_user/starred{/owner}{/repo}",
				  "subscriptions_url":"https://api.github.com/users/other_user/subscriptions",
				  "organizations_url":"https://api.github.com/users/other_user/orgs",
				  "repos_url":"https://api.github.com/users/other_user/repos",
				  "events_url":"https://api.github.com/users/other_user/events{/privacy}",
				  "received_events_url":"https://api.github.com/users/other_user/received_events",
				  "type":"User",
				  "site_admin":false
			   },
			   "team_count":2,
			   "invitation_team_url":"https://api.github.com/organizations/2/invitations/1/teams"
			}
		]`)
	})

	opts := &ListOptions{Page: 2, PerPage: 1}
	ctx := t.Context()
	failedInvitations, _, err := client.Organizations.ListFailedOrgInvitations(ctx, "o", opts)
	if err != nil {
		t.Errorf("Organizations.ListFailedOrgInvitations returned error: %v", err)
	}

	want := []*Invitation{
		{
			ID:           Ptr(int64(1)),
			Login:        Ptr("monalisa"),
			NodeID:       Ptr("MDQ6VXNlcjE="),
			Email:        Ptr("octocat@github.com"),
			Role:         Ptr("direct_member"),
			CreatedAt:    refTimestamp(1136178000),
			FailedAt:     refTimestamp(1136178001),
			FailedReason: Ptr("the reason"),
			Inviter: &User{
				Login:             Ptr("other_user"),
				ID:                Ptr(int64(1)),
				NodeID:            Ptr("MDQ6VXNlcjE="),
				AvatarURL:         Ptr("https://github.com/images/error/other_user_happy.gif"),
				GravatarID:        Ptr(""),
				URL:               Ptr("https://api.github.com/users/other_user"),
				HTMLURL:           Ptr("https://github.com/other_user"),
				FollowersURL:      Ptr("https://api.github.com/users/other_user/followers"),
				FollowingURL:      Ptr("https://api.github.com/users/other_user/following{/other_user}"),
				GistsURL:          Ptr("https://api.github.com/users/other_user/gists{/gist_id}"),
				StarredURL:        Ptr("https://api.github.com/users/other_user/starred{/owner}{/repo}"),
				SubscriptionsURL:  Ptr("https://api.github.com/users/other_user/subscriptions"),
				OrganizationsURL:  Ptr("https://api.github.com/users/other_user/orgs"),
				ReposURL:          Ptr("https://api.github.com/users/other_user/repos"),
				EventsURL:         Ptr("https://api.github.com/users/other_user/events{/privacy}"),
				ReceivedEventsURL: Ptr("https://api.github.com/users/other_user/received_events"),
				Type:              Ptr("User"),
				SiteAdmin:         Ptr(false),
			},
			TeamCount:         Ptr(2),
			InvitationTeamURL: Ptr("https://api.github.com/organizations/2/invitations/1/teams"),
		},
	}

	if !cmp.Equal(failedInvitations, want) {
		t.Errorf("Organizations.ListFailedOrgInvitations returned %+v, want %+v", failedInvitations, want)
	}

	const methodName = "ListFailedOrgInvitations"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Organizations.ListFailedOrgInvitations(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListFailedOrgInvitations(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
