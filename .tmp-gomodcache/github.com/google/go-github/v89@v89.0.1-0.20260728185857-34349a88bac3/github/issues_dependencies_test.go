// Copyright 2026 The go-github AUTHORS. All rights reserved.
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

func TestIssuesService_ListBlockedBy(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/dependencies/blocked_by", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"number":1347,"title":"Found a bug"}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	issues, _, err := client.Issues.ListBlockedBy(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("Issues.ListBlockedBy returned error: %v", err)
	}

	want := []*Issue{{Number: Ptr(1347), Title: Ptr("Found a bug")}}
	if !cmp.Equal(issues, want) {
		t.Errorf("Issues.ListBlockedBy returned %+v, want %+v", issues, want)
	}

	const methodName = "ListBlockedBy"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ListBlockedBy(ctx, "\n", "\n", -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListBlockedBy(ctx, "o", "r", 1, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_ListBlockedBy_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.ListBlockedBy(ctx, "%", "%", 1, nil)
	testURLParseError(t, err)
}

func TestIssuesService_AddBlockedBy(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := IssueDependencyRequest{IssueID: int64(42)}

	mux.HandleFunc("/repos/o/r/issues/1/dependencies/blocked_by", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)

		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"number":42,"title":"Dependency issue"}`)
	})

	ctx := t.Context()
	issue, _, err := client.Issues.AddBlockedBy(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("Issues.AddBlockedBy returned error: %v", err)
	}

	want := &Issue{Number: Ptr(42), Title: Ptr("Dependency issue")}
	if !cmp.Equal(issue, want) {
		t.Errorf("Issues.AddBlockedBy returned %+v, want %+v", issue, want)
	}

	const methodName = "AddBlockedBy"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.AddBlockedBy(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.AddBlockedBy(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_AddBlockedBy_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.AddBlockedBy(ctx, "%", "%", 1, IssueDependencyRequest{})
	testURLParseError(t, err)
}

func TestIssuesService_RemoveBlockedBy(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/dependencies/blocked_by/42", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		fmt.Fprint(w, `{"number":1,"title":"Original issue"}`)
	})

	ctx := t.Context()
	issue, _, err := client.Issues.RemoveBlockedBy(ctx, "o", "r", 1, 42)
	if err != nil {
		t.Errorf("Issues.RemoveBlockedBy returned error: %v", err)
	}

	want := &Issue{Number: Ptr(1), Title: Ptr("Original issue")}
	if !cmp.Equal(issue, want) {
		t.Errorf("Issues.RemoveBlockedBy returned %+v, want %+v", issue, want)
	}

	const methodName = "RemoveBlockedBy"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.RemoveBlockedBy(ctx, "\n", "\n", -1, 42)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.RemoveBlockedBy(ctx, "o", "r", 1, 42)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_RemoveBlockedBy_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.RemoveBlockedBy(ctx, "%", "%", 1, 42)
	testURLParseError(t, err)
}

func TestIssuesService_ListBlocking(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/dependencies/blocking", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"number":1348,"title":"Blocked issue"}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	issues, _, err := client.Issues.ListBlocking(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("Issues.ListBlocking returned error: %v", err)
	}

	want := []*Issue{{Number: Ptr(1348), Title: Ptr("Blocked issue")}}
	if !cmp.Equal(issues, want) {
		t.Errorf("Issues.ListBlocking returned %+v, want %+v", issues, want)
	}

	const methodName = "ListBlocking"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ListBlocking(ctx, "\n", "\n", -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListBlocking(ctx, "o", "r", 1, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_ListBlocking_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.ListBlocking(ctx, "%", "%", 1, nil)
	testURLParseError(t, err)
}
