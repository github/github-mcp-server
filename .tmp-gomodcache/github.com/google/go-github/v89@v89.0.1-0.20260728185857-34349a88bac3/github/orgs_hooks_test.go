// Copyright 2015 The go-github AUTHORS. All rights reserved.
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

func TestOrganizationsService_ListHooks(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/hooks", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}, {"id":2}]`)
	})

	opt := &ListOptions{Page: 2}

	ctx := t.Context()
	hooks, _, err := client.Organizations.ListHooks(ctx, "o", opt)
	if err != nil {
		t.Errorf("Organizations.ListHooks returned error: %v", err)
	}

	want := []*Hook{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if !cmp.Equal(hooks, want) {
		t.Errorf("Organizations.ListHooks returned %+v, want %+v", hooks, want)
	}

	const methodName = "ListHooks"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListHooks(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListHooks(ctx, "o", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListHooks_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Organizations.ListHooks(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestOrganizationsService_CreateHook(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &Hook{CreatedAt: &Timestamp{referenceTime}}

	mux.HandleFunc("/orgs/o/hooks", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		want := &createHookRequest{Name: "web"}
		testJSONBody(t, r, want)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	hook, _, err := client.Organizations.CreateHook(ctx, "o", input)
	if err != nil {
		t.Errorf("Organizations.CreateHook returned error: %v", err)
	}

	want := &Hook{ID: Ptr(int64(1))}
	if !cmp.Equal(hook, want) {
		t.Errorf("Organizations.CreateHook returned %+v, want %+v", hook, want)
	}

	const methodName = "CreateHook"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.CreateHook(ctx, "o", nil)
		return err
	})
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.CreateHook(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.CreateHook(ctx, "o", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_GetHook(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/hooks/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	hook, _, err := client.Organizations.GetHook(ctx, "o", 1)
	if err != nil {
		t.Errorf("Organizations.GetHook returned error: %v", err)
	}

	want := &Hook{ID: Ptr(int64(1))}
	if !cmp.Equal(hook, want) {
		t.Errorf("Organizations.GetHook returned %+v, want %+v", hook, want)
	}

	const methodName = "GetHook"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetHook(ctx, "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetHook(ctx, "o", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_GetHook_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Organizations.GetHook(ctx, "%", 1)
	testURLParseError(t, err)
}

func TestOrganizationsService_EditHook(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &Hook{}

	mux.HandleFunc("/orgs/o/hooks/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	hook, _, err := client.Organizations.EditHook(ctx, "o", 1, input)
	if err != nil {
		t.Errorf("Organizations.EditHook returned error: %v", err)
	}

	want := &Hook{ID: Ptr(int64(1))}
	if !cmp.Equal(hook, want) {
		t.Errorf("Organizations.EditHook returned %+v, want %+v", hook, want)
	}

	const methodName = "EditHook"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.EditHook(ctx, "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.EditHook(ctx, "o", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_EditHook_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Organizations.EditHook(ctx, "%", 1, nil)
	testURLParseError(t, err)
}

func TestOrganizationsService_PingHook(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/hooks/1/pings", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
	})

	ctx := t.Context()
	_, err := client.Organizations.PingHook(ctx, "o", 1)
	if err != nil {
		t.Errorf("Organizations.PingHook returned error: %v", err)
	}

	const methodName = "PingHook"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.PingHook(ctx, "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.PingHook(ctx, "o", 1)
	})
}

func TestOrganizationsService_DeleteHook(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/hooks/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Organizations.DeleteHook(ctx, "o", 1)
	if err != nil {
		t.Errorf("Organizations.DeleteHook returned error: %v", err)
	}

	const methodName = "DeleteHook"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.DeleteHook(ctx, "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.DeleteHook(ctx, "o", 1)
	})
}

func TestOrganizationsService_DeleteHook_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Organizations.DeleteHook(ctx, "%", 1)
	testURLParseError(t, err)
}
