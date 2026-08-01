// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPullRequestsService_ListComments_allPulls(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	wantAcceptHeaders := []string{mediaTypeReactionsPreview, mediaTypeMultiLineCommentsPreview}
	mux.HandleFunc("/repos/o/r/pulls/comments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", strings.Join(wantAcceptHeaders, ", "))
		testFormValues(t, r, values{
			"sort":      "updated",
			"direction": "desc",
			"since":     referenceTimeRaw,
			"page":      "2",
		})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &PullRequestListCommentsOptions{
		Sort:        "updated",
		Direction:   "desc",
		Since:       referenceTime,
		ListOptions: ListOptions{Page: 2},
	}
	ctx := t.Context()
	pulls, _, err := client.PullRequests.ListComments(ctx, "o", "r", 0, opt)
	if err != nil {
		t.Errorf("PullRequests.ListComments returned error: %v", err)
	}

	want := []*PullRequestComment{{ID: Ptr(int64(1))}}
	if !cmp.Equal(pulls, want) {
		t.Errorf("PullRequests.ListComments returned %+v, want %+v", pulls, want)
	}

	const methodName = "ListComments"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.PullRequests.ListComments(ctx, "\n", "\n", -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.PullRequests.ListComments(ctx, "o", "r", 0, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestPullRequestsService_ListComments_specificPull(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	wantAcceptHeaders := []string{mediaTypeReactionsPreview, mediaTypeMultiLineCommentsPreview}
	mux.HandleFunc("/repos/o/r/pulls/1/comments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", strings.Join(wantAcceptHeaders, ", "))
		fmt.Fprint(w, `[{"id":1, "pull_request_review_id":42}]`)
	})

	ctx := t.Context()
	pulls, _, err := client.PullRequests.ListComments(ctx, "o", "r", 1, nil)
	if err != nil {
		t.Errorf("PullRequests.ListComments returned error: %v", err)
	}

	want := []*PullRequestComment{{ID: Ptr(int64(1)), PullRequestReviewID: Ptr(int64(42))}}
	if !cmp.Equal(pulls, want) {
		t.Errorf("PullRequests.ListComments returned %+v, want %+v", pulls, want)
	}
}

func TestPullRequestsService_ListComments_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.PullRequests.ListComments(ctx, "%", "r", 1, nil)
	testURLParseError(t, err)
}

func TestPullRequestsService_GetComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	wantAcceptHeaders := []string{mediaTypeReactionsPreview, mediaTypeMultiLineCommentsPreview}
	mux.HandleFunc("/repos/o/r/pulls/comments/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", strings.Join(wantAcceptHeaders, ", "))
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	comment, _, err := client.PullRequests.GetComment(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("PullRequests.GetComment returned error: %v", err)
	}

	want := &PullRequestComment{ID: Ptr(int64(1))}
	if !cmp.Equal(comment, want) {
		t.Errorf("PullRequests.GetComment returned %+v, want %+v", comment, want)
	}

	const methodName = "GetComment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.PullRequests.GetComment(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.PullRequests.GetComment(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestPullRequestsService_GetComment_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.PullRequests.GetComment(ctx, "%", "r", 1)
	testURLParseError(t, err)
}

func TestPullRequestsService_CreateComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &PullRequestComment{Body: Ptr("b")}

	wantAcceptHeaders := []string{mediaTypeReactionsPreview, mediaTypeMultiLineCommentsPreview}
	mux.HandleFunc("/repos/o/r/pulls/1/comments", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "Accept", strings.Join(wantAcceptHeaders, ", "))
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	comment, _, err := client.PullRequests.CreateComment(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("PullRequests.CreateComment returned error: %v", err)
	}

	want := &PullRequestComment{ID: Ptr(int64(1))}
	if !cmp.Equal(comment, want) {
		t.Errorf("PullRequests.CreateComment returned %+v, want %+v", comment, want)
	}

	const methodName = "CreateComment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.PullRequests.CreateComment(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.PullRequests.CreateComment(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestPullRequestsService_CreateComment_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.PullRequests.CreateComment(ctx, "%", "r", 1, nil)
	testURLParseError(t, err)
}

func TestPullRequestsService_CreateCommentInReplyTo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &PullRequestComment{Body: Ptr("b")}

	mux.HandleFunc("/repos/o/r/pulls/1/comments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	comment, _, err := client.PullRequests.CreateCommentInReplyTo(ctx, "o", "r", 1, "b", 2)
	if err != nil {
		t.Errorf("PullRequests.CreateCommentInReplyTo returned error: %v", err)
	}

	want := &PullRequestComment{ID: Ptr(int64(1))}
	if !cmp.Equal(comment, want) {
		t.Errorf("PullRequests.CreateCommentInReplyTo returned %+v, want %+v", comment, want)
	}

	const methodName = "CreateCommentInReplyTo"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.PullRequests.CreateCommentInReplyTo(ctx, "\n", "\n", -1, "\n", -2)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.PullRequests.CreateCommentInReplyTo(ctx, "o", "r", 1, "b", 2)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestPullRequestsService_EditComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &PullRequestComment{Body: Ptr("b")}

	mux.HandleFunc("/repos/o/r/pulls/comments/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	comment, _, err := client.PullRequests.EditComment(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("PullRequests.EditComment returned error: %v", err)
	}

	want := &PullRequestComment{ID: Ptr(int64(1))}
	if !cmp.Equal(comment, want) {
		t.Errorf("PullRequests.EditComment returned %+v, want %+v", comment, want)
	}

	const methodName = "EditComment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.PullRequests.EditComment(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.PullRequests.EditComment(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestPullRequestsService_EditComment_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.PullRequests.EditComment(ctx, "%", "r", 1, nil)
	testURLParseError(t, err)
}

func TestPullRequestsService_DeleteComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/pulls/comments/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.PullRequests.DeleteComment(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("PullRequests.DeleteComment returned error: %v", err)
	}

	const methodName = "DeleteComment"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.PullRequests.DeleteComment(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.PullRequests.DeleteComment(ctx, "o", "r", 1)
	})
}

func TestPullRequestsService_DeleteComment_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.PullRequests.DeleteComment(ctx, "%", "r", 1)
	testURLParseError(t, err)
}
