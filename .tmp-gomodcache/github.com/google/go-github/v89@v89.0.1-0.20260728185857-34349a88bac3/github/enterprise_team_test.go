// Copyright 2025 The go-github AUTHORS. All rights reserved.
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

func TestEnterpriseService_ListTeams(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{
			"id": 1,
			"url": "https://example.com/team1",
			"member_url": "https://example.com/members",
			"name": "Team One",
			"html_url": "https://example.com/html",
			"slug": "team-one",
			"created_at": `+refTimeStr(1136178000)+`,
			"updated_at": `+refTimeStr(1136178001)+`,
			"group_id": "99"
		}]`)
	})

	ctx := t.Context()
	opts := &ListOptions{Page: 1, PerPage: 10}
	got, _, err := client.Enterprise.ListTeams(ctx, "e", opts)
	if err != nil {
		t.Fatalf("Enterprise.ListTeams returned error: %v", err)
	}

	want := []*EnterpriseTeam{
		{
			ID:        1,
			URL:       "https://example.com/team1",
			MemberURL: "https://example.com/members",
			Name:      "Team One",
			HTMLURL:   "https://example.com/html",
			Slug:      "team-one",
			GroupID:   "99",
			CreatedAt: *refTimestamp(1136178000),
			UpdatedAt: *refTimestamp(1136178001),
		},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Enterprise.ListTeams = %+v, want %+v", got, want)
	}

	const methodName = "ListTeams"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.ListTeams(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListTeams(ctx, "e", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateTeam(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := EnterpriseTeamCreateOrUpdateRequest{
		Name: "New Team",
	}

	mux.HandleFunc("/enterprises/e/teams", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
			"id": 10,
			"name": "New Team",
			"slug": "new-team",
			"url": "https://example.com/team"
		}`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.CreateTeam(ctx, "e", input)
	if err != nil {
		t.Fatalf("Enterprise.CreateTeam returned error: %v", err)
	}

	want := &EnterpriseTeam{
		ID:   10,
		Name: "New Team",
		Slug: "new-team",
		URL:  "https://example.com/team",
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Enterprise.CreateTeam = %+v, want %+v", got, want)
	}

	const methodName = "CreateTeam"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateTeam(ctx, "e", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetTeam(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id": 2,
			"name": "Team One",
			"slug": "t1"
		}`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.GetTeam(ctx, "e", "t1")
	if err != nil {
		t.Fatalf("Enterprise.GetTeam returned error: %v", err)
	}

	want := &EnterpriseTeam{
		ID:   2,
		Name: "Team One",
		Slug: "t1",
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Enterprise.GetTeam = %+v, want %+v", got, want)
	}

	const methodName = "GetTeam"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.GetTeam(ctx, "\n", "t1")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetTeam(ctx, "e", "t1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_UpdateTeam(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := EnterpriseTeamCreateOrUpdateRequest{
		Name: "Updated Team",
	}

	mux.HandleFunc("/enterprises/e/teams/t1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
			"id": 3,
			"name": "Updated Team",
			"slug": "t1"
		}`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.UpdateTeam(ctx, "e", "t1", input)
	if err != nil {
		t.Fatalf("Enterprise.UpdateTeam returned error: %v", err)
	}

	want := &EnterpriseTeam{
		ID:   3,
		Name: "Updated Team",
		Slug: "t1",
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Enterprise.UpdateTeam = %+v, want %+v", got, want)
	}

	const methodName = "UpdateTeam"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.UpdateTeam(ctx, "\n", "t1", input)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.UpdateTeam(ctx, "e", "t1", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_DeleteTeam(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Enterprise.DeleteTeam(ctx, "e", "t1")
	if err != nil {
		t.Fatalf("Enterprise.DeleteTeam returned error: %v", err)
	}

	const methodName = "DeleteTeam"
	testBadOptions(t, methodName, func() error {
		_, err := client.Enterprise.DeleteTeam(ctx, "\n", "t1")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.DeleteTeam(ctx, "e", "t1")
	})
}

func TestEnterpriseService_ListTeamMembers(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1/memberships", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{
			"login": "user1",
			"id": 1001,
			"url": "https://example.com/user1"
		}]`)
	})
	ctx := t.Context()
	opts := &ListOptions{Page: 1, PerPage: 10}
	got, _, err := client.Enterprise.ListTeamMembers(ctx, "e", "t1", opts)
	if err != nil {
		t.Fatalf("Enterprise.ListTeamMembers returned error: %v", err)
	}

	want := []*User{
		{
			Login: Ptr("user1"),
			ID:    Ptr(int64(1001)),
			URL:   Ptr("https://example.com/user1"),
		},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Enterprise.ListTeamMembers = %+v, want %+v", got, want)
	}

	const methodName = "ListTeamMembers"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.ListTeamMembers(ctx, "\n", "t1", opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListTeamMembers(ctx, "e", "t1", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_BulkAddTeamMembers(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1/memberships/add", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")

		fmt.Fprint(w, `[{
			"login": "u1",
			"id": 1
		},{
			"login": "u2",
			"id": 2
		}]`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.BulkAddTeamMembers(ctx, "e", "t1", []string{"u1", "u2"})
	if err != nil {
		t.Fatalf("BulkAddTeamMembers returned error: %v", err)
	}

	want := []*User{
		{Login: Ptr("u1"), ID: Ptr(int64(1))},
		{Login: Ptr("u2"), ID: Ptr(int64(2))},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("BulkAddTeamMembers = %+v, want %+v", got, want)
	}

	const methodName = "BulkAddTeamMembers"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.BulkAddTeamMembers(ctx, "\n", "t1", []string{"u1"})
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.BulkAddTeamMembers(ctx, "e", "t1", []string{"u1"})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_BulkRemoveTeamMembers(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1/memberships/remove", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")

		fmt.Fprint(w, `[{
			"login": "u1",
			"id": 1
		},{
			"login": "u2",
			"id": 2
		}]`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.BulkRemoveTeamMembers(ctx, "e", "t1", []string{"u1", "u2"})
	if err != nil {
		t.Fatalf("BulkRemoveTeamMembers returned error: %v", err)
	}

	want := []*User{
		{Login: Ptr("u1"), ID: Ptr(int64(1))},
		{Login: Ptr("u2"), ID: Ptr(int64(2))},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("BulkRemoveTeamMembers = %+v, want %+v", got, want)
	}

	const methodName = "BulkRemoveTeamMembers"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.BulkRemoveTeamMembers(ctx, "\n", "t1", []string{"u1"})
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.BulkRemoveTeamMembers(ctx, "e", "t1", []string{"u1"})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetTeamMembership(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1/memberships/u1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"login": "u1",
			"id": 10
		}`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.GetTeamMembership(ctx, "e", "t1", "u1")
	if err != nil {
		t.Fatalf("GetTeamMembership returned error: %v", err)
	}

	want := &User{
		Login: Ptr("u1"),
		ID:    Ptr(int64(10)),
	}

	if !cmp.Equal(got, want) {
		t.Errorf("GetTeamMembership = %+v, want %+v", got, want)
	}

	const methodName = "GetTeamMembership"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.GetTeamMembership(ctx, "\n", "t1", "u1")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetTeamMembership(ctx, "e", "t1", "u1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_AddTeamMember(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1/memberships/u1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		fmt.Fprint(w, `{
			"login": "u1",
			"id": 5
		}`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.AddTeamMember(ctx, "e", "t1", "u1")
	if err != nil {
		t.Fatalf("AddTeamMember returned error: %v", err)
	}

	want := &User{
		Login: Ptr("u1"),
		ID:    Ptr(int64(5)),
	}

	if !cmp.Equal(got, want) {
		t.Errorf("AddTeamMember = %+v, want %+v", got, want)
	}

	const methodName = "AddTeamMember"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.AddTeamMember(ctx, "\n", "t1", "u1")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.AddTeamMember(ctx, "e", "t1", "u1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_RemoveTeamMember(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1/memberships/u1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Enterprise.RemoveTeamMember(ctx, "e", "t1", "u1")
	if err != nil {
		t.Fatalf("RemoveTeamMember returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("RemoveTeamMember returned nil Response")
	}

	const methodName = "RemoveTeamMember"
	testBadOptions(t, methodName, func() error {
		_, err := client.Enterprise.RemoveTeamMember(ctx, "\n", "t1", "u1")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.RemoveTeamMember(ctx, "e", "t1", "u1")
	})
}

func TestEnterpriseService_ListAssignments(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1/organizations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[
        {
            "login": "team-one",
            "id": 1,
            "node_id": "node-id",
            "url": "https://example.com/team1",
            "repos_url": "https://example.com/members",
            "events_url": "https://example.com/events",
            "hooks_url": "https://api.github.com/orgs/team-one/hooks",
            "issues_url": "https://api.github.com/orgs/team-one/issues",
            "members_url": "https://api.github.com/orgs/team-one/members",
            "public_members_url": "https://api.github.com/orgs/team-one/public_members",
            "avatar_url": "https://github.com/images/error/octocat_happy.gif",
            "description": "Team One"
        }
    ]`)
	})

	ctx := t.Context()
	opts := &ListOptions{Page: 1, PerPage: 10}
	got, _, err := client.Enterprise.ListAssignments(ctx, "e", "t1", opts)
	if err != nil {
		t.Fatalf("Enterprise.ListAssignments returned error: %v", err)
	}

	want := []*Organization{
		{
			Login:            Ptr("team-one"),
			URL:              Ptr("https://example.com/team1"),
			NodeID:           Ptr("node-id"),
			ReposURL:         Ptr("https://example.com/members"),
			EventsURL:        Ptr("https://example.com/events"),
			ID:               Ptr(int64(1)),
			HooksURL:         Ptr("https://api.github.com/orgs/team-one/hooks"),
			IssuesURL:        Ptr("https://api.github.com/orgs/team-one/issues"),
			MembersURL:       Ptr("https://api.github.com/orgs/team-one/members"),
			PublicMembersURL: Ptr("https://api.github.com/orgs/team-one/public_members"),
			AvatarURL:        Ptr("https://github.com/images/error/octocat_happy.gif"),
			Description:      Ptr("Team One"),
		},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Enterprise.ListAssignments = %+v, want %+v", got, want)
	}

	const methodName = "ListAssignments"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.ListAssignments(ctx, "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ListAssignments(ctx, "e", "t1", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_AddMultipleAssignments(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1/organizations/add", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")

		fmt.Fprint(w, `[{
			"login": "o1",
			"id": 1
		},{
			"login": "o2",
			"id": 2
		}]`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.AddMultipleAssignments(ctx, "e", "t1", []string{"o1", "o2"})
	if err != nil {
		t.Fatalf("AddMultipleAssignments returned error: %v", err)
	}

	want := []*Organization{
		{Login: Ptr("o1"), ID: Ptr(int64(1))},
		{Login: Ptr("o2"), ID: Ptr(int64(2))},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("AddMultipleAssignments = %+v, want %+v", got, want)
	}

	const methodName = "AddMultipleAssignments"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.AddMultipleAssignments(ctx, "\n", "t1", []string{"o1"})
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.AddMultipleAssignments(ctx, "e", "t1", []string{"o1"})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_RemoveMultipleAssignments(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1/organizations/remove", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")

		fmt.Fprint(w, `[{
			"login": "o1",
			"id": 1
		},{
			"login": "o2",
			"id": 2
		}]`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.RemoveMultipleAssignments(ctx, "e", "t1", []string{"o1", "o2"})
	if err != nil {
		t.Fatalf("RemoveMultipleAssignments returned error: %v", err)
	}

	want := []*Organization{
		{Login: Ptr("o1"), ID: Ptr(int64(1))},
		{Login: Ptr("o2"), ID: Ptr(int64(2))},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("RemoveMultipleAssignments = %+v, want %+v", got, want)
	}

	const methodName = "RemoveMultipleAssignments"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.RemoveMultipleAssignments(ctx, "\n", "t1", []string{"o1"})
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.RemoveMultipleAssignments(ctx, "e", "t1", []string{"o1"})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetAssignment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1/organizations/o1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"login": "o1",
			"id": 10
		}`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.GetAssignment(ctx, "e", "t1", "o1")
	if err != nil {
		t.Fatalf("GetAssignment returned error: %v", err)
	}

	want := &Organization{
		Login: Ptr("o1"),
		ID:    Ptr(int64(10)),
	}

	if !cmp.Equal(got, want) {
		t.Errorf("GetAssignment = %+v, want %+v", got, want)
	}

	const methodName = "GetAssignment"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.GetAssignment(ctx, "\n", "t1", "o1")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetAssignment(ctx, "e", "t1", "o1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_AddAssignment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1/organizations/o1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		fmt.Fprint(w, `{
			"login": "o1",
			"id": 5
		}`)
	})

	ctx := t.Context()
	got, _, err := client.Enterprise.AddAssignment(ctx, "e", "t1", "o1")
	if err != nil {
		t.Fatalf("AddAssignment returned error: %v", err)
	}

	want := &Organization{
		Login: Ptr("o1"),
		ID:    Ptr(int64(5)),
	}

	if !cmp.Equal(got, want) {
		t.Errorf("AddAssignment = %+v, want %+v", got, want)
	}

	const methodName = "AddAssignment"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Enterprise.AddAssignment(ctx, "\n", "t1", "o1")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.AddAssignment(ctx, "e", "t1", "o1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_RemoveAssignment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/teams/t1/organizations/o1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Enterprise.RemoveAssignment(ctx, "e", "t1", "o1")
	if err != nil {
		t.Fatalf("RemoveAssignment returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("RemoveAssignment returned nil Response")
	}

	const methodName = "RemoveAssignment"
	testBadOptions(t, methodName, func() error {
		_, err := client.Enterprise.RemoveAssignment(ctx, "\n", "t1", "o1")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.RemoveAssignment(ctx, "e", "t1", "o1")
	})
}
