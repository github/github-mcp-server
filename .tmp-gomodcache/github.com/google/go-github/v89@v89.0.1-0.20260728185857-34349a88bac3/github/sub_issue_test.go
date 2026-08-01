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

func TestSubIssuesService_Add(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &SubIssueRequest{SubIssueID: 42}

	mux.HandleFunc("/repos/o/r/issues/1/sub_issues", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":42, "number":1}`)
	})

	ctx := t.Context()
	got, _, err := client.SubIssue.Add(ctx, "o", "r", 1, *input)
	if err != nil {
		t.Errorf("SubIssues.Add returned error: %v", err)
	}

	want := &SubIssue{Number: Ptr(1), ID: Ptr(int64(42))}
	if !cmp.Equal(got, want) {
		t.Errorf("SubIssues.Add = %+v, want %+v", got, want)
	}

	const methodName = "Add"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.SubIssue.Add(ctx, "o", "r", 1, *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestSubIssuesService_ListByIssue(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/sub_issues", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page":     "2",
			"per_page": "50",
		})

		fmt.Fprint(w, `[{"id":1},{"id":2}]`)
	})

	ctx := t.Context()
	opt := &ListOptions{
		Page:    2,
		PerPage: 50,
	}
	issues, _, err := client.SubIssue.ListByIssue(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("SubIssues.ListByIssue returned error: %v", err)
	}

	want := []*SubIssue{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if !cmp.Equal(issues, want) {
		t.Errorf("SubIssues.ListByIssue = %+v, want %+v", issues, want)
	}

	const methodName = "ListByIssue"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.SubIssue.ListByIssue(ctx, "\n", "\n", 1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.SubIssue.ListByIssue(ctx, "o", "r", 1, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestSubIssuesService_Remove(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &SubIssueRequest{SubIssueID: 42}

	mux.HandleFunc("/repos/o/r/issues/1/sub_issue", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":42, "number":1}`)
	})

	ctx := t.Context()
	got, _, err := client.SubIssue.Remove(ctx, "o", "r", 1, *input)
	if err != nil {
		t.Errorf("SubIssues.Remove returned error: %v", err)
	}

	want := &SubIssue{ID: Ptr(int64(42)), Number: Ptr(1)}
	if !cmp.Equal(got, want) {
		t.Errorf("SubIssues.Remove = %+v, want %+v", got, want)
	}

	const methodName = "Remove"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.SubIssue.Remove(ctx, "o", "r", 1, *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestSubIssuesService_Reprioritize(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &SubIssueRequest{SubIssueID: 42, AfterID: Ptr(int64(5))}

	mux.HandleFunc("/repos/o/r/issues/1/sub_issues/priority", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":42, "number":1}`)
	})

	ctx := t.Context()
	got, _, err := client.SubIssue.Reprioritize(ctx, "o", "r", 1, *input)
	if err != nil {
		t.Errorf("SubIssues.Reprioritize returned error: %v", err)
	}

	want := &SubIssue{ID: Ptr(int64(42)), Number: Ptr(1)}
	if !cmp.Equal(got, want) {
		t.Errorf("SubIssues.Reprioritize = %+v, want %+v", got, want)
	}

	const methodName = "Reprioritize"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.SubIssue.Reprioritize(ctx, "o", "r", 1, *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestSubIssuesService_GetParentIssue(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/42/parent", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1, "number":1}`)
	})

	ctx := t.Context()
	got, _, err := client.SubIssue.GetParentIssue(ctx, "o", "r", 42)
	if err != nil {
		t.Errorf("SubIssues.GetParentIssue returned error: %v", err)
	}

	want := &Issue{Number: Ptr(1), ID: Ptr(int64(1))}
	if !cmp.Equal(got, want) {
		t.Errorf("SubIssues.GetParentIssue = %+v, want %+v", got, want)
	}

	const methodName = "GetParentIssue"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.SubIssue.GetParentIssue(ctx, "o", "r", 42)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
