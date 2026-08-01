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

func TestRepositoriesService_ListComments(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/comments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}, {"id":2}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	comments, _, err := client.Repositories.ListComments(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Repositories.ListComments returned error: %v", err)
	}

	want := []*RepositoryComment{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if !cmp.Equal(comments, want) {
		t.Errorf("Repositories.ListComments returned %+v, want %+v", comments, want)
	}

	const methodName = "ListComments"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListComments(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListComments(ctx, "o", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_ListComments_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.ListComments(ctx, "%", "%", nil)
	testURLParseError(t, err)
}

func TestRepositoriesService_ListCommitComments(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/commits/s/comments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}, {"id":2}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	comments, _, err := client.Repositories.ListCommitComments(ctx, "o", "r", "s", opt)
	if err != nil {
		t.Errorf("Repositories.ListCommitComments returned error: %v", err)
	}

	want := []*RepositoryComment{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if !cmp.Equal(comments, want) {
		t.Errorf("Repositories.ListCommitComments returned %+v, want %+v", comments, want)
	}

	const methodName = "ListCommitComments"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListCommitComments(ctx, "\n", "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListCommitComments(ctx, "o", "r", "s", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_ListCommitComments_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.ListCommitComments(ctx, "%", "%", "%", nil)
	testURLParseError(t, err)
}

func TestRepositoriesService_CreateComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &RepositoryComment{Body: Ptr("b")}

	mux.HandleFunc("/repos/o/r/commits/s/comments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	comment, _, err := client.Repositories.CreateComment(ctx, "o", "r", "s", input)
	if err != nil {
		t.Errorf("Repositories.CreateComment returned error: %v", err)
	}

	want := &RepositoryComment{ID: Ptr(int64(1))}
	if !cmp.Equal(comment, want) {
		t.Errorf("Repositories.CreateComment returned %+v, want %+v", comment, want)
	}

	const methodName = "CreateComment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.CreateComment(ctx, "\n", "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.CreateComment(ctx, "o", "r", "s", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_CreateComment_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.CreateComment(ctx, "%", "%", "%", nil)
	testURLParseError(t, err)
}

func TestRepositoriesService_GetComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/comments/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	comment, _, err := client.Repositories.GetComment(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Repositories.GetComment returned error: %v", err)
	}

	want := &RepositoryComment{ID: Ptr(int64(1))}
	if !cmp.Equal(comment, want) {
		t.Errorf("Repositories.GetComment returned %+v, want %+v", comment, want)
	}

	const methodName = "GetComment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetComment(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetComment(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetComment_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.GetComment(ctx, "%", "%", 1)
	testURLParseError(t, err)
}

func TestRepositoriesService_UpdateComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &RepositoryComment{Body: Ptr("b")}

	mux.HandleFunc("/repos/o/r/comments/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)

		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	comment, _, err := client.Repositories.UpdateComment(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("Repositories.UpdateComment returned error: %v", err)
	}

	want := &RepositoryComment{ID: Ptr(int64(1))}
	if !cmp.Equal(comment, want) {
		t.Errorf("Repositories.UpdateComment returned %+v, want %+v", comment, want)
	}

	const methodName = "UpdateComment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.UpdateComment(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.UpdateComment(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_UpdateComment_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.UpdateComment(ctx, "%", "%", 1, nil)
	testURLParseError(t, err)
}

func TestRepositoriesService_DeleteComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/comments/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Repositories.DeleteComment(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Repositories.DeleteComment returned error: %v", err)
	}

	const methodName = "DeleteComment"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.DeleteComment(ctx, "\n", "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.DeleteComment(ctx, "o", "r", 1)
	})
}

func TestRepositoriesService_DeleteComment_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Repositories.DeleteComment(ctx, "%", "%", 1)
	testURLParseError(t, err)
}
