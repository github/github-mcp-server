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

func TestRepositoriesService_ListPreReceiveHooks(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/pre-receive-hooks", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypePreReceiveHooksPreview)
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}, {"id":2}]`)
	})

	opt := &ListOptions{Page: 2}

	ctx := t.Context()
	hooks, _, err := client.Repositories.ListPreReceiveHooks(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Repositories.ListPreReceiveHooks returned error: %v", err)
	}

	want := []*PreReceiveHook{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if !cmp.Equal(hooks, want) {
		t.Errorf("Repositories.ListPreReceiveHooks returned %+v, want %+v", hooks, want)
	}

	const methodName = "ListPreReceiveHooks"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListPreReceiveHooks(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListPreReceiveHooks(ctx, "o", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_ListPreReceiveHooks_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.ListPreReceiveHooks(ctx, "%", "%", nil)
	testURLParseError(t, err)
}

func TestRepositoriesService_GetPreReceiveHook(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/pre-receive-hooks/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypePreReceiveHooksPreview)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	hook, _, err := client.Repositories.GetPreReceiveHook(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Repositories.GetPreReceiveHook returned error: %v", err)
	}

	want := &PreReceiveHook{ID: Ptr(int64(1))}
	if !cmp.Equal(hook, want) {
		t.Errorf("Repositories.GetPreReceiveHook returned %+v, want %+v", hook, want)
	}

	const methodName = "GetPreReceiveHook"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetPreReceiveHook(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetPreReceiveHook(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetPreReceiveHook_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.GetPreReceiveHook(ctx, "%", "%", 1)
	testURLParseError(t, err)
}

func TestRepositoriesService_UpdatePreReceiveHook(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &PreReceiveHook{}

	mux.HandleFunc("/repos/o/r/pre-receive-hooks/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	hook, _, err := client.Repositories.UpdatePreReceiveHook(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("Repositories.UpdatePreReceiveHook returned error: %v", err)
	}

	want := &PreReceiveHook{ID: Ptr(int64(1))}
	if !cmp.Equal(hook, want) {
		t.Errorf("Repositories.UpdatePreReceiveHook returned %+v, want %+v", hook, want)
	}

	const methodName = "UpdatePreReceiveHook"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.UpdatePreReceiveHook(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.UpdatePreReceiveHook(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_PreReceiveHook_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.UpdatePreReceiveHook(ctx, "%", "%", 1, nil)
	testURLParseError(t, err)
}

func TestRepositoriesService_DeletePreReceiveHook(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/pre-receive-hooks/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Repositories.DeletePreReceiveHook(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Repositories.DeletePreReceiveHook returned error: %v", err)
	}

	const methodName = "DeletePreReceiveHook"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.DeletePreReceiveHook(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.DeletePreReceiveHook(ctx, "o", "r", 1)
	})
}

func TestRepositoriesService_DeletePreReceiveHook_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Repositories.DeletePreReceiveHook(ctx, "%", "%", 1)
	testURLParseError(t, err)
}
