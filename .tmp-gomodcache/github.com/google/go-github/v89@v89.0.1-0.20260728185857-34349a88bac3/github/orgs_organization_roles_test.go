// Copyright 2022 The go-github AUTHORS. All rights reserved.
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

func TestOrganizationsService_ListRoles(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/organization-roles", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"total_count": 1, "roles": [
			{
				"id": 1,
				"name": "Auditor",
				"permissions": ["read_audit_logs"],
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
				"updated_at": `+refTimeStr(1136178001)+`,
				"source": "Organization",
				"base_role": "admin"
			}
			]
		}`)
	})

	ctx := t.Context()
	apps, _, err := client.Organizations.ListRoles(ctx, "o")
	if err != nil {
		t.Errorf("Organizations.ListRoles returned error: %v", err)
	}

	want := &OrganizationCustomRoles{
		TotalCount: Ptr(1),
		CustomRepoRoles: []*CustomOrgRole{
			{
				ID:          Ptr(int64(1)),
				Name:        Ptr("Auditor"),
				Permissions: []string{"read_audit_logs"},
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
				Source:    Ptr("Organization"),
				BaseRole:  Ptr("admin"),
			},
		},
	}
	if !cmp.Equal(apps, want) {
		t.Errorf("Organizations.ListRoles returned %+v, want %+v", apps, want)
	}

	const methodName = "ListRoles"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListRoles(ctx, "\no")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListRoles(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_GetOrgRole(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// Test built-in org role
	mux.HandleFunc("/orgs/o/organization-roles/8132", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id": 8132,
			"name": "all_repo_read",
			"description": "Grants read access to all repositories in the organization.",
			"permissions": [],
			"created_at": `+refTimeStr(1136178000)+`,
			"updated_at": `+refTimeStr(1136178001)+`,
			"source": "Predefined",
			"base_role": "read"
		}`)
	})

	ctx := t.Context()

	gotBuiltInRole, _, err := client.Organizations.GetOrgRole(ctx, "o", 8132)
	if err != nil {
		t.Errorf("Organizations.GetOrgRole returned error: %v", err)
	}

	wantBuiltInRole := &CustomOrgRole{
		ID:          Ptr(int64(8132)),
		Name:        Ptr("all_repo_read"),
		Description: Ptr("Grants read access to all repositories in the organization."),
		Permissions: []string{},
		CreatedAt:   refTimestamp(1136178000),
		UpdatedAt:   refTimestamp(1136178001),
		Source:      Ptr("Predefined"),
		BaseRole:    Ptr("read"),
	}

	if !cmp.Equal(gotBuiltInRole, wantBuiltInRole) {
		t.Errorf("Organizations.GetOrgRole returned %+v, want %+v", gotBuiltInRole, wantBuiltInRole)
	}

	// Test custom org role
	mux.HandleFunc("/orgs/o/organization-roles/123456", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id": 123456,
			"name": "test-role",
			"description": "test-role",
			"permissions": [
				"read_organization_custom_org_role",
				"read_organization_custom_repo_role",
				"write_organization_custom_org_role"
			],
			"created_at": `+refTimeStr(1136178000)+`,
			"updated_at": `+refTimeStr(1136178001)+`,
			"source": "Organization",
			"base_role": null
			}`)
	})

	gotCustomRole, _, err := client.Organizations.GetOrgRole(ctx, "o", 123456)
	if err != nil {
		t.Errorf("Organizations.GetOrgRole returned error: %v", err)
	}

	wantCustomRole := &CustomOrgRole{
		ID:          Ptr(int64(123456)),
		Name:        Ptr("test-role"),
		Description: Ptr("test-role"),
		Permissions: []string{
			"read_organization_custom_org_role",
			"read_organization_custom_repo_role",
			"write_organization_custom_org_role",
		},
		CreatedAt: refTimestamp(1136178000),
		UpdatedAt: refTimestamp(1136178001),
		Source:    Ptr("Organization"),
		BaseRole:  nil,
	}

	if !cmp.Equal(gotCustomRole, wantCustomRole) {
		t.Errorf("Organizations.GetOrgRole returned %+v, want %+v", gotCustomRole, wantCustomRole)
	}

	const methodName = "GetOrgRole"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetOrgRole(ctx, "\no", -8132)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetOrgRole(ctx, "o", 8132)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_CreateCustomOrgRole(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/organization-roles", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"id":8030,"name":"Reader","description":"A role for reading custom org roles","permissions":["read_organization_custom_org_role"]}`)
	})

	ctx := t.Context()

	opts := CreateCustomOrgRoleRequest{
		Name:        "Reader",
		Permissions: []string{"read_organization_custom_org_role"},
		Description: Ptr("A role for reading custom org roles"),
	}
	gotRoles, _, err := client.Organizations.CreateCustomOrgRole(ctx, "o", opts)
	if err != nil {
		t.Errorf("Organizations.CreateCustomOrgRole returned error: %v", err)
	}

	want := &CustomOrgRole{ID: Ptr(int64(8030)), Name: Ptr("Reader"), Permissions: []string{"read_organization_custom_org_role"}, Description: Ptr("A role for reading custom org roles")}

	if !cmp.Equal(gotRoles, want) {
		t.Errorf("Organizations.CreateCustomOrgRole returned %+v, want %+v", gotRoles, want)
	}

	const methodName = "CreateCustomOrgRole"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.CreateCustomOrgRole(ctx, "\no", CreateCustomOrgRoleRequest{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.CreateCustomOrgRole(ctx, "o", CreateCustomOrgRoleRequest{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_UpdateCustomOrgRole(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/organization-roles/8030", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{"id":8030,"name":"Updated Name","description":"Updated Description","permissions":["read_organization_custom_org_role"]}`)
	})

	ctx := t.Context()

	opts := UpdateCustomOrgRoleRequest{
		Name:        Ptr("Updated Name"),
		Description: Ptr("Updated Description"),
	}
	gotRoles, _, err := client.Organizations.UpdateCustomOrgRole(ctx, "o", 8030, opts)
	if err != nil {
		t.Errorf("Organizations.UpdateCustomOrgRole returned error: %v", err)
	}

	want := &CustomOrgRole{ID: Ptr(int64(8030)), Name: Ptr("Updated Name"), Permissions: []string{"read_organization_custom_org_role"}, Description: Ptr("Updated Description")}

	if !cmp.Equal(gotRoles, want) {
		t.Errorf("Organizations.UpdateCustomOrgRole returned %+v, want %+v", gotRoles, want)
	}

	const methodName = "UpdateCustomOrgRole"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.UpdateCustomOrgRole(ctx, "\no", 8030, UpdateCustomOrgRoleRequest{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.UpdateCustomOrgRole(ctx, "o", 8030, UpdateCustomOrgRoleRequest{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_DeleteCustomOrgRole(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/organization-roles/8030", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()

	resp, err := client.Organizations.DeleteCustomOrgRole(ctx, "o", 8030)
	if err != nil {
		t.Errorf("Organizations.DeleteCustomOrgRole returned error: %v", err)
	}

	if !cmp.Equal(resp.StatusCode, 204) {
		t.Errorf("Organizations.DeleteCustomOrgRole returned  status code %+v, want %+v", resp.StatusCode, "204")
	}

	const methodName = "DeleteCustomOrgRole"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.DeleteCustomOrgRole(ctx, "\no", 8030)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.DeleteCustomOrgRole(ctx, "o", 8030)
	})
}

func TestOrganizationsService_AssignOrgRoleToTeam(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/organization-roles/teams/t/8030", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Organizations.AssignOrgRoleToTeam(ctx, "o", "t", 8030)
	if err != nil {
		t.Errorf("Organization.AssignOrgRoleToTeam return error: %v", err)
	}
	if !cmp.Equal(resp.StatusCode, http.StatusNoContent) {
		t.Errorf("Organizations.AssignOrgRoleToTeam returned status code %+v, want %+v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "AssignOrgRoleToTeam"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.AssignOrgRoleToTeam(ctx, "\no", "\nt", -8030)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.AssignOrgRoleToTeam(ctx, "o", "t", 8030)
	})
}

func TestOrganizationsService_RemoveOrgRoleFromTeam(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/organization-roles/teams/t/8030", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Organizations.RemoveOrgRoleFromTeam(ctx, "o", "t", 8030)
	if err != nil {
		t.Errorf("Organization.RemoveOrgRoleFromTeam return error: %v", err)
	}
	if !cmp.Equal(resp.StatusCode, http.StatusNoContent) {
		t.Errorf("Organizations.RemoveOrgRoleFromTeam returned status code %+v, want %+v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "RemoveOrgRoleFromTeam"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.RemoveOrgRoleFromTeam(ctx, "\no", "\nt", -8030)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.RemoveOrgRoleFromTeam(ctx, "o", "t", 8030)
	})
}

func TestOrganizationsService_AssignOrgRoleToUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/organization-roles/users/t/8030", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Organizations.AssignOrgRoleToUser(ctx, "o", "t", 8030)
	if err != nil {
		t.Errorf("Organization.AssignOrgRoleToUser return error: %v", err)
	}
	if !cmp.Equal(resp.StatusCode, http.StatusNoContent) {
		t.Errorf("Organizations.AssignOrgRoleToUser returned status code %+v, want %+v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "AssignOrgRoleToUser"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.AssignOrgRoleToUser(ctx, "\no", "\nt", -8030)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.AssignOrgRoleToUser(ctx, "o", "t", 8030)
	})
}

func TestOrganizationsService_RemoveOrgRoleFromUser(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/organization-roles/users/t/8030", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Organizations.RemoveOrgRoleFromUser(ctx, "o", "t", 8030)
	if err != nil {
		t.Errorf("Organization.RemoveOrgRoleFromUser return error: %v", err)
	}
	if !cmp.Equal(resp.StatusCode, http.StatusNoContent) {
		t.Errorf("Organizations.RemoveOrgRoleFromUser returned status code %+v, want %+v", resp.StatusCode, http.StatusNoContent)
	}

	const methodName = "RemoveOrgRoleFromUser"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.RemoveOrgRoleFromUser(ctx, "\no", "\nt", -8030)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.RemoveOrgRoleFromUser(ctx, "o", "t", 8030)
	})
}

func TestOrganizationsService_ListTeamsAssignedToOrgRole(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/organization-roles/1729/teams", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":1}]`)
	})
	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	apps, _, err := client.Organizations.ListTeamsAssignedToOrgRole(ctx, "o", 1729, opt)
	if err != nil {
		t.Errorf("Organizations.ListTeamsAssignedToOrgRole returned error: %v", err)
	}

	want := []*Team{{ID: Ptr(int64(1))}}
	if !cmp.Equal(apps, want) {
		t.Errorf("Organizations.ListTeamsAssignedToOrgRole returned %+v, want %+v", apps, want)
	}

	const methodName = "ListTeamsAssignedToOrgRole"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListTeamsAssignedToOrgRole(ctx, "\no", 1729, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListTeamsAssignedToOrgRole(ctx, "o", 1729, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListUsersAssignedToOrgRole(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/organization-roles/1729/users", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"id":1}]`)
	})
	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	apps, _, err := client.Organizations.ListUsersAssignedToOrgRole(ctx, "o", 1729, opt)
	if err != nil {
		t.Errorf("Organizations.ListUsersAssignedToOrgRole returned error: %v", err)
	}

	want := []*User{{ID: Ptr(int64(1))}}
	if !cmp.Equal(apps, want) {
		t.Errorf("Organizations.ListUsersAssignedToOrgRole returned %+v, want %+v", apps, want)
	}

	const methodName = "ListUsersAssignedToOrgRole"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListUsersAssignedToOrgRole(ctx, "\no", 1729, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListUsersAssignedToOrgRole(ctx, "o", 1729, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListFineGrainedPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/organization-fine-grained-permissions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[{"name":"p1", "description":"d1"}]`)
	})

	ctx := t.Context()
	permissions, _, err := client.Organizations.ListFineGrainedPermissions(ctx, "o")
	if err != nil {
		t.Errorf("Organizations.ListFineGrainedPermissions returned error: %v", err)
	}

	want := []*OrganizationFineGrainedPermission{{Name: "p1", Description: "d1"}}
	if !cmp.Equal(permissions, want) {
		t.Errorf("Organizations.ListFineGrainedPermissions returned %+v, want %+v", permissions, want)
	}

	const methodName = "ListFineGrainedPermissions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListFineGrainedPermissions(ctx, "\no")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListFineGrainedPermissions(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
