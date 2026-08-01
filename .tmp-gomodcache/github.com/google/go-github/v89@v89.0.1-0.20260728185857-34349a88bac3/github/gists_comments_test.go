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

func TestGistsService_ListComments(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1/comments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id": 1}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	comments, _, err := client.Gists.ListComments(ctx, "1", opt)
	if err != nil {
		t.Errorf("Gists.ListComments returned error: %v", err)
	}

	want := []*GistComment{{ID: Ptr(int64(1))}}
	if !cmp.Equal(comments, want) {
		t.Errorf("Gists.ListComments returned %+v, want %+v", comments, want)
	}

	const methodName = "ListComments"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.ListComments(ctx, "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.ListComments(ctx, "1", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_ListComments_invalidID(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Gists.ListComments(ctx, "%", nil)
	testURLParseError(t, err)
}

func TestGistsService_GetComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1/comments/2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id": 1}`)
	})

	ctx := t.Context()
	comment, _, err := client.Gists.GetComment(ctx, "1", 2)
	if err != nil {
		t.Errorf("Gists.GetComment returned error: %v", err)
	}

	want := &GistComment{ID: Ptr(int64(1))}
	if !cmp.Equal(comment, want) {
		t.Errorf("Gists.GetComment returned %+v, want %+v", comment, want)
	}

	const methodName = "GetComment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.GetComment(ctx, "\n", -2)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.GetComment(ctx, "1", 2)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_GetComment_invalidID(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Gists.GetComment(ctx, "%", 1)
	testURLParseError(t, err)
}

func TestGistsService_CreateComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateGistCommentRequest{Body: "b"}

	mux.HandleFunc("/gists/1/comments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	comment, _, err := client.Gists.CreateComment(ctx, "1", input)
	if err != nil {
		t.Errorf("Gists.CreateComment returned error: %v", err)
	}

	want := &GistComment{ID: Ptr(int64(1))}
	if !cmp.Equal(comment, want) {
		t.Errorf("Gists.CreateComment returned %+v, want %+v", comment, want)
	}

	const methodName = "CreateComment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.CreateComment(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.CreateComment(ctx, "1", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_CreateComment_invalidID(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Gists.CreateComment(ctx, "%", CreateGistCommentRequest{})
	testURLParseError(t, err)
}

func TestGistsService_UpdateComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := UpdateGistCommentRequest{Body: "b"}

	mux.HandleFunc("/gists/1/comments/2", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	comment, _, err := client.Gists.UpdateComment(ctx, "1", 2, input)
	if err != nil {
		t.Errorf("Gists.UpdateComment returned error: %v", err)
	}

	want := &GistComment{ID: Ptr(int64(1))}
	if !cmp.Equal(comment, want) {
		t.Errorf("Gists.UpdateComment returned %+v, want %+v", comment, want)
	}

	const methodName = "UpdateComment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Gists.UpdateComment(ctx, "\n", -2, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Gists.UpdateComment(ctx, "1", 2, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGistsService_UpdateComment_invalidID(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Gists.UpdateComment(ctx, "%", 1, UpdateGistCommentRequest{})
	testURLParseError(t, err)
}

func TestGistsService_DeleteComment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/gists/1/comments/2", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Gists.DeleteComment(ctx, "1", 2)
	if err != nil {
		t.Errorf("Gists.DeleteComment returned error: %v", err)
	}

	const methodName = "DeleteComment"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Gists.DeleteComment(ctx, "\n", -2)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Gists.DeleteComment(ctx, "1", 2)
	})
}

func TestGistsService_DeleteComment_invalidID(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Gists.DeleteComment(ctx, "%", 1)
	testURLParseError(t, err)
}
