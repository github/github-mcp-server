// Copyright 2018 The go-github AUTHORS. All rights reserved.
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

func TestTeamsService_ListTeams(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	teams, _, err := client.Teams.ListTeams(ctx, "o", opt)
	if err != nil {
		t.Errorf("Teams.ListTeams returned error: %v", err)
	}

	want := []*Team{{ID: Ptr(int64(1))}}
	if !cmp.Equal(teams, want) {
		t.Errorf("Teams.ListTeams returned %+v, want %+v", teams, want)
	}

	const methodName = "ListTeams"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListTeams(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListTeams(ctx, "o", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_ListTeams_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Teams.ListTeams(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestTeamsService_GetTeamByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1, "name":"n", "description": "d", "url":"u", "slug": "s", "permission":"p", "ldap_dn":"cn=n,ou=groups,dc=example,dc=com", "type": "organization", "parent":null}`)
	})

	ctx := t.Context()
	team, _, err := client.Teams.GetTeamByID(ctx, 1, 1)
	if err != nil {
		t.Errorf("Teams.GetTeamByID returned error: %v", err)
	}

	want := &Team{ID: Ptr(int64(1)), Name: Ptr("n"), Description: Ptr("d"), URL: Ptr("u"), Slug: Ptr("s"), Permission: Ptr("p"), LDAPDN: Ptr("cn=n,ou=groups,dc=example,dc=com"), Type: Ptr("organization")}
	if !cmp.Equal(team, want) {
		t.Errorf("Teams.GetTeamByID returned %+v, want %+v", team, want)
	}

	const methodName = "GetTeamByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.GetTeamByID(ctx, -1, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.GetTeamByID(ctx, 1, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_GetTeamByID_notFound(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	team, resp, err := client.Teams.GetTeamByID(ctx, 1, 2)
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Teams.GetTeamByID returned status %v, want %v", got, want)
	}
	if team != nil {
		t.Errorf("Teams.GetTeamByID returned %+v, want nil", team)
	}
}

func TestTeamsService_GetTeamBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/s", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1, "name":"n", "description": "d", "url":"u", "slug": "s", "permission":"p", "ldap_dn":"cn=n,ou=groups,dc=example,dc=com", "type": "organization", "parent":null}`)
	})

	ctx := t.Context()
	team, _, err := client.Teams.GetTeamBySlug(ctx, "o", "s")
	if err != nil {
		t.Errorf("Teams.GetTeamBySlug returned error: %v", err)
	}

	want := &Team{ID: Ptr(int64(1)), Name: Ptr("n"), Description: Ptr("d"), URL: Ptr("u"), Slug: Ptr("s"), Permission: Ptr("p"), LDAPDN: Ptr("cn=n,ou=groups,dc=example,dc=com"), Type: Ptr("organization")}
	if !cmp.Equal(team, want) {
		t.Errorf("Teams.GetTeamBySlug returned %+v, want %+v", team, want)
	}

	const methodName = "GetTeamBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.GetTeamBySlug(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.GetTeamBySlug(ctx, "o", "s")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_GetTeamBySlug_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Teams.GetTeamBySlug(ctx, "%", "s")
	testURLParseError(t, err)
}

func TestTeamsService_GetTeamBySlug_notFound(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/s", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	team, resp, err := client.Teams.GetTeamBySlug(ctx, "o", "s")
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Teams.GetTeamBySlug returned status %v, want %v", got, want)
	}
	if team != nil {
		t.Errorf("Teams.GetTeamBySlug returned %+v, want nil", team)
	}
}

