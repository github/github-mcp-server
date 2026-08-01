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

func TestRepositoriesService_ListTagProtection(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/tags/protection", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")

		fmt.Fprint(w, `[{"id":1, "pattern":"tag1"},{"id":2, "pattern":"tag2"}]`)
	})

	ctx := t.Context()
	tagProtections, _, err := client.Repositories.ListTagProtection(ctx, "o", "r")
	if err != nil {
		t.Errorf("Repositories.ListTagProtection returned error: %v", err)
	}

	want := []*TagProtection{{ID: Ptr(int64(1)), Pattern: Ptr("tag1")}, {ID: Ptr(int64(2)), Pattern: Ptr("tag2")}}
	if !cmp.Equal(tagProtections, want) {
		t.Errorf("Repositories.ListTagProtection returned %+v, want %+v", tagProtections, want)
	}

	const methodName = "ListTagProtection"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListTagProtection(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListTagProtection(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_ListTagProtection_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.ListTagProtection(ctx, "%", "r")
	testURLParseError(t, err)
}

func TestRepositoriesService_CreateTagProtection(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	pattern := "tag*"

	mux.HandleFunc("/repos/o/r/tags/protection", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		want := &tagProtectionRequest{Pattern: "tag*"}
		testJSONBody(t, r, want)
		fmt.Fprint(w, `{"id":1,"pattern":"tag*"}`)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.CreateTagProtection(ctx, "o", "r", pattern)
	if err != nil {
		t.Errorf("Repositories.CreateTagProtection returned error: %v", err)
	}

	want := &TagProtection{ID: Ptr(int64(1)), Pattern: Ptr("tag*")}
	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.CreateTagProtection returned %+v, want %+v", got, want)
	}

	const methodName = "CreateTagProtection"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.CreateTagProtection(ctx, "\n", "\n", pattern)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.CreateTagProtection(ctx, "o", "r", pattern)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_DeleteTagProtection(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/tags/protection/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Repositories.DeleteTagProtection(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Repositories.DeleteTagProtection returned error: %v", err)
	}

	const methodName = "DeleteTagProtection"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.DeleteTagProtection(ctx, "\n", "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.DeleteTagProtection(ctx, "o", "r", 1)
	})
}
