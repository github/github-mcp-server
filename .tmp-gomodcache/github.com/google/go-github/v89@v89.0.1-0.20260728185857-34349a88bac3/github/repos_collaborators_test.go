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

func TestRepositoriesService_ListCollaborators(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/collaborators", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}, {"id":2}]`)
	})

	opt := &ListCollaboratorsOptions{
		ListOptions: ListOptions{Page: 2},
	}
	ctx := t.Context()
	users, _, err := client.Repositories.ListCollaborators(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Repositories.ListCollaborators returned error: %v", err)
	}

	want := []*User{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if !cmp.Equal(users, want) {
		t.Errorf("Repositories.ListCollaborators returned %+v, want %+v", users, want)
	}

	const methodName = "ListCollaborators"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListCollaborators(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListCollaborators(ctx, "o", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_ListCollaborators_withAffiliation(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/collaborators", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"affiliation": "all", "page": "2"})
		fmt.Fprint(w, `[{"id":1}, {"id":2}]`)
	})

	opt := &ListCollaboratorsOptions{
		ListOptions: ListOptions{Page: 2},
		Affiliation: "all",
	}
	ctx := t.Context()
	users, _, err := client.Repositories.ListCollaborators(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Repositories.ListCollaborators returned error: %v", err)
	}

	want := []*User{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if !cmp.Equal(users, want) {
		t.Errorf("Repositories.ListCollaborators returned %+v, want %+v", users, want)
	}

	const methodName = "ListCollaborators"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListCollaborators(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListCollaborators(ctx, "o", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_ListCollaborators_withPermission(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/collaborators", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"permission": "pull", "page": "2"})
		fmt.Fprint(w, `[{"id":1}, {"id":2}]`)
	})

	opt := &ListCollaboratorsOptions{
		ListOptions: ListOptions{Page: 2},
		Permission:  "pull",
	}
	ctx := t.Context()
	users, _, err := client.Repositories.ListCollaborators(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Repositories.ListCollaborators returned error: %v", err)
	}

	want := []*User{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if !cmp.Equal(users, want) {
		t.Errorf("Repositories.ListCollaborators returned %+v, want %+v", users, want)
	}

	const methodName = "ListCollaborators"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListCollaborators(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListCollaborators(ctx, "o", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_ListCollaborators_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.ListCollaborators(ctx, "%", "%", nil)
	testURLParseError(t, err)
}

func TestRepositoriesService_IsCollaborator_True(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/collaborators/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	isCollab, _, err := client.Repositories.IsCollaborator(ctx, "o", "r", "u")
	if err != nil {
		t.Errorf("Repositories.IsCollaborator returned error: %v", err)
	}

	if !isCollab {
		t.Error("Repositories.IsCollaborator returned false, want true")
	}

	const methodName = "IsCollaborator"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.IsCollaborator(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.IsCollaborator(ctx, "o", "r", "u")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want false", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_IsCollaborator_False(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/collaborators/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	isCollab, _, err := client.Repositories.IsCollaborator(ctx, "o", "r", "u")
	if err != nil {
		t.Errorf("Repositories.IsCollaborator returned error: %v", err)
	}

	if isCollab {
		t.Error("Repositories.IsCollaborator returned true, want false")
	}

	const methodName = "IsCollaborator"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.IsCollaborator(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.IsCollaborator(ctx, "o", "r", "u")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want false", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_IsCollaborator_invalidUser(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.IsCollaborator(ctx, "%", "%", "%")
	testURLParseError(t, err)
}

func TestRepositoryService_GetPermissionLevel(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/collaborators/u/permission", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"permission":"admin","user":{"login":"u"}}`)
	})

	ctx := t.Context()
	rpl, _, err := client.Repositories.GetPermissionLevel(ctx, "o", "r", "u")
	if err != nil {
		t.Errorf("Repositories.GetPermissionLevel returned error: %v", err)
	}

	want := &RepositoryPermissionLevel{
		Permission: Ptr("admin"),
		User: &User{
			Login: Ptr("u"),
		},
	}

	if !cmp.Equal(rpl, want) {
		t.Errorf("Repositories.GetPermissionLevel returned %+v, want %+v", rpl, want)
	}

	const methodName = "GetPermissionLevel"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetPermissionLevel(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetPermissionLevel(ctx, "o", "r", "u")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_AddCollaborator(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	opt := &RepositoryAddCollaboratorOptions{Permission: "admin"}
	mux.HandleFunc("/repos/o/r/collaborators/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, opt)
		w.WriteHeader(http.StatusOK)
		assertWrite(t, w, []byte(`{"permissions": "write","url": "https://api.github.com/user/repository_invitations/1296269","html_url": "https://github.com/octocat/Hello-World/invitations","id":1,"permissions":"write","repository":{"url":"s","name":"r","id":1},"invitee":{"login":"u"},"inviter":{"login":"o"}}`))
	})
	ctx := t.Context()
	collaboratorInvitation, _, err := client.Repositories.AddCollaborator(ctx, "o", "r", "u", opt)
	if err != nil {
		t.Errorf("Repositories.AddCollaborator returned error: %v", err)
	}
	want := &CollaboratorInvitation{
		ID: Ptr(int64(1)),
		Repo: &Repository{
			ID:   Ptr(int64(1)),
			URL:  Ptr("s"),
			Name: Ptr("r"),
		},
		Invitee: &User{
			Login: Ptr("u"),
		},
		Inviter: &User{
			Login: Ptr("o"),
		},
		Permissions: Ptr("write"),
		URL:         Ptr("https://api.github.com/user/repository_invitations/1296269"),
		HTMLURL:     Ptr("https://github.com/octocat/Hello-World/invitations"),
	}

	if !cmp.Equal(collaboratorInvitation, want) {
		t.Errorf("AddCollaborator returned %+v, want %+v", collaboratorInvitation, want)
	}

	const methodName = "AddCollaborator"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.AddCollaborator(ctx, "\n", "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.AddCollaborator(ctx, "o", "r", "u", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_AddCollaborator_invalidUser(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.AddCollaborator(ctx, "%", "%", "%", nil)
	testURLParseError(t, err)
}

func TestRepositoriesService_RemoveCollaborator(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/collaborators/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Repositories.RemoveCollaborator(ctx, "o", "r", "u")
	if err != nil {
		t.Errorf("Repositories.RemoveCollaborator returned error: %v", err)
	}

	const methodName = "RemoveCollaborator"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.RemoveCollaborator(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.RemoveCollaborator(ctx, "o", "r", "u")
	})
}

func TestRepositoriesService_RemoveCollaborator_invalidUser(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Repositories.RemoveCollaborator(ctx, "%", "%", "%")
	testURLParseError(t, err)
}