func TestTeamsService_CreateTeam(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := NewTeam{Name: "n", Privacy: Ptr("closed"), RepoNames: []string{"r"}}

	mux.HandleFunc("/orgs/o/teams", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	team, _, err := client.Teams.CreateTeam(ctx, "o", input)
	if err != nil {
		t.Errorf("Teams.CreateTeam returned error: %v", err)
	}

	want := &Team{ID: Ptr(int64(1))}
	if !cmp.Equal(team, want) {
		t.Errorf("Teams.CreateTeam returned %+v, want %+v", team, want)
	}

	const methodName = "CreateTeam"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.CreateTeam(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.CreateTeam(ctx, "o", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_CreateTeam_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Teams.CreateTeam(ctx, "%", NewTeam{})
	testURLParseError(t, err)
}

func TestTeamsService_EditTeamByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := NewTeam{Name: "n", Privacy: Ptr("closed")}

	mux.HandleFunc("/organizations/1/team/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	team, _, err := client.Teams.EditTeamByID(ctx, 1, 1, input, false)
	if err != nil {
		t.Errorf("Teams.EditTeamByID returned error: %v", err)
	}

	want := &Team{ID: Ptr(int64(1))}
	if !cmp.Equal(team, want) {
		t.Errorf("Teams.EditTeamByID returned %+v, want %+v", team, want)
	}

	const methodName = "EditTeamByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.EditTeamByID(ctx, -1, -1, input, false)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.EditTeamByID(ctx, 1, 1, input, false)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_EditTeamByID_RemoveParent(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := NewTeam{Name: "n", NotificationSetting: Ptr("notifications_enabled"), Privacy: Ptr("closed")}

	mux.HandleFunc("/organizations/1/team/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)

		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	team, _, err := client.Teams.EditTeamByID(ctx, 1, 1, input, true)
	if err != nil {
		t.Errorf("Teams.EditTeamByID returned error: %v", err)
	}

	want := &Team{ID: Ptr(int64(1))}
	if !cmp.Equal(team, want) {
		t.Errorf("Teams.EditTeamByID returned %+v, want %+v", team, want)
	}
}

func TestTeamsService_EditTeamBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := NewTeam{Name: "n", Privacy: Ptr("closed")}

	mux.HandleFunc("/orgs/o/teams/s", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	team, _, err := client.Teams.EditTeamBySlug(ctx, "o", "s", input, false)
	if err != nil {
		t.Errorf("Teams.EditTeamBySlug returned error: %v", err)
	}

	want := &Team{ID: Ptr(int64(1))}
	if !cmp.Equal(team, want) {
		t.Errorf("Teams.EditTeamBySlug returned %+v, want %+v", team, want)
	}

	const methodName = "EditTeamBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.EditTeamBySlug(ctx, "\n", "\n", input, false)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.EditTeamBySlug(ctx, "o", "s", input, false)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_EditTeamBySlug_RemoveParent(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := NewTeam{Name: "n", NotificationSetting: Ptr("notifications_disabled"), Privacy: Ptr("closed")}

	mux.HandleFunc("/orgs/o/teams/s", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)

		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	team, _, err := client.Teams.EditTeamBySlug(ctx, "o", "s", input, true)
	if err != nil {
		t.Errorf("Teams.EditTeamBySlug returned error: %v", err)
	}

	want := &Team{ID: Ptr(int64(1))}
	if !cmp.Equal(team, want) {
		t.Errorf("Teams.EditTeamBySlug returned %+v, want %+v", team, want)
	}
}

func TestTeamsService_DeleteTeamByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Teams.DeleteTeamByID(ctx, 1, 1)
	if err != nil {
		t.Errorf("Teams.DeleteTeamByID returned error: %v", err)
	}

	const methodName = "DeleteTeamByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Teams.DeleteTeamByID(ctx, -1, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Teams.DeleteTeamByID(ctx, 1, 1)
	})
}

func TestTeamsService_DeleteTeamBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/s", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Teams.DeleteTeamBySlug(ctx, "o", "s")
	if err != nil {
		t.Errorf("Teams.DeleteTeamBySlug returned error: %v", err)
	}

	const methodName = "DeleteTeamBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Teams.DeleteTeamBySlug(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Teams.DeleteTeamBySlug(ctx, "o", "s")
	})
}

func TestTeamsService_ListChildTeamsByParentID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/2/teams", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":2}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	teams, _, err := client.Teams.ListChildTeamsByParentID(ctx, 1, 2, opt)
	if err != nil {
		t.Errorf("Teams.ListChildTeamsByParentID returned error: %v", err)
	}

	want := []*Team{{ID: Ptr(int64(2))}}
	if !cmp.Equal(teams, want) {
		t.Errorf("Teams.ListChildTeamsByParentID returned %+v, want %+v", teams, want)
	}

	const methodName = "ListChildTeamsByParentID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListChildTeamsByParentID(ctx, -1, -2, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListChildTeamsByParentID(ctx, 1, 2, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_ListChildTeamsByParentSlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/s/teams", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":2}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	teams, _, err := client.Teams.ListChildTeamsByParentSlug(ctx, "o", "s", opt)
	if err != nil {
		t.Errorf("Teams.ListChildTeamsByParentSlug returned error: %v", err)
	}

	want := []*Team{{ID: Ptr(int64(2))}}
	if !cmp.Equal(teams, want) {
		t.Errorf("Teams.ListChildTeamsByParentSlug returned %+v, want %+v", teams, want)
	}

	const methodName = "ListChildTeamsByParentSlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListChildTeamsByParentSlug(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListChildTeamsByParentSlug(ctx, "o", "s", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_ListTeamReposByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1/repos", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeTopicsPreview)
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	members, _, err := client.Teams.ListTeamReposByID(ctx, 1, 1, opt)
	if err != nil {
		t.Errorf("Teams.ListTeamReposByID returned error: %v", err)
	}

	want := []*Repository{{ID: Ptr(int64(1))}}
	if !cmp.Equal(members, want) {
		t.Errorf("Teams.ListTeamReposByID returned %+v, want %+v", members, want)
	}

	const methodName = "ListTeamReposByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListTeamReposByID(ctx, -1, -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListTeamReposByID(ctx, 1, 1, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_ListTeamReposBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/s/repos", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeTopicsPreview)
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	members, _, err := client.Teams.ListTeamReposBySlug(ctx, "o", "s", opt)
	if err != nil {
		t.Errorf("Teams.ListTeamReposBySlug returned error: %v", err)
	}

	want := []*Repository{{ID: Ptr(int64(1))}}
	if !cmp.Equal(members, want) {
		t.Errorf("Teams.ListTeamReposBySlug returned %+v, want %+v", members, want)
	}

	const methodName = "ListTeamReposBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListTeamReposBySlug(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListTeamReposBySlug(ctx, "o", "s", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_IsTeamRepoByID_true(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeOrgPermissionRepo)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	repo, _, err := client.Teams.IsTeamRepoByID(ctx, 1, 1, "owner", "repo")
	if err != nil {
		t.Errorf("Teams.IsTeamRepoByID returned error: %v", err)
	}

	want := &Repository{ID: Ptr(int64(1))}
	if !cmp.Equal(repo, want) {
		t.Errorf("Teams.IsTeamRepoByID returned %+v, want %+v", repo, want)
	}

	const methodName = "IsTeamRepoByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.IsTeamRepoByID(ctx, -1, -1, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.IsTeamRepoByID(ctx, 1, 1, "owner", "repo")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_IsTeamRepoBySlug_true(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/org/teams/slug/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeOrgPermissionRepo)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	repo, _, err := client.Teams.IsTeamRepoBySlug(ctx, "org", "slug", "owner", "repo")
	if err != nil {
		t.Errorf("Teams.IsTeamRepoBySlug returned error: %v", err)
	}

	want := &Repository{ID: Ptr(int64(1))}
	if !cmp.Equal(repo, want) {
		t.Errorf("Teams.IsTeamRepoBySlug returned %+v, want %+v", repo, want)
	}

	const methodName = "IsTeamRepoBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.IsTeamRepoBySlug(ctx, "\n", "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.IsTeamRepoBySlug(ctx, "org", "slug", "owner", "repo")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_IsTeamRepoByID_false(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	repo, resp, err := client.Teams.IsTeamRepoByID(ctx, 1, 1, "owner", "repo")
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Teams.IsTeamRepoByID returned status %v, want %v", got, want)
	}
	if repo != nil {
		t.Errorf("Teams.IsTeamRepoByID returned %+v, want nil", repo)
	}
}

