// Copyright 2024 The go-github AUTHORS. All rights reserved.
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

func TestOrganizationsService_ListCustomRepoRoles(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/custom-repository-roles", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"total_count": 1, "custom_roles": [
			{
				"id": 1,
				"name": "Developer",
				"base_role": "write",
				"permissions": ["delete_alerts_code_scanning"],
				"organization": {
					"login": "l",
					"id": 1,
					"node_id": "n",
					"avatar_url": "a",
					"html_url": "h",
					"name": "n",
					"company": "c",
					"blog": "b",
					"location": "l",
					"email": "e"
				},
				"created_at": `+refTimeStr(1136178000)+`,
				"updated_at": `+refTimeStr(1136178001)+`
			}
		  ]
		}`)
	})

	ctx := t.Context()
	apps, _, err := client.Organizations.ListCustomRepoRoles(ctx, "o")
	if err != nil {
		t.Errorf("Organizations.ListCustomRepoRoles returned error: %v", err)
	}

	want := &OrganizationCustomRepoRoles{
		TotalCount: Ptr(1),
		CustomRepoRoles: []*CustomRepoRoles{
			{
				ID:          Ptr(int64(1)),
				Name:        Ptr("Developer"),
				BaseRole:    Ptr("write"),
				Permissions: []string{"delete_alerts_code_scanning"},
				Org: &Organization{
					Login:     Ptr("l"),
					ID:        Ptr(int64(1)),
					NodeID:    Ptr("n"),
					AvatarURL: Ptr("a"),
					HTMLURL:   Ptr("h"),
					Name:      Ptr("n"),
					Company:   Ptr("c"),
					Blog:      Ptr("b"),
					Location:  Ptr("l"),
					Email:     Ptr("e"),
				},
				CreatedAt: refTimestamp(1136178000),
				UpdatedAt: refTimestamp(1136178001),
			},
		},
	}
	if !cmp.Equal(apps, want) {
		t.Errorf("Organizations.ListCustomRepoRoles returned %+v, want %+v", apps, want)
	}

	const methodName = "ListCustomRepoRoles"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListCustomRepoRoles(ctx, "\no")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListCustomRepoRoles(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_GetCustomRepoRole(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/custom-repository-roles/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
				"id": 1,
				"name": "Developer",
				"base_role": "write",
				"permissions": ["delete_alerts_code_scanning"],
				"organization": {
					"login": "l",
					"id": 1,
					"node_id": "n",
					"avatar_url": "a",
					"html_url": "h",
					"name": "n",
					"company": "c",
					"blog": "b",
					"location": "l",
					"email": "e"
				},
				"created_at": `+refTimeStr(1136178000)+`,
				"updated_at": `+refTimeStr(1136178001)+`
			}`)
	})

	ctx := t.Context()
	role, _, err := client.Organizations.GetCustomRepoRole(ctx, "o", 1)
	if err != nil {
		t.Errorf("Organizations.GetCustomRepoRole returned error: %v", err)
	}

	want := &CustomRepoRoles{
		ID:          Ptr(int64(1)),
		Name:        Ptr("Developer"),
		BaseRole:    Ptr("write"),
		Permissions: []string{"delete_alerts_code_scanning"},
		Org: &Organization{
			Login:     Ptr("l"),
			ID:        Ptr(int64(1)),
			NodeID:    Ptr("n"),
			AvatarURL: Ptr("a"),
			HTMLURL:   Ptr("h"),
			Name:      Ptr("n"),
			Company:   Ptr("c"),
			Blog:      Ptr("b"),
			Location:  Ptr("l"),
			Email:     Ptr("e"),
		},
		CreatedAt: refTimestamp(1136178000),
		UpdatedAt: refTimestamp(1136178001),
	}
	if !cmp.Equal(role, want) {
		t.Errorf("Organizations.GetCustomRepoRole returned %+v, want %+v", role, want)
	}

	const methodName = "GetCustomRepoRole"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetCustomRepoRole(ctx, "\no", 1)
		return err
	})

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetCustomRepoRole(ctx, "o", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetCustomRepoRole(ctx, "o", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_CreateCustomRepoRole(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/custom-repository-roles", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"id":8030,"name":"Labeler","description":"A role for issue and PR labelers","base_role":"read","permissions":["add_label"]}`)
	})

	ctx := t.Context()

	body := CreateCustomRepoRoleRequest{
		Name:        "Labeler",
		Description: Ptr("A role for issue and PR labelers"),
		BaseRole:    "read",
		Permissions: []string{"add_label"},
	}
	apps, _, err := client.Organizations.CreateCustomRepoRole(ctx, "o", body)
	if err != nil {
		t.Errorf("Organizations.CreateCustomRepoRole returned error: %v", err)
	}

	want := &CustomRepoRoles{ID: Ptr(int64(8030)), Name: Ptr("Labeler"), BaseRole: Ptr("read"), Permissions: []string{"add_label"}, Description: Ptr("A role for issue and PR labelers")}

	if !cmp.Equal(apps, want) {
		t.Errorf("Organizations.CreateCustomRepoRole returned %+v, want %+v", apps, want)
	}

	const methodName = "CreateCustomRepoRole"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.CreateCustomRepoRole(ctx, "\no", CreateCustomRepoRoleRequest{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.CreateCustomRepoRole(ctx, "o", CreateCustomRepoRoleRequest{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_UpdateCustomRepoRole(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/custom-repository-roles/8030", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{"id":8030,"name":"Updated Name","description":"Updated Description","base_role":"read","permissions":["add_label"]}`)
	})

	ctx := t.Context()

	body := UpdateCustomRepoRoleRequest{
		Name:        Ptr("Updated Name"),
		Description: Ptr("Updated Description"),
	}
	apps, _, err := client.Organizations.UpdateCustomRepoRole(ctx, "o", 8030, body)
	if err != nil {
		t.Errorf("Organizations.UpdateCustomRepoRole returned error: %v", err)
	}

	want := &CustomRepoRoles{ID: Ptr(int64(8030)), Name: Ptr("Updated Name"), BaseRole: Ptr("read"), Permissions: []string{"add_label"}, Description: Ptr("Updated Description")}

	if !cmp.Equal(apps, want) {
		t.Errorf("Organizations.UpdateCustomRepoRole returned %+v, want %+v", apps, want)
	}

	const methodName = "UpdateCustomRepoRole"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.UpdateCustomRepoRole(ctx, "\no", 8030, UpdateCustomRepoRoleRequest{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.UpdateCustomRepoRole(ctx, "o", 8030, UpdateCustomRepoRoleRequest{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_DeleteCustomRepoRole(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/custom-repository-roles/8030", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()

	resp, err := client.Organizations.DeleteCustomRepoRole(ctx, "o", 8030)
	if err != nil {
		t.Errorf("Organizations.DeleteCustomRepoRole returned error: %v", err)
	}

	if !cmp.Equal(resp.StatusCode, 204) {
		t.Errorf("Organizations.DeleteCustomRepoRole returned  status code %+v, want %+v", resp.StatusCode, "204")
	}

	const methodName = "DeleteCustomRepoRole"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.DeleteCustomRepoRole(ctx, "\no", 8030)
		return err
	})
}

func TestOrganizationsService_ListRepositoryFineGrainedPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/repository-fine-grained-permissions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[
			{
				"name": "add_assignee",
				"description": "Assign or remove a user"
			},
			{
				"name": "add_label",
				"description": "Add or remove a label"
			}
		]`)
	})

	ctx := t.Context()
	perms, _, err := client.Organizations.ListRepositoryFineGrainedPermissions(ctx, "o")
	if err != nil {
		t.Errorf("Organizations.ListRepositoryFineGrainedPermissions returned error: %v", err)
	}

	want := []*RepoFineGrainedPermission{
		{
			Name:        "add_assignee",
			Description: "Assign or remove a user",
		},
		{
			Name:        "add_label",
			Description: "Add or remove a label",
		},
	}
	if !cmp.Equal(perms, want) {
		t.Errorf("Organizations.ListRepositoryFineGrainedPermissions returned %+v, want %+v", perms, want)
	}

	const methodName = "ListRepositoryFineGrainedPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListRepositoryFineGrainedPermissions(ctx, "\no")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListRepositoryFineGrainedPermissions(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
