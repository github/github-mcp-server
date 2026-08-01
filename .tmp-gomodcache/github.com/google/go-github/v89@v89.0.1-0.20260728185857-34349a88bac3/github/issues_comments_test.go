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

func TestIssuesService_ListComments_allIssues(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/comments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		testFormValues(t, r, values{
			"sort":      "updated",
			"direction": "desc",
			"since":     referenceTimeRaw,
			"page":      "2",
		})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &IssueListCommentsOptions{
		Sort:        Ptr("updated"),
		Direction:   Ptr("desc"),
		Since:       &referenceTime,
		ListOptions: ListOptions{Page: 2},
	}
	ctx := t.Context()
	comments, _, err := client.Issues.ListComments(ctx, "o", "r", 0, opt)
	if err != nil {
		t.Errorf("Issues.ListComments returned error: %v", err)
	}

	want := []*IssueComment{{ID: Ptr(int64(1))}}
	if !cmp.Equal(comments, want) {
		t.Errorf("Issues.ListComments returned %+v, want %+v", comments, want)
	}

	const methodName = "ListComments"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ListComments(ctx, "\n", "\n", -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListComments(ctx, "o", "r", 0, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_ListComments_specificIssue(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		fmt.Fprint(w, `[{"id":1}]`)
	})

	ctx := t.Context()
	comments, _, err := client.Issues.ListComments(ctx, "o", "r", 1, nil)
	if err != nil {
		t.Errorf("Issues.ListComments returned error: %v", err)
	}

	want := []*IssueComment{{ID: Ptr(int64(1))}}
	if !cmp.Equal(comments, want) {
		t.Errorf("Issues.ListComments returned %+v, want %+v", comments, want)
	}

	const methodName = "ListComments"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ListComments(ctx, "\n", "\n", -1, nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListComments(ctx, "o", "r", 1, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_ListComments_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.ListComments(ctx, "%", "r", 1, nil)
	testURLParseError(t, err)
}

func TestIssuesService_GetComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/comments/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	comment, _, err := client.Issues.GetComment(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Issues.GetComment returned error: %v", err)
	}

	want := &IssueComment{ID: Ptr(int64(1))}
	if !cmp.Equal(comment, want) {
		t.Errorf("Issues.GetComment returned %+v, want %+v", comment, want)
	}

	const methodName = "GetComment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.GetComment(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.GetComment(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_GetComment_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.GetComment(ctx, "%", "r", 1)
	testURLParseError(t, err)
}

func TestIssuesService_CreateComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &IssueComment{Body: Ptr("b")}

	mux.HandleFunc("/repos/o/r/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	comment, _, err := client.Issues.CreateComment(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("Issues.CreateComment returned error: %v", err)
	}

	want := &IssueComment{ID: Ptr(int64(1))}
	if !cmp.Equal(comment, want) {
		t.Errorf("Issues.CreateComment returned %+v, want %+v", comment, want)
	}

	const methodName = "CreateComment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.CreateComment(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.CreateComment(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_CreateComment_invalidOrg(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.CreateComment(ctx, "%", "r", 1, nil)
	testURLParseError(t, err)
}

func TestIssuesService_EditComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &IssueComment{Body: Ptr("b")}

	mux.HandleFunc("/repos/o/r/issues/comments/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	comment, _, err := client.Issues.EditComment(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("Issues.EditComment returned error: %v", err)
	}

	want := &IssueComment{ID: Ptr(int64(1))}
	if !cmp.Equal(comment, want) {
		t.Errorf("Issues.EditComment returned %+v, want %+v", comment, want)
	}

	const methodName = "EditComment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.EditComment(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.EditComment(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_EditComment_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.EditComment(ctx, "%", "r", 1, nil)
	testURLParseError(t, err)
}

func TestIssuesService_DeleteComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/comments/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Issues.DeleteComment(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Issues.DeleteComments returned error: %v", err)
	}

	const methodName = "DeleteComment"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Issues.DeleteComment(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Issues.DeleteComment(ctx, "o", "r", 1)
	})
}

func TestIssuesService_DeleteComment_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Issues.DeleteComment(ctx, "%", "r", 1)
	testURLParseError(t, err)
}