func TestTeamsService_IsTeamRepoBySlug_false(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/org/teams/slug/repos/o/r", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	repo, resp, err := client.Teams.IsTeamRepoBySlug(ctx, "org", "slug", "owner", "repo")
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Teams.IsTeamRepoBySlug returned status %v, want %v", got, want)
	}
	if repo != nil {
		t.Errorf("Teams.IsTeamRepoBySlug returned %+v, want nil", repo)
	}
}

func TestTeamsService_IsTeamRepoByID_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		http.Error(w, "BadRequest", http.StatusBadRequest)
	})

	ctx := t.Context()
	repo, resp, err := client.Teams.IsTeamRepoByID(ctx, 1, 1, "owner", "repo")
	if err == nil {
		t.Error("Expected HTTP 400 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusBadRequest; got != want {
		t.Errorf("Teams.IsTeamRepoByID returned status %v, want %v", got, want)
	}
	if repo != nil {
		t.Errorf("Teams.IsTeamRepoByID returned %+v, want nil", repo)
	}
}

func TestTeamsService_IsTeamRepoBySlug_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/org/teams/slug/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		http.Error(w, "BadRequest", http.StatusBadRequest)
	})

	ctx := t.Context()
	repo, resp, err := client.Teams.IsTeamRepoBySlug(ctx, "org", "slug", "owner", "repo")
	if err == nil {
		t.Error("Expected HTTP 400 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusBadRequest; got != want {
		t.Errorf("Teams.IsTeamRepoBySlug returned status %v, want %v", got, want)
	}
	if repo != nil {
		t.Errorf("Teams.IsTeamRepoBySlug returned %+v, want nil", repo)
	}
}

func TestTeamsService_IsTeamRepoByID_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Teams.IsTeamRepoByID(ctx, 1, 1, "%", "r")
	testURLParseError(t, err)
}

func TestTeamsService_IsTeamRepoBySlug_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Teams.IsTeamRepoBySlug(ctx, "o", "s", "%", "r")
	testURLParseError(t, err)
}

func TestTeamsService_AddTeamRepoByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	opt := &TeamAddTeamRepoOptions{Permission: "admin"}

	mux.HandleFunc("/organizations/1/team/1/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, opt)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Teams.AddTeamRepoByID(ctx, 1, 1, "owner", "repo", opt)
	if err != nil {
		t.Errorf("Teams.AddTeamRepoByID returned error: %v", err)
	}

	const methodName = "AddTeamRepoByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Teams.AddTeamRepoByID(ctx, 1, 1, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Teams.AddTeamRepoByID(ctx, 1, 1, "owner", "repo", opt)
	})
}

func TestTeamsService_AddTeamRepoBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	opt := &TeamAddTeamRepoOptions{Permission: "admin"}

	mux.HandleFunc("/orgs/org/teams/slug/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, opt)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Teams.AddTeamRepoBySlug(ctx, "org", "slug", "owner", "repo", opt)
	if err != nil {
		t.Errorf("Teams.AddTeamRepoBySlug returned error: %v", err)
	}

	const methodName = "AddTeamRepoBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Teams.AddTeamRepoBySlug(ctx, "\n", "\n", "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Teams.AddTeamRepoBySlug(ctx, "org", "slug", "owner", "repo", opt)
	})
}

func TestTeamsService_AddTeamRepoByID_noAccess(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		w.WriteHeader(http.StatusUnprocessableEntity)
	})

	ctx := t.Context()
	_, err := client.Teams.AddTeamRepoByID(ctx, 1, 1, "owner", "repo", nil)
	if err == nil {
		t.Error("Expected error to be returned")
	}
}

func TestTeamsService_AddTeamRepoBySlug_noAccess(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/org/teams/slug/repos/o/r", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		w.WriteHeader(http.StatusUnprocessableEntity)
	})

	ctx := t.Context()
	_, err := client.Teams.AddTeamRepoBySlug(ctx, "org", "slug", "owner", "repo", nil)
	if err == nil {
		t.Error("Expected error to be returned")
	}
}

func TestTeamsService_AddTeamRepoByID_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Teams.AddTeamRepoByID(ctx, 1, 1, "%", "r", nil)
	testURLParseError(t, err)
}

func TestTeamsService_AddTeamRepoBySlug_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Teams.AddTeamRepoBySlug(ctx, "o", "s", "%", "r", nil)
	testURLParseError(t, err)
}

func TestTeamsService_RemoveTeamRepoByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Teams.RemoveTeamRepoByID(ctx, 1, 1, "owner", "repo")
	if err != nil {
		t.Errorf("Teams.RemoveTeamRepoByID returned error: %v", err)
	}

	const methodName = "RemoveTeamRepoByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Teams.RemoveTeamRepoByID(ctx, -1, -1, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Teams.RemoveTeamRepoByID(ctx, 1, 1, "owner", "repo")
	})
}

func TestTeamsService_RemoveTeamRepoBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/org/teams/slug/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Teams.RemoveTeamRepoBySlug(ctx, "org", "slug", "owner", "repo")
	if err != nil {
		t.Errorf("Teams.RemoveTeamRepoBySlug returned error: %v", err)
	}

	const methodName = "RemoveTeamRepoBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Teams.RemoveTeamRepoBySlug(ctx, "\n", "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Teams.RemoveTeamRepoBySlug(ctx, "org", "slug", "owner", "repo")
	})
}

func TestTeamsService_RemoveTeamRepoByID_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Teams.RemoveTeamRepoByID(ctx, 1, 1, "%", "r")
	testURLParseError(t, err)
}

func TestTeamsService_RemoveTeamRepoBySlug_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Teams.RemoveTeamRepoBySlug(ctx, "o", "s", "%", "r")
	testURLParseError(t, err)
}

func TestTeamsService_ListUserTeams(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/teams", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "1"})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &ListOptions{Page: 1}
	ctx := t.Context()
	teams, _, err := client.Teams.ListUserTeams(ctx, opt)
	if err != nil {
		t.Errorf("Teams.ListUserTeams returned error: %v", err)
	}

	want := []*Team{{ID: Ptr(int64(1))}}
	if !cmp.Equal(teams, want) {
		t.Errorf("Teams.ListUserTeams returned %+v, want %+v", teams, want)
	}

	const methodName = "ListUserTeams"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListUserTeams(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_ListProjectsByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1/projects", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeProjectsPreview)
		fmt.Fprint(w, `[{"id":1}]`)
	})

	ctx := t.Context()
	projects, _, err := client.Teams.ListTeamProjectsByID(ctx, 1, 1)
	if err != nil {
		t.Errorf("Teams.ListTeamProjectsByID returned error: %v", err)
	}

	want := []*ProjectV2{{ID: Ptr(int64(1))}}
	if !cmp.Equal(projects, want) {
		t.Errorf("Teams.ListTeamProjectsByID returned %+v, want %+v", projects, want)
	}

	const methodName = "ListTeamProjectsByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListTeamProjectsByID(ctx, -1, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListTeamProjectsByID(ctx, 1, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_ListProjectsBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/s/projects", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeProjectsPreview)
		fmt.Fprint(w, `[{"id":1}]`)
	})

	ctx := t.Context()
	projects, _, err := client.Teams.ListTeamProjectsBySlug(ctx, "o", "s")
	if err != nil {
		t.Errorf("Teams.ListTeamProjectsBySlug returned error: %v", err)
	}

	want := []*ProjectV2{{ID: Ptr(int64(1))}}
	if !cmp.Equal(projects, want) {
		t.Errorf("Teams.ListTeamProjectsBySlug returned %+v, want %+v", projects, want)
	}

	const methodName = "ListTeamProjectsBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListTeamProjectsBySlug(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListTeamProjectsBySlug(ctx, "o", "s")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_ReviewProjectsByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1/projects/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeProjectsPreview)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	project, _, err := client.Teams.ReviewTeamProjectsByID(ctx, 1, 1, 1)
	if err != nil {
		t.Errorf("Teams.ReviewTeamProjectsByID returned error: %v", err)
	}

	want := &ProjectV2{ID: Ptr(int64(1))}
	if !cmp.Equal(project, want) {
		t.Errorf("Teams.ReviewTeamProjectsByID returned %+v, want %+v", project, want)
	}

	const methodName = "ReviewTeamProjectsByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ReviewTeamProjectsByID(ctx, -1, -1, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ReviewTeamProjectsByID(ctx, 1, 1, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_ReviewProjectsBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/s/projects/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeProjectsPreview)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	project, _, err := client.Teams.ReviewTeamProjectsBySlug(ctx, "o", "s", 1)
	if err != nil {
		t.Errorf("Teams.ReviewTeamProjectsBySlug returned error: %v", err)
	}

	want := &ProjectV2{ID: Ptr(int64(1))}
	if !cmp.Equal(project, want) {
		t.Errorf("Teams.ReviewTeamProjectsBySlug returned %+v, want %+v", project, want)
	}

	const methodName = "ReviewTeamProjectsBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ReviewTeamProjectsBySlug(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ReviewTeamProjectsBySlug(ctx, "o", "s", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_AddTeamProjectByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	opt := &TeamProjectOptions{
		Permission: Ptr("admin"),
	}

	mux.HandleFunc("/organizations/1/team/1/projects/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Accept", mediaTypeProjectsPreview)
		testJSONBody(t, r, opt)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Teams.AddTeamProjectByID(ctx, 1, 1, 1, opt)
	if err != nil {
		t.Errorf("Teams.AddTeamProjectByID returned error: %v", err)
	}

	const methodName = "AddTeamProjectByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Teams.AddTeamProjectByID(ctx, -1, -1, -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Teams.AddTeamProjectByID(ctx, 1, 1, 1, opt)
	})
}

func TestTeamsService_AddTeamProjectBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	opt := &TeamProjectOptions{
		Permission: Ptr("admin"),
	}

	mux.HandleFunc("/orgs/o/teams/s/projects/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testHeader(t, r, "Accept", mediaTypeProjectsPreview)
		testJSONBody(t, r, opt)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Teams.AddTeamProjectBySlug(ctx, "o", "s", 1, opt)
	if err != nil {
		t.Errorf("Teams.AddTeamProjectBySlug returned error: %v", err)
	}

	const methodName = "AddTeamProjectBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Teams.AddTeamProjectBySlug(ctx, "\n", "\n", -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Teams.AddTeamProjectBySlug(ctx, "o", "s", 1, opt)
	})
}

func TestTeamsService_RemoveTeamProjectByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1/projects/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeProjectsPreview)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Teams.RemoveTeamProjectByID(ctx, 1, 1, 1)
	if err != nil {
		t.Errorf("Teams.RemoveTeamProjectByID returned error: %v", err)
	}

	const methodName = "RemoveTeamProjectByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Teams.RemoveTeamProjectByID(ctx, -1, -1, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Teams.RemoveTeamProjectByID(ctx, 1, 1, 1)
	})
}

func TestTeamsService_RemoveTeamProjectBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/s/projects/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testHeader(t, r, "Accept", mediaTypeProjectsPreview)
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Teams.RemoveTeamProjectBySlug(ctx, "o", "s", 1)
	if err != nil {
		t.Errorf("Teams.RemoveTeamProjectBySlug returned error: %v", err)
	}

	const methodName = "RemoveTeamProjectBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Teams.RemoveTeamProjectBySlug(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Teams.RemoveTeamProjectBySlug(ctx, "o", "s", 1)
	})
}

func TestTeamsService_ListIDPGroupsInOrganization(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/team-sync/groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page": "url-encoded-next-page-token",
			"q":    "n",
		})
		fmt.Fprint(w, `{"groups": [{"group_id": "1",  "group_name": "n", "group_description": "d"}]}`)
	})

	opt := &ListIDPGroupsOptions{
		Query:             "n",
		ListCursorOptions: ListCursorOptions{Page: "url-encoded-next-page-token"},
	}
	ctx := t.Context()
	groups, _, err := client.Teams.ListIDPGroupsInOrganization(ctx, "o", opt)
	if err != nil {
		t.Errorf("Teams.ListIDPGroupsInOrganization returned error: %v", err)
	}

	want := &IDPGroupList{
		Groups: []*IDPGroup{
			{
				GroupID:          Ptr("1"),
				GroupName:        Ptr("n"),
				GroupDescription: Ptr("d"),
			},
		},
	}
	if !cmp.Equal(groups, want) {
		t.Errorf("Teams.ListIDPGroupsInOrganization returned %+v. want %+v", groups, want)
	}

	const methodName = "ListIDPGroupsInOrganization"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListIDPGroupsInOrganization(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListIDPGroupsInOrganization(ctx, "o", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_ListIDPGroupsForTeamByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1/team-sync/group-mappings", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"groups": [{"group_id": "1",  "group_name": "n", "group_description": "d"}]}`)
	})

	ctx := t.Context()
	groups, _, err := client.Teams.ListIDPGroupsForTeamByID(ctx, 1, 1)
	if err != nil {
		t.Errorf("Teams.ListIDPGroupsForTeamByID returned error: %v", err)
	}

	want := &IDPGroupList{
		Groups: []*IDPGroup{
			{
				GroupID:          Ptr("1"),
				GroupName:        Ptr("n"),
				GroupDescription: Ptr("d"),
			},
		},
	}
	if !cmp.Equal(groups, want) {
		t.Errorf("Teams.ListIDPGroupsForTeamByID returned %+v. want %+v", groups, want)
	}

	const methodName = "ListIDPGroupsForTeamByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListIDPGroupsForTeamByID(ctx, -1, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListIDPGroupsForTeamByID(ctx, 1, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_ListIDPGroupsForTeamBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/slug/team-sync/group-mappings", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"groups": [{"group_id": "1",  "group_name": "n", "group_description": "d"}]}`)
	})

	ctx := t.Context()
	groups, _, err := client.Teams.ListIDPGroupsForTeamBySlug(ctx, "o", "slug")
	if err != nil {
		t.Errorf("Teams.ListIDPGroupsForTeamBySlug returned error: %v", err)
	}

	want := &IDPGroupList{
		Groups: []*IDPGroup{
			{
				GroupID:          Ptr("1"),
				GroupName:        Ptr("n"),
				GroupDescription: Ptr("d"),
			},
		},
	}
	if !cmp.Equal(groups, want) {
		t.Errorf("Teams.ListIDPGroupsForTeamBySlug returned %+v. want %+v", groups, want)
	}

	const methodName = "ListIDPGroupsForTeamBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListIDPGroupsForTeamBySlug(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListIDPGroupsForTeamBySlug(ctx, "o", "slug")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_CreateOrUpdateIDPGroupConnectionsByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1/team-sync/group-mappings", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{"groups": [{"group_id": "1",  "group_name": "n", "group_description": "d"}]}`)
	})

	input := IDPGroupList{
		Groups: []*IDPGroup{
			{
				GroupID:          Ptr("1"),
				GroupName:        Ptr("n"),
				GroupDescription: Ptr("d"),
			},
		},
	}

	ctx := t.Context()
	groups, _, err := client.Teams.CreateOrUpdateIDPGroupConnectionsByID(ctx, 1, 1, input)
	if err != nil {
		t.Errorf("Teams.CreateOrUpdateIDPGroupConnectionsByID returned error: %v", err)
	}

	want := &IDPGroupList{
		Groups: []*IDPGroup{
			{
				GroupID:          Ptr("1"),
				GroupName:        Ptr("n"),
				GroupDescription: Ptr("d"),
			},
		},
	}
	if !cmp.Equal(groups, want) {
		t.Errorf("Teams.CreateOrUpdateIDPGroupConnectionsByID returned %+v. want %+v", groups, want)
	}

	const methodName = "CreateOrUpdateIDPGroupConnectionsByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.CreateOrUpdateIDPGroupConnectionsByID(ctx, -1, -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.CreateOrUpdateIDPGroupConnectionsByID(ctx, 1, 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_CreateOrUpdateIDPGroupConnectionsBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/slug/team-sync/group-mappings", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{"groups": [{"group_id": "1",  "group_name": "n", "group_description": "d"}]}`)
	})

	input := IDPGroupList{
		Groups: []*IDPGroup{
			{
				GroupID:          Ptr("1"),
				GroupName:        Ptr("n"),
				GroupDescription: Ptr("d"),
			},
		},
	}

	ctx := t.Context()
	groups, _, err := client.Teams.CreateOrUpdateIDPGroupConnectionsBySlug(ctx, "o", "slug", input)
	if err != nil {
		t.Errorf("Teams.CreateOrUpdateIDPGroupConnectionsBySlug returned error: %v", err)
	}

	want := &IDPGroupList{
		Groups: []*IDPGroup{
			{
				GroupID:          Ptr("1"),
				GroupName:        Ptr("n"),
				GroupDescription: Ptr("d"),
			},
		},
	}
	if !cmp.Equal(groups, want) {
		t.Errorf("Teams.CreateOrUpdateIDPGroupConnectionsBySlug returned %+v. want %+v", groups, want)
	}

	const methodName = "CreateOrUpdateIDPGroupConnectionsBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.CreateOrUpdateIDPGroupConnectionsBySlug(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.CreateOrUpdateIDPGroupConnectionsBySlug(ctx, "o", "slug", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_CreateOrUpdateIDPGroupConnectionsByID_empty(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/organizations/1/team/1/team-sync/group-mappings", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{"groups": []}`)
	})

	input := IDPGroupList{
		Groups: []*IDPGroup{},
	}

	ctx := t.Context()
	groups, _, err := client.Teams.CreateOrUpdateIDPGroupConnectionsByID(ctx, 1, 1, input)
	if err != nil {
		t.Errorf("Teams.CreateOrUpdateIDPGroupConnectionsByID returned error: %v", err)
	}

	want := &IDPGroupList{
		Groups: []*IDPGroup{},
	}
	if !cmp.Equal(groups, want) {
		t.Errorf("Teams.CreateOrUpdateIDPGroupConnectionsByID returned %+v. want %+v", groups, want)
	}
}

func TestTeamsService_CreateOrUpdateIDPGroupConnectionsBySlug_empty(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/slug/team-sync/group-mappings", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{"groups": []}`)
	})

	input := IDPGroupList{
		Groups: []*IDPGroup{},
	}

	ctx := t.Context()
	groups, _, err := client.Teams.CreateOrUpdateIDPGroupConnectionsBySlug(ctx, "o", "slug", input)
	if err != nil {
		t.Errorf("Teams.CreateOrUpdateIDPGroupConnectionsBySlug returned error: %v", err)
	}

	want := &IDPGroupList{
		Groups: []*IDPGroup{},
	}
	if !cmp.Equal(groups, want) {
		t.Errorf("Teams.CreateOrUpdateIDPGroupConnectionsBySlug returned %+v. want %+v", groups, want)
	}
}

func TestTeamsService_GetExternalGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/external-group/123", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"group_id": 123,
			"group_name": "Octocat admins",
			"updated_at": "2006-01-02T15:04:05Z",
			"teams": [
				{
					"team_id": 1,
					"team_name": "team-test"
				},
				{
					"team_id": 2,
					"team_name": "team-test2"
				}
			],
			"members": [
				{
					"member_id": 1,
					"member_login": "mona-lisa_eocsaxrs",
					"member_name": "Mona Lisa",
					"member_email": "mona_lisa@github.com"
				},
				{
					"member_id": 2,
					"member_login": "octo-lisa_eocsaxrs",
					"member_name": "Octo Lisa",
					"member_email": "octo_lisa@github.com"
				}
			]
		}`)
	})

	ctx := t.Context()
	externalGroup, _, err := client.Teams.GetExternalGroup(ctx, "o", 123)
	if err != nil {
		t.Errorf("Teams.GetExternalGroup returned error: %v", err)
	}

	want := &ExternalGroup{
		GroupID:   Ptr(int64(123)),
		GroupName: Ptr("Octocat admins"),
		UpdatedAt: &Timestamp{Time: referenceTime},
		Teams: []*ExternalGroupTeam{
			{
				TeamID:   Ptr(int64(1)),
				TeamName: Ptr("team-test"),
			},
			{
				TeamID:   Ptr(int64(2)),
				TeamName: Ptr("team-test2"),
			},
		},
		Members: []*ExternalGroupMember{
			{
				MemberID:    Ptr(int64(1)),
				MemberLogin: Ptr("mona-lisa_eocsaxrs"),
				MemberName:  Ptr("Mona Lisa"),
				MemberEmail: Ptr("mona_lisa@github.com"),
			},
			{
				MemberID:    Ptr(int64(2)),
				MemberLogin: Ptr("octo-lisa_eocsaxrs"),
				MemberName:  Ptr("Octo Lisa"),
				MemberEmail: Ptr("octo_lisa@github.com"),
			},
		},
	}
	if !cmp.Equal(externalGroup, want) {
		t.Errorf("Teams.GetExternalGroup returned %+v, want %+v", externalGroup, want)
	}

	const methodName = "GetExternalGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.GetExternalGroup(ctx, "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.GetExternalGroup(ctx, "o", 123)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_GetExternalGroup_notFound(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/external-group/123", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	eg, resp, err := client.Teams.GetExternalGroup(ctx, "o", 123)
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Teams.GetExternalGroup returned status %v, want %v", got, want)
	}
	if eg != nil {
		t.Errorf("Teams.GetExternalGroup returned %+v, want nil", eg)
	}
}

func TestTeamsService_ListExternalGroups(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/external-groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"groups": [
				{
					"group_id": 123,
					"group_name": "Octocat admins",
					"updated_at": "2006-01-02T15:04:05Z"
				}
			]
		}`)
	})

	ctx := t.Context()
	opts := &ListExternalGroupsOptions{
		DisplayName: Ptr("Octocat"),
	}
	list, _, err := client.Teams.ListExternalGroups(ctx, "o", opts)
	if err != nil {
		t.Errorf("Teams.ListExternalGroups returned error: %v", err)
	}

	want := &ExternalGroupList{
		Groups: []*ExternalGroup{
			{
				GroupID:   Ptr(int64(123)),
				GroupName: Ptr("Octocat admins"),
				UpdatedAt: &Timestamp{Time: referenceTime},
			},
		},
	}
	if !cmp.Equal(list, want) {
		t.Errorf("Teams.ListExternalGroups returned %+v, want %+v", list, want)
	}

	const methodName = "ListExternalGroups"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListExternalGroups(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListExternalGroups(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_ListExternalGroups_notFound(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/external-groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	eg, resp, err := client.Teams.ListExternalGroups(ctx, "o", nil)
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Teams.ListExternalGroups returned status %v, want %v", got, want)
	}
	if eg != nil {
		t.Errorf("Teams.ListExternalGroups returned %+v, want nil", eg)
	}
}

func TestTeamsService_ListExternalGroupsForTeamBySlug(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/t/external-groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"groups": [
				{
					"group_id": 123,
					"group_name": "Octocat admins",
					"updated_at": "2006-01-02T15:04:05Z"
				}
			]
		}`)
	})

	ctx := t.Context()
	list, _, err := client.Teams.ListExternalGroupsForTeamBySlug(ctx, "o", "t")
	if err != nil {
		t.Errorf("Teams.ListExternalGroupsForTeamBySlug returned error: %v", err)
	}

	want := &ExternalGroupList{
		Groups: []*ExternalGroup{
			{
				GroupID:   Ptr(int64(123)),
				GroupName: Ptr("Octocat admins"),
				UpdatedAt: &Timestamp{Time: referenceTime},
			},
		},
	}
	if !cmp.Equal(list, want) {
		t.Errorf("Teams.ListExternalGroupsForTeamBySlug returned %+v, want %+v", list, want)
	}

	const methodName = "ListExternalGroupsForTeamBySlug"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.ListExternalGroupsForTeamBySlug(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.ListExternalGroupsForTeamBySlug(ctx, "o", "t")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_ListExternalGroupsForTeamBySlug_notFound(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/t/external-groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	eg, resp, err := client.Teams.ListExternalGroupsForTeamBySlug(ctx, "o", "t")
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Teams.ListExternalGroupsForTeamBySlug returned status %v, want %v", got, want)
	}
	if eg != nil {
		t.Errorf("Teams.ListExternalGroupsForTeamBySlug returned %+v, want nil", eg)
	}
}

func TestTeamsService_UpdateConnectedExternalGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/t/external-groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{
			"group_id": 123,
			"group_name": "Octocat admins",
			"updated_at": "2006-01-02T15:04:05Z",
			"teams": [
				{
					"team_id": 1,
					"team_name": "team-test"
				},
				{
					"team_id": 2,
					"team_name": "team-test2"
				}
			],
			"members": [
				{
					"member_id": 1,
					"member_login": "mona-lisa_eocsaxrs",
					"member_name": "Mona Lisa",
					"member_email": "mona_lisa@github.com"
				},
				{
					"member_id": 2,
					"member_login": "octo-lisa_eocsaxrs",
					"member_name": "Octo Lisa",
					"member_email": "octo_lisa@github.com"
				}
			]
		}`)
	})

	ctx := t.Context()
	body := &ExternalGroup{
		GroupID: Ptr(int64(123)),
	}
	externalGroup, _, err := client.Teams.UpdateConnectedExternalGroup(ctx, "o", "t", body)
	if err != nil {
		t.Errorf("Teams.UpdateConnectedExternalGroup returned error: %v", err)
	}

	want := &ExternalGroup{
		GroupID:   Ptr(int64(123)),
		GroupName: Ptr("Octocat admins"),
		UpdatedAt: &Timestamp{Time: referenceTime},
		Teams: []*ExternalGroupTeam{
			{
				TeamID:   Ptr(int64(1)),
				TeamName: Ptr("team-test"),
			},
			{
				TeamID:   Ptr(int64(2)),
				TeamName: Ptr("team-test2"),
			},
		},
		Members: []*ExternalGroupMember{
			{
				MemberID:    Ptr(int64(1)),
				MemberLogin: Ptr("mona-lisa_eocsaxrs"),
				MemberName:  Ptr("Mona Lisa"),
				MemberEmail: Ptr("mona_lisa@github.com"),
			},
			{
				MemberID:    Ptr(int64(2)),
				MemberLogin: Ptr("octo-lisa_eocsaxrs"),
				MemberName:  Ptr("Octo Lisa"),
				MemberEmail: Ptr("octo_lisa@github.com"),
			},
		},
	}
	if !cmp.Equal(externalGroup, want) {
		t.Errorf("Teams.GetExternalGroup returned %+v, want %+v", externalGroup, want)
	}

	const methodName = "UpdateConnectedExternalGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Teams.UpdateConnectedExternalGroup(ctx, "\n", "\n", body)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Teams.UpdateConnectedExternalGroup(ctx, "o", "t", body)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestTeamsService_UpdateConnectedExternalGroup_notFound(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/t/external-groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	body := &ExternalGroup{
		GroupID: Ptr(int64(123)),
	}
	eg, resp, err := client.Teams.UpdateConnectedExternalGroup(ctx, "o", "t", body)
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Teams.UpdateConnectedExternalGroup returned status %v, want %v", got, want)
	}
	if eg != nil {
		t.Errorf("Teams.UpdateConnectedExternalGroup returned %+v, want nil", eg)
	}
}

func TestTeamsService_RemoveConnectedExternalGroup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/t/external-groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Teams.RemoveConnectedExternalGroup(ctx, "o", "t")
	if err != nil {
		t.Errorf("Teams.RemoveConnectedExternalGroup returned error: %v", err)
	}

	const methodName = "RemoveConnectedExternalGroup"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Teams.RemoveConnectedExternalGroup(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Teams.RemoveConnectedExternalGroup(ctx, "o", "t")
	})
}

func TestTeamsService_RemoveConnectedExternalGroup_notFound(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/teams/t/external-groups", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	resp, err := client.Teams.RemoveConnectedExternalGroup(ctx, "o", "t")
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Teams.RemoveConnectedExternalGroup returned status %v, want %v", got, want)
	}
}
